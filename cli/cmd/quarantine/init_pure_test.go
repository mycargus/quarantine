package main

import (
	"fmt"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- classifyGitHubError unit tests ---

func TestClassifyGitHubError401(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("401 Unauthorized"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 401 error from the GitHub API",
		Should:   "return message about invalid or expired token",
		Actual:   strings.Contains(msg, "invalid or expired"),
		Expected: true,
	})
}

func TestClassifyGitHubError403(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("403 Forbidden"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 403 error from the GitHub API",
		Should:   "return message about lacking permission including owner/repo",
		Actual:   strings.Contains(msg, "lacks permission") && strings.Contains(msg, "my-org/my-repo"),
		Expected: true,
	})
}

func TestClassifyGitHubErrorNetworkFailure(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("connection refused"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a connection refused error",
		Should:   "return message about unable to reach GitHub API",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})
}

func TestClassifyGitHubErrorGeneric(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("unexpected status 500"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an unrecognized GitHub API error",
		Should:   "return generic GitHub API error message",
		Actual:   strings.Contains(msg, "GitHub API error"),
		Expected: true,
	})
}

// --- classifyGitRemoteError unit tests ---

func TestClassifyGitRemoteErrorNotGitRepo(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("not a git repository: /some/path"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a not-a-git-repository error",
		Should:   "return message about running from a git repository",
		Actual:   strings.Contains(msg, "Not a git repository"),
		Expected: true,
	})
}

func TestClassifyGitRemoteErrorNotGitHubURL(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("remote 'origin' is not a GitHub URL: https://gitlab.com/foo/bar.git"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-GitHub remote URL error",
		Should:   "return message containing the URL",
		Actual:   strings.Contains(msg, "gitlab.com/foo/bar.git"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-GitHub remote URL error",
		Should:   "mention manual config option",
		Actual:   strings.Contains(msg, "quarantine.yml"),
		Expected: true,
	})
}

func TestClassifyGitRemoteErrorGeneric(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("exec: git not found"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an unrecognized git remote error",
		Should:   "return message containing the original error",
		Actual:   strings.Contains(msg, "git not found"),
		Expected: true,
	})
}

// --- formatInitSummary unit tests ---

func TestFormatInitSummaryNewBranch(t *testing.T) {
	summary := formatInitSummary("jest", 3, "junit.xml", false)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "contain framework jest",
		Actual:   strings.Contains(summary, "Framework:  jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "show branch as created",
		Actual:   strings.Contains(summary, "quarantine/state (created)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "contain quarantine doctor next step",
		Actual:   strings.Contains(summary, "quarantine doctor"),
		Expected: true,
	})
}

func TestFormatInitSummaryExistingBranch(t *testing.T) {
	summary := formatInitSummary("rspec", 5, "custom.xml", true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec, retries=5, existing branch",
		Should:   "show branch as already exists",
		Actual:   strings.Contains(summary, "quarantine/state (already exists)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec, retries=5, existing branch",
		Should:   "contain retries value",
		Actual:   strings.Contains(summary, "Retries:    5"),
		Expected: true,
	})
}

// --- jestRecommendation unit tests ---

func TestJestRecommendationForJest(t *testing.T) {
	rec := jestRecommendation("jest")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework is jest",
		Should:   "return non-empty recommendation",
		Actual:   rec != "",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework is jest",
		Should:   "contain jest-junit configuration",
		Actual:   strings.Contains(rec, "jest-junit"),
		Expected: true,
	})
}

func TestJestRecommendationForRSpec(t *testing.T) {
	rec := jestRecommendation("rspec")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework is rspec",
		Should:   "return empty string",
		Actual:   rec,
		Expected: "",
	})
}

func TestJestRecommendationForVitest(t *testing.T) {
	rec := jestRecommendation("vitest")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework is vitest",
		Should:   "return empty string",
		Actual:   rec,
		Expected: "",
	})
}

// --- frameworkWorkflowSnippet unit tests ---
// Kills mutations on lines 307 (jest constant) and 309 (rspec constant).

func TestFrameworkWorkflowSnippetJest(t *testing.T) {
	snippet := frameworkWorkflowSnippet("jest", "junit.xml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework jest",
		Should:   "contain full jest CI command with jest-junit reporters",
		Actual:   strings.Contains(snippet, "jest --ci --reporters=default --reporters=jest-junit"),
		Expected: true,
	})
}

func TestFrameworkWorkflowSnippetRSpec(t *testing.T) {
	snippet := frameworkWorkflowSnippet("rspec", "results/rspec.xml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework rspec with custom junitxml path",
		Should:   "contain rspec command with RspecJunitFormatter and the junitxml path",
		Actual:   strings.Contains(snippet, "rspec --format RspecJunitFormatter --out results/rspec.xml"),
		Expected: true,
	})
}

func TestFrameworkWorkflowSnippetVitest(t *testing.T) {
	snippet := frameworkWorkflowSnippet("vitest", "junit.xml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework vitest",
		Should:   "contain vitest run command with junit reporter",
		Actual:   strings.Contains(snippet, "vitest run --reporter=junit"),
		Expected: true,
	})
}

// --- extractURLFromError unit tests ---
// Kills mutation on line 261: `idx == -1` → `idx != -1`.

func TestExtractURLFromErrorWithPrefix(t *testing.T) {
	errMsg := "remote 'origin' is not a GitHub URL: https://gitlab.com/foo/bar.git"
	url := extractURLFromError(errMsg)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "error message containing 'not a GitHub URL: <url>'",
		Should:   "return only the URL portion after the prefix",
		Actual:   url,
		Expected: "https://gitlab.com/foo/bar.git",
	})
}

func TestExtractURLFromErrorWithoutPrefix(t *testing.T) {
	errMsg := "some unrelated error message"
	url := extractURLFromError(errMsg)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "error message not containing the expected prefix",
		Should:   "return the full error message unchanged",
		Actual:   url,
		Expected: errMsg,
	})
}

// --- classifyGitHubError network error variants ---
// Kills mutation on line 221: `||` → `&&`.

func TestClassifyGitHubErrorTimeoutOnly(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("request timeout exceeded"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a timeout error (without 'failed' keyword)",
		Should:   "return message about unable to reach GitHub API",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})
}

func TestClassifyGitHubErrorFailedOnly(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("dial failed: no route to host"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 'failed' error (without 'timeout' keyword)",
		Should:   "return message about unable to reach GitHub API",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})
}
