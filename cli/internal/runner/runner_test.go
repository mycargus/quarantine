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
		Should:   "substitute {name} with the test name",
		Actual:   cmd,
		Expected: "custom-runner --test my test",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty customTemplate",
		Should:   "return nil args",
		Actual:   args == nil,
		Expected: true,
	})
}

func TestRerunCommandPlaceholderSubstitution(t *testing.T) {
	cmd, _ := runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "run {name}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template with {name} placeholder",
		Should:   "replace {name} with the test name",
		Actual:   cmd,
		Expected: "run my test",
	})

	cmd, _ = runner.RerunCommand(runner.RSpec, "my test", "MyClass", "spec/foo_spec.rb", "run {classname}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template with {classname} placeholder",
		Should:   "replace {classname} with the classname",
		Actual:   cmd,
		Expected: "run MyClass",
	})

	cmd, _ = runner.RerunCommand(runner.Vitest, "my test", "MyClass", "src/foo.test.ts", "run {file}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template with {file} placeholder",
		Should:   "replace {file} with the file path",
		Actual:   cmd,
		Expected: "run src/foo.test.ts",
	})

	cmd, _ = runner.RerunCommand(runner.Jest, "my test", "MyClass", "src/foo.test.js", "run {name} in {classname} at {file}")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "custom template with all three placeholders",
		Should:   "replace all placeholders",
		Actual:   cmd,
		Expected: "run my test in MyClass at src/foo.test.js",
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
