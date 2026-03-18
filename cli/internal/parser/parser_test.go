package parser_test

import (
	"os"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/parser"
)

func TestParseSinglePassJest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/jest/single-pass.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer f.Close()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid Jest single-pass JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Jest XML with 3 passing tests",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first test case",
		Should:   "construct the correct test_id",
		Actual:   results[0].TestID,
		Expected: "__tests__/auth/login.test.js::LoginForm validates input::should reject empty email",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first test case",
		Should:   "extract the file path from the file attribute",
		Actual:   results[0].FilePath,
		Expected: "__tests__/auth/login.test.js",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first test case",
		Should:   "extract the classname",
		Actual:   results[0].Classname,
		Expected: "LoginForm validates input",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first test case",
		Should:   "extract the name",
		Actual:   results[0].Name,
		Expected: "should reject empty email",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing test",
		Should:   "have status passed",
		Actual:   results[0].Status,
		Expected: "passed",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "the first test case with time=0.045",
		Should:   "convert duration to 45 milliseconds",
		Actual:   results[0].DurationMs,
		Expected: 45,
	})

	// Verify all tests are passing.
	for _, r := range results {
		riteway.Assert(t, riteway.Case[string]{
			Given:    "a single-pass fixture",
			Should:   "report all tests as passed",
			Actual:   r.Status,
			Expected: "passed",
		})
	}
}
