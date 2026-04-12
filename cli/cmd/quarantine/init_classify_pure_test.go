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
		Should:   "mention .quarantine/config.yml as the manual config option",
		Actual:   strings.Contains(msg, ".quarantine/config.yml"),
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

// --- extractURLFromError unit tests ---

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
