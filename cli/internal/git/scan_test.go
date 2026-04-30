package git

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- parseGitRemoteVerbose unit tests ---

func TestParseGitRemoteVerboseEmptyOutput(t *testing.T) {
	hints := parseGitRemoteVerbose("")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty `git remote -v` output (no remotes configured)",
		Should:   "return zero hints",
		Actual:   len(hints),
		Expected: 0,
	})
}

func TestParseGitRemoteVerboseSingleGitHubRemote(t *testing.T) {
	output := "origin\thttps://github.com/my-org/my-project.git (fetch)\n" +
		"origin\thttps://github.com/my-org/my-project.git (push)\n"

	hints := parseGitRemoteVerbose(output)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one github.com remote that appears as both fetch and push",
		Should:   "return exactly one hint (deduplicated by remote name)",
		Actual:   len(hints),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[GitHubRemoteHint]{
		Given:    "origin -> https://github.com/my-org/my-project.git",
		Should:   "yield a hint with name=origin, owner=my-org, repo=my-project",
		Actual:   hints[0],
		Expected: GitHubRemoteHint{Name: "origin", Owner: "my-org", Repo: "my-project"},
	})
}

func TestParseGitRemoteVerboseForkAndUpstream(t *testing.T) {
	output := "origin\tgit@github.com:mhargiss/quarantine.git (fetch)\n" +
		"origin\tgit@github.com:mhargiss/quarantine.git (push)\n" +
		"upstream\thttps://github.com/mycargus/quarantine.git (fetch)\n" +
		"upstream\thttps://github.com/mycargus/quarantine.git (push)\n"

	hints := parseGitRemoteVerbose(output)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two github.com remotes (fork origin + upstream)",
		Should:   "return exactly two hints",
		Actual:   len(hints),
		Expected: 2,
	})
}

func TestParseGitRemoteVerboseSkipsNonGitHubRemote(t *testing.T) {
	output := "origin\thttps://gerrit.example.com/foo/bar.git (fetch)\n" +
		"origin\thttps://gerrit.example.com/foo/bar.git (push)\n"

	hints := parseGitRemoteVerbose(output)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a single non-github.com remote (gerrit)",
		Should:   "return zero hints",
		Actual:   len(hints),
		Expected: 0,
	})
}

func TestParseGitRemoteVerboseMixedRemotes(t *testing.T) {
	output := "origin\thttps://gerrit.example.com/foo/bar.git (fetch)\n" +
		"origin\thttps://gerrit.example.com/foo/bar.git (push)\n" +
		"github\thttps://github.com/my-org/my-project.git (fetch)\n" +
		"github\thttps://github.com/my-org/my-project.git (push)\n"

	hints := parseGitRemoteVerbose(output)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one gerrit remote and one github.com remote",
		Should:   "return exactly one hint (the github.com one)",
		Actual:   len(hints),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the github remote is named 'github'",
		Should:   "yield a hint with that remote name",
		Actual:   hints[0].Name,
		Expected: "github",
	})
}
