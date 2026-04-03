package main

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"
	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
)

// --- Unit tests for buildFlakyEntries (mutation targets) ---

// TestBuildFlakyEntriesExcludesNonFlakyStatus kills the mutation on line 389:
// `t.Status != "flaky"` → `== "flaky"`. Only tests with Status=="flaky" should
// appear in the output. A test with Status=="failed" must be excluded.
func TestBuildFlakyEntriesExcludesNonFlakyStatus(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::failed-test", Name: "failed-test", Status: "failed"},
			{TestID: "suite::passed-test", Name: "passed-test", Status: "passed"},
		},
	}

	withIssue, newToPR := buildFlakyEntries(res, nil, nil)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with only failed and passed tests (no flaky)",
		Should:   "return empty withIssue slice",
		Actual:   len(withIssue),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with only failed and passed tests (no flaky)",
		Should:   "return empty newToPR slice",
		Actual:   len(newToPR),
		Expected: 0,
	})
}

// TestBuildFlakyEntriesIncludesFlakyStatus provides the counterpart: a flaky
// test is included in the output.
func TestBuildFlakyEntriesIncludesFlakyStatus(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-test", Name: "flaky-test", Status: "flaky"},
			{TestID: "suite::failed-test", Name: "failed-test", Status: "failed"},
		},
	}

	withIssue, _ := buildFlakyEntries(res, map[string]issueRef{
		"suite::flaky-test": {Number: 7, URL: "https://github.com/o/r/issues/7"},
	}, nil)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with one flaky and one failed test",
		Should:   "return exactly one entry in withIssue (the flaky test)",
		Actual:   len(withIssue),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "flaky test in result",
		Should:   "have the flaky test name in the entry",
		Actual:   withIssue[0].Name,
		Expected: "flaky-test",
	})
}

// TestBuildFlakyEntriesSkipReasonRoutesToNewToPR kills the mutation on line 392:
// `skipReasons[t.TestID] != ""` → `== ""`. Tests WITH a skip reason should be
// placed in newToPR, not in withIssue. The mutation would include skip-reason
// tests in withIssue and exclude them from newToPR.
func TestBuildFlakyEntriesSkipReasonRoutesToNewToPR(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-skipped", Name: "flaky-skipped", Status: "flaky"},
			{TestID: "suite::flaky-normal", Name: "flaky-normal", Status: "flaky"},
		},
	}
	skipReasons := map[string]string{
		"suite::flaky-skipped": "new-to-pr",
	}

	withIssue, newToPR := buildFlakyEntries(res, map[string]issueRef{
		"suite::flaky-normal": {Number: 3, URL: "https://github.com/o/r/issues/3"},
	}, skipReasons)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one flaky test with skip reason and one without",
		Should:   "route the skip-reason test to newToPR (not withIssue)",
		Actual:   len(newToPR),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "flaky test with skip reason",
		Should:   "have the test name in newToPR",
		Actual:   newToPR[0].Name,
		Expected: "flaky-skipped",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one flaky test with skip reason and one without",
		Should:   "put only the non-skip-reason test in withIssue",
		Actual:   len(withIssue),
		Expected: 1,
	})
}

// --- Unit tests for buildQuarantinedEntries (mutation targets) ---

// TestBuildQuarantinedEntriesIssueNumberNilGivesZero kills the mutation on
// line 415: `e.IssueNumber != nil` → `== nil`. When an entry HAS an
// IssueNumber, its value must appear in the output. The mutation would use 0
// for entries with an issue number and dereference nil for entries without one.
func TestBuildQuarantinedEntriesIssueNumberNilGivesZero(t *testing.T) {
	n := 42
	state := &qstate.State{
		Tests: map[string]qstate.Entry{
			"suite::with-issue": {
				Name:          "with-issue",
				IssueNumber:   &n,
				IssueURL:      "https://github.com/o/r/issues/42",
				QuarantinedAt: "2026-01-01",
			},
			"suite::without-issue": {
				Name:          "without-issue",
				IssueNumber:   nil,
				IssueURL:      "",
				QuarantinedAt: "2026-01-02",
			},
		},
	}

	entries := buildQuarantinedEntries(state)

	var withIssueEntry, withoutIssueEntry *QuarantinedEntry
	for i := range entries {
		switch entries[i].Name {
		case "with-issue":
			withIssueEntry = &entries[i]
		case "without-issue":
			withoutIssueEntry = &entries[i]
		}
	}

	if withIssueEntry == nil || withoutIssueEntry == nil {
		t.Fatal("expected both entries in output, got nil")
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "entry with IssueNumber=42",
		Should:   "have IssueNum=42 in the output entry",
		Actual:   withIssueEntry.IssueNum,
		Expected: 42,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "entry with nil IssueNumber",
		Should:   "have IssueNum=0 in the output entry",
		Actual:   withoutIssueEntry.IssueNum,
		Expected: 0,
	})
}

// --- Unit tests for buildUnquarantinedEntries (mutation targets) ---

// TestBuildUnquarantinedEntriesIssueNumberNilGivesZero kills the mutation on
// line 434: `e.IssueNumber != nil` → `== nil`. When an unquarantined entry HAS
// an IssueNumber, its value must appear in the output.
func TestBuildUnquarantinedEntriesIssueNumberNilGivesZero(t *testing.T) {
	n := 77
	removed := []qstate.Entry{
		{
			Name:        "unquarantined-with-issue",
			IssueNumber: &n,
			IssueURL:    "https://github.com/o/r/issues/77",
		},
		{
			Name:        "unquarantined-without-issue",
			IssueNumber: nil,
			IssueURL:    "",
		},
	}

	entries := buildUnquarantinedEntries(removed)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two removed entries",
		Should:   "return two unquarantined entries",
		Actual:   len(entries),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "removed entry with IssueNumber=77",
		Should:   "have IssueNum=77 in the first output entry",
		Actual:   entries[0].IssueNum,
		Expected: 77,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "removed entry with nil IssueNumber",
		Should:   "have IssueNum=0 in the second output entry",
		Actual:   entries[1].IssueNum,
		Expected: 0,
	})
}

// --- Unit tests for buildFailureEntries (mutation targets) ---

// TestBuildFailureEntriesExcludesNonFailedStatus kills the mutation on line 451:
// `t.Status != "failed"` → `== "failed"`. Only tests with Status=="failed"
// should appear. A test with Status=="flaky" must be excluded.
func TestBuildFailureEntriesExcludesNonFailedStatus(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-test", Name: "flaky-test", Status: "flaky"},
			{TestID: "suite::passed-test", Name: "passed-test", Status: "passed"},
		},
	}

	entries := buildFailureEntries(res)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with flaky and passed tests (no failed)",
		Should:   "return an empty entries slice",
		Actual:   len(entries),
		Expected: 0,
	})
}

// TestBuildFailureEntriesIncludesFailedStatus provides the counterpart: a
// failed test is included and a flaky test in the same result is not.
func TestBuildFailureEntriesIncludesFailedStatus(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::failed-test", Name: "failed-test", Status: "failed"},
			{TestID: "suite::flaky-test", Name: "flaky-test", Status: "flaky"},
		},
	}

	entries := buildFailureEntries(res)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with one failed and one flaky test",
		Should:   "return exactly one entry (the failed test)",
		Actual:   len(entries),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "failed test in result",
		Should:   "have the failed test name in the entry",
		Actual:   entries[0].Name,
		Expected: "failed-test",
	})
}

// TestBuildFailureEntriesNonNilFailureMessageIncluded kills the mutation on
// line 455: `t.FailureMessage != nil` → `== nil`. When FailureMessage is
// non-nil, its value must appear in the output entry.
func TestBuildFailureEntriesNonNilFailureMessageIncluded(t *testing.T) {
	msg := "expected 1 but got 2"
	res := result.Result{
		Tests: []result.TestEntry{
			{
				TestID:         "suite::failed-test",
				Name:           "failed-test",
				Status:         "failed",
				FailureMessage: &msg,
			},
		},
	}

	entries := buildFailureEntries(res)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "failed test with non-nil FailureMessage",
		Should:   "return one entry",
		Actual:   len(entries),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "failed test with FailureMessage='expected 1 but got 2'",
		Should:   "include the failure message text in the entry",
		Actual:   entries[0].Message,
		Expected: "expected 1 but got 2",
	})
}

// TestBuildFailureEntriesNilFailureMessageGivesEmptyString verifies the
// nil-FailureMessage path: the entry is still created but Message is "".
func TestBuildFailureEntriesNilFailureMessageGivesEmptyString(t *testing.T) {
	res := result.Result{
		Tests: []result.TestEntry{
			{
				TestID:         "suite::failed-test",
				Name:           "failed-test",
				Status:         "failed",
				FailureMessage: nil,
			},
		},
	}

	entries := buildFailureEntries(res)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "failed test with nil FailureMessage",
		Should:   "still return one entry",
		Actual:   len(entries),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "failed test with nil FailureMessage",
		Should:   "have empty string as the Message field",
		Actual:   entries[0].Message,
		Expected: "",
	})
}
