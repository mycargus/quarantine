package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
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
