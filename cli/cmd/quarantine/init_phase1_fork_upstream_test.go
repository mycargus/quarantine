package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/git"
)

// --- Scenario 184: phase 1 emits hint comments for fork+upstream remotes ---
//
// When the developer runs `quarantine init` in a repository with two github.com
// remotes (e.g. a fork's `origin` and the canonical `upstream`), phase 1 must:
//   - render both remotes as advisory YAML hint comments under the `github` block,
//   - leave `github.owner` and `github.repo` empty (never pre-fill from a candidate),
//   - exit 2 with the same hand-edit instructions used in scenario 174.
//
// This is a CLI-orchestrator regression test. The hint formatting itself is
// covered by `TestFormatPartialConfigWithHints` (pure unit) and the git-remote
// scan logic is covered by `parseGitRemoteVerbose` unit tests. This test wires
// them together end-to-end via the `init` cobra command.

func TestInitPhase1EmitsHintsForForkAndUpstreamGitHubRemotes(t *testing.T) {
	dir := t.TempDir()
	// Two github.com remotes: an SSH fork origin and an HTTPS upstream.
	setupFakeGitRepo(t, dir, "git@github.com:mhargiss/quarantine.git")
	runCmd(t, dir, "git", "remote", "add", "upstream", "https://github.com/mycargus/quarantine.git")
	// jest detection so frameworks slice is non-empty (drives test_suites stub).
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	// Tripwire server: phase 1 must make zero GitHub API calls.
	var apiCallCount int32
	tripwire := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(tripwire.Close)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": tripwire.URL,
		},
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 with fork+upstream github.com remotes",
		Should:   "make zero GitHub API calls",
		Actual:   int(atomic.LoadInt32(&apiCallCount)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 finishes by writing partial config with hints",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 has finished",
		Should:   "print the phase-1 hand-edit message to stdout",
		Actual:   strings.Contains(stdout, formatPhase1ExitMessage(true)),
		Expected: true,
	})

	configBytes, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 writes a partial config",
		Should:   "create .quarantine/config.yml on disk",
		Actual:   readErr == nil,
		Expected: true,
	})

	expectedHints := []git.GitHubRemoteHint{
		{Name: "origin", Owner: "mhargiss", Repo: "quarantine"},
		{Name: "upstream", Owner: "mycargus", Repo: "quarantine"},
	}
	riteway.Assert(t, riteway.Case[string]{
		Given:    "init phase 1 wrote .quarantine/config.yml for a fork+upstream repo",
		Should:   "match the partial config formatter output for jest + both hints",
		Actual:   string(configBytes),
		Expected: formatPartialConfig([]string{"jest"}, expectedHints),
	})
}
