package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 186: phase 1 with no GitHub token set ---
//
// Phase 1 of `quarantine init` writes a partial `.quarantine/config.yml` and
// exits 2 with hand-edit instructions. Because phase 1 makes no GitHub API
// call, the absence of a GitHub token MUST NOT prevent the partial config
// from being written. Instead, the printed exit message must include an
// additional note telling the user they will need a token before re-running
// init — surfacing both problems (missing owner/repo + missing token) at once.
//
// Per ADR-037 / scenario 174: phase 1 is offline. Token validation is
// deferred to phase 2 (when owner/repo are populated and the CLI must
// reach api.github.com).

func TestInitPhase1WithoutTokenWritesConfigAndIncludesTokenNote(t *testing.T) {
	dir := t.TempDir()
	// Any non-github origin so we exercise the no-hints branch of formatPartialConfig.
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	// jest detection so frameworks slice is non-empty (drives test_suites stub).
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	// Tripwire server: phase 1 must make zero GitHub API calls even when
	// no token is set.
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
			"QUARANTINE_GITHUB_TOKEN":        "",
			"GITHUB_TOKEN":                   "",
			"QUARANTINE_GITHUB_API_BASE_URL": tripwire.URL,
		},
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 with no GitHub token set",
		Should:   "make zero GitHub API calls (phase 1 is offline)",
		Actual:   int(atomic.LoadInt32(&apiCallCount)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 finishes by writing partial config without a token",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 has finished without a token",
		Should:   "print the canonical phase-1 hand-edit message including the token note",
		Actual:   strings.Contains(stdout, formatPhase1ExitMessage(false)),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 has finished without a token",
		Should:   "include the token note substring in stdout",
		Actual:   strings.Contains(stdout, "Note: You will also need a GitHub token."),
		Expected: true,
	})

	configBytes, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init phase 1 writes a partial config without a token",
		Should:   "create .quarantine/config.yml on disk",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "init phase 1 wrote .quarantine/config.yml without a token",
		Should:   "match the partial config formatter output (jest, no hints)",
		Actual:   string(configBytes),
		Expected: formatPartialConfig([]string{"jest"}, nil),
	})
}
