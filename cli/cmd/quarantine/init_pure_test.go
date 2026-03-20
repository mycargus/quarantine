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
