package main

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/result"
)

func TestBackfillIssueNumbers(t *testing.T) {
	tests := []result.TestEntry{
		{TestID: "suite::flaky_test", Name: "flaky_test", Status: "flaky", IssueNumber: nil},
		{TestID: "suite::passing_test", Name: "passing_test", Status: "passed", IssueNumber: nil},
	}
	refs := map[string]issueRef{
		"suite::flaky_test": {Number: 42, URL: "https://github.com/o/r/issues/42"},
	}

	backfillIssueNumbers(tests, refs)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test with a matching issueRef",
		Should:   "set IssueNumber to the ref's Number",
		Actual:   tests[0].IssueNumber != nil && *tests[0].IssueNumber == 42,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a passing test with no matching issueRef",
		Should:   "leave IssueNumber as nil",
		Actual:   tests[1].IssueNumber == nil,
		Expected: true,
	})
}

func TestBackfillIssueNumbersEmptyRefs(t *testing.T) {
	tests := []result.TestEntry{
		{TestID: "suite::flaky_test", Name: "flaky_test", Status: "flaky", IssueNumber: nil},
	}

	backfillIssueNumbers(tests, map[string]issueRef{})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty issueRefs map",
		Should:   "leave all IssueNumber fields as nil",
		Actual:   tests[0].IssueNumber == nil,
		Expected: true,
	})
}

func TestBackfillIssueNumbersMultipleMatches(t *testing.T) {
	tests := []result.TestEntry{
		{TestID: "suite::test_a", Name: "test_a", Status: "flaky", IssueNumber: nil},
		{TestID: "suite::test_b", Name: "test_b", Status: "flaky", IssueNumber: nil},
		{TestID: "suite::test_c", Name: "test_c", Status: "passed", IssueNumber: nil},
	}
	refs := map[string]issueRef{
		"suite::test_a": {Number: 10, URL: "https://github.com/o/r/issues/10"},
		"suite::test_b": {Number: 20, URL: "https://github.com/o/r/issues/20"},
	}

	backfillIssueNumbers(tests, refs)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two flaky tests with matching issueRefs",
		Should:   "set IssueNumber on the first flaky test",
		Actual:   tests[0].IssueNumber != nil && *tests[0].IssueNumber == 10,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two flaky tests with matching issueRefs",
		Should:   "set IssueNumber on the second flaky test",
		Actual:   tests[1].IssueNumber != nil && *tests[1].IssueNumber == 20,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a passing test with no matching issueRef",
		Should:   "leave IssueNumber as nil",
		Actual:   tests[2].IssueNumber == nil,
		Expected: true,
	})
}
