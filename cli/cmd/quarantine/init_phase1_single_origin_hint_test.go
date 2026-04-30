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

// --- Scenario 185: phase 1 emits one hint comment for a single github.com origin ---
//
// The most common bootstrapping path: a developer with a single github.com origin
// runs `quarantine init`. Phase 1 must:
//   - render exactly one advisory YAML hint comment for the origin under the github block,
//   - leave `github.owner` and `github.repo` empty (never pre-fill from the candidate),
//   - exit 2 with the same hand-edit instructions used in scenario 174.
//
// The hint formatter and the git-remote scan are covered by their own pure tests
// (`TestFormatPartialConfigWithHints` and `parseGitRemoteVerbose` unit tests). This
// test is a CLI-orchestrator regression for the single-origin case, mirroring the
// fork+upstream test (scenario 184) but with only `origin` configured.

func TestInitPhase1EmitsSingleHintForLoneGitHubOrigin(t *testing.T) {
	dir := t.TempDir()
	// Single github.com origin (HTTPS).
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")
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
		Given:    "init phase 1 with a single github.com origin",
		Should:   "make zero GitHub API calls",
		Actual:   int(atomic.LoadInt32(&apiCallCount)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 finishes by writing partial config with one hint",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 has finished",
		Should:   "print the phase-1 hand-edit message to stdout",
		Actual:   strings.Contains(stdout, formatPhase1ExitMessage()),
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
		{Name: "origin", Owner: "my-org", Repo: "my-project"},
	}
	riteway.Assert(t, riteway.Case[string]{
		Given:    "init phase 1 wrote .quarantine/config.yml for a single-origin repo",
		Should:   "match the partial config formatter output for jest + the single origin hint",
		Actual:   string(configBytes),
		Expected: formatPartialConfig([]string{"jest"}, expectedHints),
	})
}
