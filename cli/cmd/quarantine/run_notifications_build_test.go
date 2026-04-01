package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
)

// --- Unit tests for buildRunURL and renderIssueBody Build line ---

func TestBuildRunURL(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "a valid repo and numeric run ID",
		Should:   "return the GitHub Actions run URL",
		Actual:   buildRunURL("owner/repo", "12345"),
		Expected: "https://github.com/owner/repo/actions/runs/12345",
	})
}

func TestBuildRunURLLocalRunID(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "a run ID starting with 'local-'",
		Should:   "return an empty string",
		Actual:   buildRunURL("owner/repo", "local-abc123"),
		Expected: "",
	})
}

func TestBuildRunURLEmptyRepo(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "an empty repo string",
		Should:   "return an empty string",
		Actual:   buildRunURL("", "12345"),
		Expected: "",
	})
}

func TestBuildRunURLEmptyRunID(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "an empty run ID",
		Should:   "return an empty string",
		Actual:   buildRunURL("owner/repo", ""),
		Expected: "",
	})
}

func TestRenderIssueBodyWithBuildURL(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "main",
		CommitSHA: "abc1234",
		PRNumber:  42,
		RunURL:    "https://github.com/owner/repo/actions/runs/12345",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty RunURL",
		Should:   "include the Build line with a clickable link",
		Actual:   strings.Contains(body, "**Build:** <https://github.com/owner/repo/actions/runs/12345>"),
		Expected: true,
	})
}

func TestRenderIssueBodyOmitsBuildLineWhenRunURLEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "main",
		CommitSHA: "abc1234",
		PRNumber:  42,
		RunURL:    "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty RunURL",
		Should:   "omit the Build line entirely",
		Actual:   strings.Contains(body, "**Build:**"),
		Expected: false,
	})
}

// --- Unit tests for renderIssueBody conditional sections (mutation targets) ---

// TestRenderIssueBodyOmitsRetryResultsWhenEmpty kills the mutation on line 131:
// `len(data.Retries) > 0` → `>= 0`. An empty slice must NOT produce the section.
func TestRenderIssueBodyOmitsRetryResultsWhenEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Retries:   []result.RetryEntry{},
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty Retries slice",
		Should:   "not include the Retry Results section",
		Actual:   strings.Contains(body, "### Retry Results"),
		Expected: false,
	})
}

// TestRenderIssueBodyIncludesRetryResultsWhenNonEmpty provides the counterpart
// assertion: a non-empty Retries slice must produce the section.
func TestRenderIssueBodyIncludesRetryResultsWhenNonEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Retries: []result.RetryEntry{
			{Attempt: 1, Status: "failed", DurationMs: 100},
		},
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty Retries slice",
		Should:   "include the Retry Results section",
		Actual:   strings.Contains(body, "### Retry Results"),
		Expected: true,
	})
}

// TestRenderIssueBodyOmitsFailureMessageWhenEmpty kills the mutation on line 139:
// `data.FailureMessage != ""` → `== ""`. An empty string must omit the section.
func TestRenderIssueBodyOmitsFailureMessageWhenEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:         "src/foo.test.js::Foo::bar",
		Suite:          "src/foo.test.js",
		Name:           "bar",
		Timestamp:      "2026-03-28T00:00:00Z",
		FailureMessage: "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty FailureMessage",
		Should:   "not include the Failure Message section",
		Actual:   strings.Contains(body, "### Failure Message"),
		Expected: false,
	})
}

// TestRenderIssueBodyIncludesFailureMessageWhenNonEmpty provides the counterpart:
// a non-empty FailureMessage must produce the section and the message text.
func TestRenderIssueBodyIncludesFailureMessageWhenNonEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:         "src/foo.test.js::Foo::bar",
		Suite:          "src/foo.test.js",
		Name:           "bar",
		Timestamp:      "2026-03-28T00:00:00Z",
		FailureMessage: "expected true but got false",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty FailureMessage",
		Should:   "include the Failure Message section heading",
		Actual:   strings.Contains(body, "### Failure Message"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty FailureMessage",
		Should:   "include the failure message text in the body",
		Actual:   strings.Contains(body, "expected true but got false"),
		Expected: true,
	})
}

// --- Unit tests for renderPRComment conditional sections (mutation targets) ---

// TestRenderPRCommentOmitsNewlyFlakySectionWhenEmpty kills the mutation on line 210:
// `len(data.NewlyFlaky) > 0` → `>= 0`. Empty slice must omit the section.
func TestRenderPRCommentOmitsNewlyFlakySectionWhenEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:      3,
		Passed:     3,
		NewlyFlaky: []FlakyEntry{},
		Version:    "0.1.0",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty NewlyFlaky slice",
		Should:   "not include the 'New Flaky Tests Detected' section",
		Actual:   strings.Contains(comment, "New Flaky Tests Detected"),
		Expected: false,
	})
}

// TestRenderPRCommentOmitsQuarantinedSectionWhenEmpty kills the mutation on line 233:
// `len(data.QuarantinedTests) > 0` → `>= 0`. Empty slice must omit the section.
func TestRenderPRCommentOmitsQuarantinedSectionWhenEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:            2,
		Passed:           2,
		QuarantinedTests: []QuarantinedEntry{},
		Version:          "0.1.0",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty QuarantinedTests slice",
		Should:   "not include the 'Quarantined Tests (Excluded)' section",
		Actual:   strings.Contains(comment, "Quarantined Tests (Excluded)"),
		Expected: false,
	})
}

// TestRenderPRCommentIncludesQuarantinedSectionWhenNonEmpty provides the
// counterpart: a non-empty slice must produce the section.
func TestRenderPRCommentIncludesQuarantinedSectionWhenNonEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:   2,
		Passed:  1,
		Version: "0.1.0",
		QuarantinedTests: []QuarantinedEntry{
			{Name: "some flaky test", IssueURL: "https://github.com/o/r/issues/5", IssueNum: 5, Since: "2026-01-01"},
		},
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty QuarantinedTests slice",
		Should:   "include the 'Quarantined Tests (Excluded)' section",
		Actual:   strings.Contains(comment, "Quarantined Tests (Excluded)"),
		Expected: true,
	})
}

// TestRenderPRCommentOmitsFailuresSectionWhenEmpty kills the mutation on line 254:
// `len(data.Failures) > 0` → `>= 0`. Empty slice must omit the section.
func TestRenderPRCommentOmitsFailuresSectionWhenEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:    2,
		Passed:   2,
		Failures: []FailureEntry{},
		Version:  "0.1.0",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty Failures slice",
		Should:   "not include the 'Real Failures' section",
		Actual:   strings.Contains(comment, "Real Failures"),
		Expected: false,
	})
}

// TestRenderPRCommentIncludesFailuresSectionWhenNonEmpty provides the counterpart:
// a non-empty Failures slice must produce the section with the test name.
func TestRenderPRCommentIncludesFailuresSectionWhenNonEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:   1,
		Failed:  1,
		Version: "0.1.0",
		Failures: []FailureEntry{
			{Name: "should not crash", Message: "panic: nil pointer"},
		},
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty Failures slice",
		Should:   "include the 'Real Failures' section",
		Actual:   strings.Contains(comment, "Real Failures"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty Failures slice",
		Should:   "include the failing test name",
		Actual:   strings.Contains(comment, "should not crash"),
		Expected: true,
	})
}

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
