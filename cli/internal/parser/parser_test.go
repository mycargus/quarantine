package parser_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/parser"
)

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

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a skipped test case with time=0.000",
		Should:   "produce DurationMs of 0",
		Actual:   results[3].DurationMs,
		Expected: 0,
	})
}

func TestParseEmptyFileAndSuiteName(t *testing.T) {
	const raw = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="">
    <testcase classname="SomeClass" name="test name" time="0.010">
    </testcase>
  </testsuite>
</testsuites>`

	results, err := parser.Parse(strings.NewReader(raw))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a JUnit XML with empty suite name and no file attribute on testcase",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a testcase with no file attribute inside a testsuite with empty name",
		Should:   "have an empty FilePath",
		Actual:   results[0].FilePath,
		Expected: "",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a testcase with no file attribute inside a testsuite with empty name",
		Should:   "construct TestID as '::SomeClass::test name'",
		Actual:   results[0].TestID,
		Expected: "::SomeClass::test name",
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

// TestParseDurationMilliseconds verifies that tc.Time is converted to milliseconds
// by multiplying by 1000 (not 999 or another value).
// Kills mutation on line 128: `tc.Time * 1000` → `tc.Time * 999`.
func TestParseDurationMilliseconds(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0" errors="0" time="1.0">
  <testsuite name="suite" tests="1" failures="0" time="1.0">
    <testcase classname="Foo" name="bar" file="foo.test.js" time="1.000"/>
  </testsuite>
</testsuites>`

	results, err := parser.Parse(strings.NewReader(xml))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a test case with time='1.000' (1 second)",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a test case with time='1.000' (1 second = 1000 milliseconds)",
		Should:   "return DurationMs=1000 (not 999)",
		Actual:   results[0].DurationMs,
		Expected: 1000,
	})
}
