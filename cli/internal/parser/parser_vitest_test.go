package parser_test

import (
	"os"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/parser"
)

func TestParseSinglePassVitest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/vitest/single-pass.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid Vitest single-pass JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Vitest XML with 3 passing tests",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a Vitest test case (no file attribute)",
		Should:   "extract file_path from the suite name attribute",
		Actual:   results[0].FilePath,
		Expected: "src/utils/__tests__/math.test.ts",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first Vitest test case",
		Should:   "construct the correct test_id",
		Actual:   results[0].TestID,
		Expected: "src/utils/__tests__/math.test.ts::src/utils/__tests__/math.test.ts::math > add > should add positive numbers",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing Vitest test",
		Should:   "have status passed",
		Actual:   results[0].Status,
		Expected: "passed",
	})
}

func TestParseVitestParameterized(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/vitest/parameterized.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Vitest JUnit XML fixture with two test.each variants",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Vitest XML with 2 parameterized test variants",
		Should:   "return 2 test results",
		Actual:   len(results),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first Vitest test.each variant (processes foo with count 1)",
		Should:   "construct test_id from suite name, classname, and variant name",
		Actual:   results[0].TestID,
		Expected: "src/processor.test.ts::src/processor.test.ts::processes foo with count 1",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second Vitest test.each variant (processes bar with count 2)",
		Should:   "construct test_id from suite name, classname, and variant name",
		Actual:   results[1].TestID,
		Expected: "src/processor.test.ts::src/processor.test.ts::processes bar with count 2",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a Vitest test case with no file attribute",
		Should:   "extract file_path from the suite name attribute",
		Actual:   results[0].FilePath,
		Expected: "src/processor.test.ts",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing Vitest parameterized test",
		Should:   "have status passed",
		Actual:   results[0].Status,
		Expected: "passed",
	})
}

func TestParseSingleFailureVitest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/vitest/single-failure.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid Vitest single-failure JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a Vitest XML with 3 tests (1 failure)",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second Vitest test case with a <failure> element",
		Should:   "have status failed",
		Actual:   results[1].Status,
		Expected: "failed",
	})

	if results[1].FailureMessage == nil {
		t.Fatal("FailureMessage is nil for the failing Vitest test case")
	}

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second Vitest test case with a <failure> element",
		Should:   "populate FailureMessage from the failure message attribute",
		Actual:   *results[1].FailureMessage,
		Expected: "AssertionError: expected 'NetworkError' to be 'NotFoundError'",
	})
}

func TestParseMalformedXMLVitest(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/vitest/malformed.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	_, err = parser.Parse(f)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a malformed (truncated) Vitest JUnit XML fixture",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}
