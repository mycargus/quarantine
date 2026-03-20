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

// RerunCommand returns the framework-specific command and arguments for
// rerunning a single failed test.
func RerunCommand(fw Framework, name, classname, file, customTemplate string) (string, []string) {
	if customTemplate != "" {
		// TODO: Implement placeholder substitution for custom rerun_command.
		return customTemplate, nil
	}

	switch fw {
	case Jest:
		return "jest", []string{"--testNamePattern", name}
	case RSpec:
		return "rspec", []string{"-e", name}
	case Vitest:
		return "vitest", []string{"run", "--reporter=junit", file, "-t", name}
	default:
		return "", nil
	}
}
