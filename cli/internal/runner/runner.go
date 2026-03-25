// Package runner handles test command execution and framework-specific
// rerun command construction.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// Framework identifies a test framework supported by the CLI.
type Framework string

const (
	Jest   Framework = "jest"
	RSpec  Framework = "rspec"
	Vitest Framework = "vitest"
)

// RunResult holds the outcome of executing a test command.
type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Run executes the given test command, piping stdout and stderr to the
// provided writers. Forwards SIGINT and SIGTERM to the child process.
// Returns the exit code.
func Run(ctx context.Context, command string, args []string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("failed to execute test command: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	err := cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("failed to execute test command: %w", err)
	}

	return 0, nil
}

// jestRegexSpecialChars lists the regex special characters that must be
// escaped in Jest --testNamePattern values. Backslash is listed first so
// that added escape characters are not re-escaped.
var jestRegexSpecialChars = []string{"\\", ".", "(", ")", "[", "]"}

// EscapeJestPattern escapes regex special characters in a Jest test name so it
// can be passed safely to --testNamePattern.
// This is a pure function — no I/O.
func EscapeJestPattern(name string) string {
	for _, ch := range jestRegexSpecialChars {
		name = strings.ReplaceAll(name, ch, `\`+ch)
	}
	return name
}

// RerunCommand returns the framework-specific command and arguments for
// rerunning a single failed test.
func RerunCommand(fw Framework, name, classname, file, customTemplate string) (string, []string) {
	if customTemplate != "" {
		// Split the template into tokens BEFORE substituting placeholders.
		// This ensures that placeholder values containing spaces (e.g. a test
		// name like "should handle timeout") stay as a single argument.
		parts := SplitShellArgs(customTemplate)
		for i, p := range parts {
			p = strings.ReplaceAll(p, "{name}", name)
			p = strings.ReplaceAll(p, "{classname}", classname)
			p = strings.ReplaceAll(p, "{file}", file)
			parts[i] = p
		}
		if len(parts) == 0 {
			return "", nil
		}
		return parts[0], parts[1:]
	}

	switch fw {
	case Jest:
		return "jest", []string{"--testNamePattern", EscapeJestPattern(name)}
	case RSpec:
		return "rspec", []string{"-e", name}
	case Vitest:
		return "vitest", []string{"run", "--reporter=junit", file, "-t", name}
	default:
		return "", nil
	}
}

// SplitShellArgs splits a command string into tokens, respecting single and
// double quotes. Quotes are stripped from the output. This is a pure function.
func SplitShellArgs(s string) []string {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for _, r := range s {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
