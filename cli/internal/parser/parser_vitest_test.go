package parser_test

import (
	"os"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/parser"
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

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the second Vitest test case with a <failure> element",
		Should:   "have a non-nil FailureMessage",
		Actual:   results[1].FailureMessage != nil,
		Expected: true,
	})
}
