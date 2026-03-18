// Package runner handles test command execution and framework-specific
// rerun command construction.
package runner

import (
	"context"
	"fmt"
	"os/exec"
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

// Run executes the given test command and returns its result.
func Run(ctx context.Context, command string, args []string) (*RunResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute test command: %w", err)
		}
	}

	return &RunResult{
		ExitCode: exitCode,
		Stdout:   output,
	}, nil
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
