package git_test

import (
	"os"
	"os/exec"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/git"
)

// --- ParseGitHubURL ---

func TestParseGitHubURLHTTPS(t *testing.T) {
	owner, repo, err := git.ParseGitHubURL("https://github.com/my-org/my-project.git")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "an HTTPS GitHub remote URL",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an HTTPS GitHub remote URL",
		Should:   "return the owner",
		Actual:   owner,
		Expected: "my-org",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an HTTPS GitHub remote URL",
		Should:   "return the repo",
		Actual:   repo,
		Expected: "my-project",
	})
}

func TestParseGitHubURLHTTPSNoGitSuffix(t *testing.T) {
	owner, repo, err := git.ParseGitHubURL("https://github.com/my-org/my-project")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "an HTTPS GitHub remote URL without .git suffix",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an HTTPS GitHub remote URL without .git suffix",
		Should:   "return the owner",
		Actual:   owner,
		Expected: "my-org",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an HTTPS GitHub remote URL without .git suffix",
		Should:   "return the repo",
		Actual:   repo,
		Expected: "my-project",
	})
}

func TestParseGitHubURLSSH(t *testing.T) {
	owner, repo, err := git.ParseGitHubURL("git@github.com:my-org/my-project.git")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "an SSH GitHub remote URL",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an SSH GitHub remote URL",
		Should:   "return the owner",
		Actual:   owner,
		Expected: "my-org",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an SSH GitHub remote URL",
		Should:   "return the repo",
		Actual:   repo,
		Expected: "my-project",
	})
}

func TestParseGitHubURLNonGitHub(t *testing.T) {
	_, _, err := git.ParseGitHubURL("https://gitlab.com/my-org/my-project.git")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-GitHub remote URL (GitLab)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseGitHubURLNonGitHubErrorMessage(t *testing.T) {
	rawURL := "https://gitlab.com/my-org/my-project.git"
	_, _, err := git.ParseGitHubURL(rawURL)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a non-GitHub remote URL (GitLab)",
		Should:   "include the raw URL in the error message",
		Actual:   err.Error(),
		Expected: "remote 'origin' is not a GitHub URL: https://gitlab.com/my-org/my-project.git",
	})
}

func TestParseGitHubURLEmpty(t *testing.T) {
	_, _, err := git.ParseGitHubURL("")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty URL",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseGitHubURLSSHMissingOwner(t *testing.T) {
	_, _, err := git.ParseGitHubURL("git@github.com:/repo.git")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an SSH URL with empty owner (git@github.com:/repo.git)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseGitHubURLHTTPSSinglePathComponent(t *testing.T) {
	_, _, err := git.ParseGitHubURL("https://github.com/owner")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an HTTPS URL with only one path component (no repo)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- ParseRemote integration tests (run git subprocess) ---

func TestParseRemoteNotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	// No git init — just an empty directory.

	_, _, err := git.ParseRemote(dir)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a directory without a .git folder",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseRemoteNonGitHubOrigin(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "remote", "add", "origin", "https://gitlab.com/my-org/my-project.git")

	_, _, err := git.ParseRemote(dir)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a git repo with a GitLab origin remote",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// runGit is a test helper that runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
