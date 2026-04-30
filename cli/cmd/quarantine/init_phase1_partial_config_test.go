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

// --- Scenario 174: phase 1 writes partial config, exits 2 ---
//
// Init phase 1 must NOT make any GitHub API call. We satisfy this requirement
// by pointing the API base URL at a server that records every request — the
// test fails if any request is observed. The origin host is irrelevant to
// init's behavior; we use a Gerrit URL to demonstrate that explicitly.

func TestInitPhase1WritesPartialConfigAndExitsTwoForGerritOrigin(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	// jest detection so frameworks slice is non-empty (drives test_suites stub).
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	// Tripwire server: records request count. If init touches the GitHub API
	// during phase 1, this counter increments and the assertion below fires.
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
		Given:    "init phase 1 (no owner/repo in config yet)",
		Should:   "make zero GitHub API calls",
		Actual:   int(atomic.LoadInt32(&apiCallCount)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init phase 1 finishes by writing partial config",
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

	riteway.Assert(t, riteway.Case[string]{
		Given:    "init phase 1 wrote .quarantine/config.yml",
		Should:   "match the partial config formatter output for the gerrit-origin/jest case",
		Actual:   string(configBytes),
		Expected: formatPartialConfig([]string{"jest"}, nil),
	})
}

// exitCodeFromError extracts the exit code from an error returned by
// runInit / runRun / runDoctor. Mirrors the logic in main.go.
// This is a pure function — no I/O.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if code, ok := err.(exitCodeError); ok {
		return int(code)
	}
	return 2
}
