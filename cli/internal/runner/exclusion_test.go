package runner_test

import (
	"strings"
	"testing"

	"github.com/mycargus/quarantine/internal/quarantine"
	"github.com/mycargus/quarantine/internal/runner"
	riteway "github.com/mycargus/riteway-golang"
)

// --- BuildExclusionArgs ---

func TestBuildExclusionArgsRSpecReturnsNil(t *testing.T) {
	entries := []quarantine.Entry{
		{TestID: "spec/models/user_spec.rb::User::is valid", Name: "is valid", FilePath: "spec/models/user_spec.rb"},
	}

	args := runner.BuildExclusionArgs(runner.RSpec, entries)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec framework with quarantined entries",
		Should:   "return nil (no pre-execution exclusion for RSpec in v1)",
		Actual:   args == nil,
		Expected: true,
	})
}

func TestBuildExclusionArgsEmptyEntriesReturnsNil(t *testing.T) {
	args := runner.BuildExclusionArgs(runner.Jest, []quarantine.Entry{})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest framework with no quarantined entries",
		Should:   "return nil",
		Actual:   args == nil,
		Expected: true,
	})
}

func TestBuildExclusionArgsNilEntriesReturnsNil(t *testing.T) {
	args := runner.BuildExclusionArgs(runner.Jest, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest framework with nil entries",
		Should:   "return nil",
		Actual:   args == nil,
		Expected: true,
	})
}

func TestBuildExclusionArgsJestSingleEntryUsesTestNamePattern(t *testing.T) {
	entries := []quarantine.Entry{
		{
			TestID:   "src/payment.test.js::PaymentService::should handle charge timeout",
			Name:     "should handle charge timeout",
			FilePath: "src/payment.test.js",
		},
	}

	args := runner.BuildExclusionArgs(runner.Jest, entries)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with one quarantined test",
		Should:   "return non-nil args",
		Actual:   args != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with one quarantined test",
		Should:   "include --testNamePattern flag",
		Actual:   containsArg(args, "--testNamePattern"),
		Expected: true,
	})

	// The pattern must be a negative lookahead excluding the escaped test name.
	pattern := argAfter(args, "--testNamePattern")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with test name 'should handle charge timeout'",
		Should:   "include the test name in the negative lookahead pattern",
		Actual:   strings.Contains(pattern, "should handle charge timeout"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest exclusion pattern",
		Should:   "use negative lookahead format ^(?!.*(...)).*$",
		Actual:   strings.HasPrefix(pattern, "^(?!.*(") && strings.HasSuffix(pattern, ")).*$"),
		Expected: true,
	})
}

func TestBuildExclusionArgsJestMultipleEntriesUsesNegativeLookahead(t *testing.T) {
	entries := []quarantine.Entry{
		{Name: "should handle timeout", FilePath: "src/payment.test.js"},
		{Name: "should retry on error", FilePath: "src/payment.test.js"},
	}

	args := runner.BuildExclusionArgs(runner.Jest, entries)
	pattern := argAfter(args, "--testNamePattern")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with two quarantined tests in same file",
		Should:   "include both test names in the pattern joined by |",
		Actual:   strings.Contains(pattern, "should handle timeout") && strings.Contains(pattern, "should retry on error") && strings.Contains(pattern, "|"),
		Expected: true,
	})
}

func TestBuildExclusionArgsJestEscapesSpecialCharsInTestName(t *testing.T) {
	entries := []quarantine.Entry{
		{Name: "should return foo.bar (test case)", FilePath: "src/foo.test.js"},
	}

	args := runner.BuildExclusionArgs(runner.Jest, entries)
	pattern := argAfter(args, "--testNamePattern")

	// Dots and parens must be escaped in the pattern.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with test name containing regex special chars '.' and '()'",
		Should:   "escape the dot to \\. in the pattern",
		Actual:   strings.Contains(pattern, `\.`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with test name containing regex special chars '.' and '()'",
		Should:   "escape parentheses to \\( and \\) in the pattern",
		Actual:   strings.Contains(pattern, `\(`) && strings.Contains(pattern, `\)`),
		Expected: true,
	})
}

func TestBuildExclusionArgsVitestUsesExcludeFlag(t *testing.T) {
	entries := []quarantine.Entry{
		{
			TestID:   "src/payment.test.ts::PaymentService::should handle charge timeout",
			Name:     "should handle charge timeout",
			FilePath: "src/payment.test.ts",
		},
	}

	args := runner.BuildExclusionArgs(runner.Vitest, entries)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Vitest with one quarantined test",
		Should:   "return non-nil args",
		Actual:   args != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Vitest with one quarantined test",
		Should:   "include -t flag for name-based exclusion",
		Actual:   containsArg(args, "-t"),
		Expected: true,
	})

	pattern := argAfter(args, "-t")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Vitest exclusion pattern",
		Should:   "use negative lookahead format",
		Actual:   strings.HasPrefix(pattern, "^(?!.*(") && strings.HasSuffix(pattern, ")).*$"),
		Expected: true,
	})
}

// containsArg checks if the slice contains the given string.
func containsArg(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

// argAfter returns the element immediately following the given flag in args.
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
