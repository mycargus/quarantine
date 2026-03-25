package runner_test

import (
	"context"
	"io"
	"testing"

	"github.com/mycargus/quarantine/internal/runner"
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
	cmd, args := runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "Jest framework",
		Should:   "use jest as command",
		Actual:   cmd,
		Expected: "jest",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "Jest framework",
		Should:   "pass --testNamePattern with test name",
		Actual:   args,
		Expected: []string{"--testNamePattern", "my test"},
	})

	cmd, args = runner.RerunCommand(runner.RSpec, "my test", "MyClass", "spec/foo_spec.rb", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "RSpec framework",
		Should:   "use rspec as command",
		Actual:   cmd,
		Expected: "rspec",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "RSpec framework",
		Should:   "pass -e with test name",
		Actual:   args,
		Expected: []string{"-e", "my test"},
	})

	cmd, args = runner.RerunCommand(runner.Vitest, "my test", "MyClass", "src/foo.test.ts", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "Vitest framework",
		Should:   "use vitest as command",
		Actual:   cmd,
		Expected: "vitest",
	})
	riteway.Assert(t, riteway.Case[[]string]{
		Given:    "Vitest framework",
		Should:   "pass run --reporter=junit <file> -t <name>",
		Actual:   args,
		Expected: []string{"run", "--reporter=junit", "src/foo.test.ts", "-t", "my test"},
	})

	cmd, args = runner.RerunCommand("unknown", "my test", "MyClass", "src/foo.test.js", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "unknown framework",
		Should:   "return empty command",
		Actual:   cmd,
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "unknown framework",
		Should:   "return nil args",
		Actual:   args == nil,
		Expected: true,
	})

	cmd, args = runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "custom-runner --test {name}")
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
	cmd, args := runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "run {name}")
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

	cmd, args = runner.RerunCommand(runner.RSpec, "my test", "MyClass", "spec/foo_spec.rb", "run {classname}")
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

	cmd, args = runner.RerunCommand(runner.Vitest, "my test", "MyClass", "src/foo.test.ts", "run {file}")
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

	cmd, args = runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "npx jest --testNamePattern '{name}' --config jest.ci.config.js")
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

func TestEscapeJestPattern(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "test name with a dot",
		Should:   "escape dot to \\.",
		Actual:   runner.EscapeJestPattern("foo.bar"),
		Expected: `foo\.bar`,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test name with parentheses",
		Should:   "escape ( and ) to \\( and \\)",
		Actual:   runner.EscapeJestPattern("test (case)"),
		Expected: `test \(case\)`,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test name with square brackets",
		Should:   "escape [ and ] to \\[ and \\]",
		Actual:   runner.EscapeJestPattern("arr[0]"),
		Expected: `arr\[0\]`,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "plain test name with no special chars",
		Should:   "return name unchanged",
		Actual:   runner.EscapeJestPattern("should do something"),
		Expected: "should do something",
	})
}
