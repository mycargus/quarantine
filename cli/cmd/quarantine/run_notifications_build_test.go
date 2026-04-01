package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
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
