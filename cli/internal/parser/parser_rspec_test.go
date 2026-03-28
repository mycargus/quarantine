package parser_test

import (
	"os"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/parser"
)

func TestParseSinglePassRSpec(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/rspec/single-pass.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid RSpec single-pass JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "an RSpec XML with 3 passing tests",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first RSpec test case",
		Should:   "extract file_path from the file attribute",
		Actual:   results[0].FilePath,
		Expected: "./spec/models/user_spec.rb",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first RSpec test case",
		Should:   "construct the correct test_id",
		Actual:   results[0].TestID,
		Expected: "./spec/models/user_spec.rb::spec.models.user_spec::User#valid? returns true for valid attributes",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing RSpec test",
		Should:   "have status passed",
		Actual:   results[0].Status,
		Expected: "passed",
	})
}

func TestParseSingleFailureRSpec(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/rspec/single-failure.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid RSpec single-failure JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "an RSpec XML with 3 tests (1 failure)",
		Should:   "return 3 test results",
		Actual:   len(results),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second RSpec test case with a <failure> element",
		Should:   "have status failed",
		Actual:   results[1].Status,
		Expected: "failed",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the second RSpec test case with a <failure> element",
		Should:   "have a non-nil FailureMessage",
		Actual:   results[1].FailureMessage != nil,
		Expected: true,
	})
}
