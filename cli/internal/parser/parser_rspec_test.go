package parser_test

import (
	"os"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/parser"
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

func TestParseRSpecSharedExamples(t *testing.T) {
	f, err := os.Open("../../../testdata/junit-xml/rspec/shared-examples.xml")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	results, err := parser.Parse(f)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid RSpec shared-examples JUnit XML fixture",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "an RSpec XML with 2 shared-example test cases",
		Should:   "return 2 test results",
		Actual:   len(results),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the first shared-example test case with classname 'User when admin'",
		Should:   "construct a test_id incorporating the classname",
		Actual:   results[0].TestID,
		Expected: "./spec/models/user_spec.rb::User when admin::has admin privileges",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second shared-example test case with classname 'ServiceAccount when admin'",
		Should:   "construct a test_id incorporating the classname",
		Actual:   results[1].TestID,
		Expected: "./spec/models/user_spec.rb::ServiceAccount when admin::has admin privileges",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a shared-example test case",
		Should:   "extract the file path from the file attribute",
		Actual:   results[0].FilePath,
		Expected: "./spec/models/user_spec.rb",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a shared-example test case with no failure element",
		Should:   "have status 'passed'",
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

	if results[1].FailureMessage == nil {
		t.Fatal("FailureMessage is nil for the failing test case")
	}

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the second RSpec test case with a <failure> element",
		Should:   "populate FailureMessage from the failure message attribute",
		Actual:   *results[1].FailureMessage,
		Expected: "expected: 503\n     got: 500\n\n(compared using ==)",
	})
}
