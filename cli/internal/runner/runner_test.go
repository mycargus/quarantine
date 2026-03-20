package runner_test

import (
	"testing"

	"github.com/mycargus/quarantine/internal/runner"
	riteway "github.com/mycargus/riteway-golang"
)

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
}
