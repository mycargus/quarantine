package runner_test

import (
	"context"
	"io"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/runner"
	riteway "github.com/mycargus/riteway-golang"
)

func TestRunStartFails(t *testing.T) {
	exitCode, err := runner.Run(context.Background(), "/nonexistent/path", nil, io.Discard, io.Discard)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a non-existent command path",
		Should:   "return exit code -1",
		Actual:   exitCode,
		Expected: -1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-existent command path",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestRerunCommand(t *testing.T) {
	cmd, args := runner.RerunCommand("my test", "MyClass", "src/foo.test.js", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "no custom rerun_command",
		Should:   "return empty command",
		Actual:   cmd,
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no custom rerun_command",
		Should:   "return nil args",
		Actual:   args == nil,
		Expected: true,
	})

	cmd, args = runner.RerunCommand("my test", "MyClass", "src/foo.test.js", "custom-runner --test {name}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "a custom template with {name} placeholder",
		Should:   "use the first token as the command",
		Actual:   cmd,
		Expected: "custom-runner",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "a custom template 'custom-runner --test {name}' with name 'my test'",
		Should:   "split into args with the substituted name as one token",
		Actual:   args,
		Expected: []string{"--test", "my test"},
	})
}

func TestRerunCommandPlaceholderSubstitution(t *testing.T) {
	cmd, args := runner.RerunCommand("my test", "MyClass", "src/foo.test.js", "run {name}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template 'run {name}'",
		Should:   "use 'run' as the command",
		Actual:   cmd,
		Expected: "run",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "custom template 'run {name}' with name 'my test'",
		Should:   "keep the substituted name as a single arg",
		Actual:   args,
		Expected: []string{"my test"},
	})

	cmd, args = runner.RerunCommand("my test", "MyClass", "spec/foo_spec.rb", "run {classname}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template 'run {classname}'",
		Should:   "use 'run' as the command",
		Actual:   cmd,
		Expected: "run",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "custom template 'run {classname}' with classname 'MyClass'",
		Should:   "substitute {classname} in args",
		Actual:   args,
		Expected: []string{"MyClass"},
	})

	cmd, args = runner.RerunCommand("my test", "MyClass", "src/foo.test.ts", "run {file}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template 'run {file}'",
		Should:   "use 'run' as the command",
		Actual:   cmd,
		Expected: "run",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "custom template 'run {file}'",
		Should:   "substitute {file} in args",
		Actual:   args,
		Expected: []string{"src/foo.test.ts"},
	})

	cmd, args = runner.RerunCommand("my test", "MyClass", "src/foo.test.js", "npx jest --testNamePattern '{name}' --config jest.ci.config.js")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "realistic custom template with quoted {name}",
		Should:   "use 'npx' as the command",
		Actual:   cmd,
		Expected: "npx",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "realistic custom template with quoted {name}",
		Should:   "split correctly with name as one token (quotes stripped)",
		Actual:   args,
		Expected: []string{"jest", "--testNamePattern", "my test", "--config", "jest.ci.config.js"},
	})
}

func TestSplitShellArgs(t *testing.T) {
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "simple command with args",
		Should:   "split on spaces",
		Actual:   runner.SplitShellArgs("cmd --flag value"),
		Expected: []string{"cmd", "--flag", "value"},
	})

	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "single-quoted argument",
		Should:   "keep quoted content as one token with quotes stripped",
		Actual:   runner.SplitShellArgs("cmd 'hello world'"),
		Expected: []string{"cmd", "hello world"},
	})

	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "double-quoted argument",
		Should:   "keep quoted content as one token with quotes stripped",
		Actual:   runner.SplitShellArgs(`cmd "hello world"`),
		Expected: []string{"cmd", "hello world"},
	})

	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "multiple spaces between tokens",
		Should:   "collapse whitespace",
		Actual:   runner.SplitShellArgs("cmd   --flag   value"),
		Expected: []string{"cmd", "--flag", "value"},
	})

	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "empty string",
		Should:   "return nil",
		Actual:   runner.SplitShellArgs(""),
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "single command with no args",
		Should:   "return one-element slice",
		Actual:   runner.SplitShellArgs("cmd"),
		Expected: []string{"cmd"},
	})
}

// TestRunSuccessReturnsZeroExitCode verifies that a command exiting 0 returns
// exit code 0 and no error.
// Kills mutations:
//   - Line 57: `err != nil` → `err == nil` (enters error path on success)
//   - Line 64: `return 0` → `return 1` (returns wrong exit code on success)
func TestRunSuccessReturnsZeroExitCode(t *testing.T) {
	exitCode, err := runner.Run(context.Background(), "true", nil, io.Discard, io.Discard)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "command 'true' exits with code 0",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "command 'true' exits with code 0",
		Should:   "return exit code 0",
		Actual:   exitCode,
		Expected: 0,
	})
}

// TestRunNonZeroExitCodeIsPreserved verifies that a command exiting with a
// specific non-zero code returns that exact code.
// Kills mutation on line 58: `ok` → `!ok` (skips ExitError extraction).
func TestRunNonZeroExitCodeIsPreserved(t *testing.T) {
	// 'sh -c "exit 42"' exits with code 42.
	exitCode, err := runner.Run(context.Background(), "sh", []string{"-c", "exit 42"}, io.Discard, io.Discard)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "command exits with code 42",
		Should:   "return no error (non-zero exit is not a runner error)",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "command exits with code 42",
		Should:   "return exit code 42",
		Actual:   exitCode,
		Expected: 42,
	})
}
