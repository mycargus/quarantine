package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestDetectPRNumberFromEventFile(t *testing.T) {
	// pull_request_target event uses .number at top level.
	eventJSON := `{"number":55,"action":"opened"}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, err := detectPRNumber(0, eventPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pull_request_target event with .number=55",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "pull_request_target event with .number=55",
		Should:   "return 55",
		Actual:   n,
		Expected: 55,
	})
}

func TestDetectPRNumberPrefersFlag(t *testing.T) {
	// Even if event file has a different PR number, flag wins.
	eventJSON := `{"pull_request":{"number":77}}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, _ := detectPRNumber(42, eventPath)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "--pr flag=42 and event file contains PR number 77",
		Should:   "prefer the flag value (42)",
		Actual:   n,
		Expected: 42,
	})
}

// Test that the PR comment marker is on the FIRST line (spec requirement).
func TestRenderPRCommentMarkerIsFirstLine(t *testing.T) {
	data := PRCommentData{
		Total:   1,
		Passed:  1,
		Version: "0.1.0",
	}
	comment := renderPRComment(data, PRCommentMarker)
	firstLine := strings.SplitN(comment, "\n", 2)[0]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "PR comment with no flaky tests",
		Should:   "have <!-- quarantine-bot --> as the first line",
		Actual:   firstLine,
		Expected: "<!-- quarantine-bot -->",
	})
}

func TestRenderPRCommentNilFlakySection(t *testing.T) {
	// When there are no flaky tests, the flaky section should not appear.
	data := PRCommentData{
		Total:      2,
		Passed:     2,
		NewlyFlaky: nil,
		Version:    "0.1.0",
	}
	comment := renderPRComment(data, PRCommentMarker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment with no flaky tests",
		Should:   "not contain 'New Flaky Tests Detected' section",
		Actual:   strings.Contains(comment, "New Flaky Tests Detected"),
		Expected: false,
	})
}

func TestBuildQuarantinedEntriesNilState(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil quarantine state",
		Should:   "return nil without panicking",
		Actual:   buildQuarantinedEntries(nil) == nil,
		Expected: true,
	})
}

// Verify that the JSON event parsing is correct.
func TestDetectPRNumberFromEventFileInvalidJSON(t *testing.T) {
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, _ := detectPRNumber(0, eventPath)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "event file with invalid JSON",
		Should:   "return 0 (skip gracefully)",
		Actual:   n,
		Expected: 0,
	})
}

// TestDetectPRNumberPullRequestZeroFallsBack verifies that when the event file
// has pull_request.number == 0, the code falls through to evt.Number.
// Kills mutation on line 57: `evt.PullRequest.Number > 0` → `>= 0`.
func TestDetectPRNumberPullRequestZeroFallsBack(t *testing.T) {
	// Event JSON: pull_request exists but number is 0; outer number is 7.
	eventJSON := `{"pull_request":{"number":0},"number":7}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event: %v", err)
	}

	n, err := detectPRNumber(0, eventPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pull_request.number is 0 in event JSON",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "pull_request.number is 0 (not a valid PR number)",
		Should:   "fall through to evt.Number (7), not return 0",
		Actual:   n,
		Expected: 7,
	})
}

// TestRenderIssueBodyPRNumberZeroOmitsPRLine verifies that when PRNumber is 0
// the "**PR:**" line is NOT rendered in the issue body.
// Kills mutation on line 86: `data.PRNumber > 0` → `data.PRNumber >= 0`.
func TestRenderIssueBodyPRNumberZeroOmitsPRLine(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "main",
		CommitSHA: "abc1234",
		PRNumber:  0, // zero — should not appear in body
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PRNumber is 0",
		Should:   "NOT include '**PR:**' line in issue body",
		Actual:   !strings.Contains(body, "**PR:**"),
		Expected: true,
	})
}

// TestRenderIssueBodyPositivePRNumberShowsPRLine verifies that a non-zero PR
// number IS rendered, providing the counterpart assertion.
func TestRenderIssueBodyPositivePRNumberShowsPRLine(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "main",
		CommitSHA: "abc1234",
		PRNumber:  42,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PRNumber is 42",
		Should:   "include '**PR:** #42' line in issue body",
		Actual:   strings.Contains(body, "**PR:** #42"),
		Expected: true,
	})
}

// TestRenderIssueBodyDetectedInWithOnlyBranch kills the mutation on line 122:
// `data.Branch != "" || data.CommitSHA != ""` → `&&`.
// With OR, having ONLY a Branch (empty CommitSHA) is sufficient to render the
// "Detected in" line. The && mutation would suppress it.
func TestRenderIssueBodyDetectedInWithOnlyBranch(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "feature/my-branch",
		CommitSHA: "", // empty
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Branch is set but CommitSHA is empty",
		Should:   "still include the 'Detected in:' line",
		Actual:   strings.Contains(body, "**Detected in:**"),
		Expected: true,
	})
}

// TestRenderIssueBodyDetectedInWithOnlyCommitSHA kills the mutation on line 122:
// `data.Branch != "" || data.CommitSHA != ""` → `&&`.
// With OR, having ONLY a CommitSHA (empty Branch) is sufficient to render the
// "Detected in" line. The && mutation would suppress it.
func TestRenderIssueBodyDetectedInWithOnlyCommitSHA(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "", // empty
		CommitSHA: "deadbeef",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "CommitSHA is set but Branch is empty",
		Should:   "still include the 'Detected in:' line",
		Actual:   strings.Contains(body, "**Detected in:**"),
		Expected: true,
	})
}

// TestRenderPRCommentOmitsNewToPRFlakySectionWhenEmpty kills the mutation on
// line 221: `len(data.NewToPRFlaky) > 0` → `>= 0`.
// An empty NewToPRFlaky slice must NOT produce the "Flaky Tests in This PR"
// section.
func TestRenderPRCommentOmitsNewToPRFlakySectionWhenEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:        3,
		Passed:       3,
		NewToPRFlaky: []FlakyEntry{},
		Version:      "0.1.0",
	}, PRCommentMarker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty NewToPRFlaky slice",
		Should:   "not include the 'Flaky Tests in This PR' section",
		Actual:   strings.Contains(comment, "Flaky Tests in This PR"),
		Expected: false,
	})
}

// TestRenderPRCommentIncludesNewToPRFlakySectionWhenNonEmpty provides the
// counterpart: a non-empty NewToPRFlaky slice must produce the section.
func TestRenderPRCommentIncludesNewToPRFlakySectionWhenNonEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:   2,
		Passed:  1,
		Version: "0.1.0",
		NewToPRFlaky: []FlakyEntry{
			{Name: "brand new flaky test"},
		},
	}, PRCommentMarker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty NewToPRFlaky slice",
		Should:   "include the 'Flaky Tests in This PR' section",
		Actual:   strings.Contains(comment, "Flaky Tests in This PR"),
		Expected: true,
	})
}

// TestRenderPRCommentOmitsUnquarantinedSectionWhenEmpty kills the mutation on
// line 244: `len(data.UnquarantinedTests) > 0` → `>= 0`.
// An empty UnquarantinedTests slice must NOT produce the "Unquarantined Tests"
// section.
func TestRenderPRCommentOmitsUnquarantinedSectionWhenEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:              2,
		Passed:             2,
		UnquarantinedTests: []UnquarantinedEntry{},
		Version:            "0.1.0",
	}, PRCommentMarker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty UnquarantinedTests slice",
		Should:   "not include the 'Unquarantined Tests' section",
		Actual:   strings.Contains(comment, "### Unquarantined Tests"),
		Expected: false,
	})
}

// TestRenderPRCommentIncludesUnquarantinedSectionWhenNonEmpty provides the
// counterpart: a non-empty UnquarantinedTests slice must produce the section.
func TestRenderPRCommentIncludesUnquarantinedSectionWhenNonEmpty(t *testing.T) {
	comment := renderPRComment(PRCommentData{
		Total:   2,
		Passed:  2,
		Version: "0.1.0",
		UnquarantinedTests: []UnquarantinedEntry{
			{Name: "formerly-flaky-test", IssueURL: "https://github.com/o/r/issues/9", IssueNum: 9},
		},
	}, PRCommentMarker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty UnquarantinedTests slice",
		Should:   "include the 'Unquarantined Tests' section",
		Actual:   strings.Contains(comment, "### Unquarantined Tests"),
		Expected: true,
	})
}

// TestRenderIssueBodyOmitsDetectedInWhenEmpty verifies that when both branch
// and commitSHA are empty strings, the "Detected in:" line is omitted entirely.
func TestRenderIssueBodyOmitsDetectedInWhenEmpty(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/foo.test.js::Foo::bar",
		Suite:     "src/foo.test.js",
		Name:      "bar",
		Timestamp: "2026-03-28T00:00:00Z",
		Branch:    "",
		CommitSHA: "",
		PRNumber:  42,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty branch and commitSHA",
		Should:   "omit the 'Detected in:' line entirely",
		Actual:   strings.Contains(body, "**Detected in:**"),
		Expected: false,
	})

	// Other fields should still render normally.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty branch and commitSHA",
		Should:   "still include the Test ID line",
		Actual:   strings.Contains(body, "**Test ID:** `src/foo.test.js::Foo::bar`"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty branch and commitSHA",
		Should:   "still include the PR line",
		Actual:   strings.Contains(body, "**PR:** #42"),
		Expected: true,
	})
}
