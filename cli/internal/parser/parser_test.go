package parser_test

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/parser"
)

func TestParseSinglePassJest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/jest/single-pass.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

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

func TestParseSingleFailureJest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/jest/single-failure.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Jest JUnit XML fixture with one failure",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Jest XML with 3 tests (1 failure, 2 passing)",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second test case with a <failure> element",
		Should:   "have status 'failed'",
		Actual:   results[1].Status,
		Expected: "failed",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the second test case with a <failure> element",
		Should:   "have a non-nil FailureMessage",
		Actual:   results[1].FailureMessage != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the failing test case",
		Should:   "populate FailureMessage with the failure message attribute",
		Actual:   *results[1].FailureMessage,
		Expected: "expected 'declined' but received 'error'",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing test case in the same suite",
		Should:   "have status 'passed'",
		Actual:   results[0].Status,
		Expected: "passed",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a passing test case",
		Should:   "have a nil FailureMessage",
		Actual:   results[0].FailureMessage == nil,
		Expected: true,
	})
}

func TestParseMalformedXML(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/jest/malformed.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	_, err = parser.Parse(f)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a malformed (truncated) JUnit XML fixture",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseMultipleSuitesJest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/jest/multiple-suites.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Jest JUnit XML fixture with 3 suites and 6 tests",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Jest XML with 6 tests across 3 suites",
		Should:   "return 6 test results",
		Actual:   len(results),
		Expected: 6,
	})

	// results[3] is the 4th test: Modal accessibility "should trap focus when open" with <skipped/>.
	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test case with a <skipped/> element",
		Should:   "have status 'skipped'",
		Actual:   results[3].Status,
		Expected: "skipped",
	})
}

func TestParseAllStatusTypes(t *testing.T) {
	const raw = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="suite.test.js">
    <testcase classname="Suite" name="passes" file="suite.test.js" time="0.010">
    </testcase>
    <testcase classname="Suite" name="fails" file="suite.test.js" time="0.020">
      <failure message="expected true but got false" type="AssertionError">stacktrace</failure>
    </testcase>
    <testcase classname="Suite" name="errors" file="suite.test.js" time="0.005">
      <error message="runtime panic" type="Error">panic stacktrace</error>
    </testcase>
    <testcase classname="Suite" name="is skipped" file="suite.test.js" time="0.000">
      <skipped/>
    </testcase>
  </testsuite>
</testsuites>`

	results, err := parser.Parse(strings.NewReader(raw))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a JUnit XML with all four status types",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a JUnit XML with 4 test cases",
		Should:   "return 4 results",
		Actual:   len(results),
		Expected: 4,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test case with no child elements",
		Should:   "have status 'passed'",
		Actual:   results[0].Status,
		Expected: "passed",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test case with a <failure> element",
		Should:   "have status 'failed'",
		Actual:   results[1].Status,
		Expected: "failed",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test case with an <error> element",
		Should:   "have status 'error'",
		Actual:   results[2].Status,
		Expected: "error",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test case with a <skipped> element",
		Should:   "have status 'skipped'",
		Actual:   results[3].Status,
		Expected: "skipped",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a passing test case",
		Should:   "have a nil FailureMessage",
		Actual:   results[0].FailureMessage == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a failing test case",
		Should:   "have a non-nil FailureMessage",
		Actual:   results[1].FailureMessage != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a failing test case with a failure message attribute",
		Should:   "populate FailureMessage with the attribute value",
		Actual:   *results[1].FailureMessage,
		Expected: "expected true but got false",
	})
}

func TestParseEmptyTestSuite(t *testing.T) {
	const raw = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="empty.test.js" tests="0">
  </testsuite>
</testsuites>`

	results, err := parser.Parse(strings.NewReader(raw))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a JUnit XML with a test suite containing zero test cases",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a test suite with zero test cases",
		Should:   "return zero results",
		Actual:   len(results),
		Expected: 0,
	})
}
