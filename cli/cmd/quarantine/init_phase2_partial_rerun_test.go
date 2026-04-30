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

// --- Scenario 187: re-run on partial config with owner/repo still empty ---
//
// A developer who ran `quarantine init` (phase 1) but has not yet hand-edited
// the config re-runs init accidentally. Init must:
//   - Not overwrite the existing config (any user edits are preserved).
//   - Print the same hand-edit instructions as Scenario 174.
//   - Exit with code 2.
//   - Make zero GitHub API calls (owner/repo are unknown).
//
// We prove "not overwrite" by embedding a sentinel comment in the pre-existing
// config and asserting the file's bytes are byte-for-byte unchanged after
// `init` runs.

// preCreatePartialConfigWithSentinel writes a `.quarantine/config.yml` whose
// `github.owner` and `github.repo` values are empty (mirroring phase 1 output)
// and includes a sentinel comment that phase 1's `formatPartialConfig` would
// never emit. If init overwrites the file, the sentinel disappears and the
// byte-equality assertion fails.
func preCreatePartialConfigWithSentinel(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir+"/.quarantine", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `# user-edit-marker-do-not-overwrite
version: 1

github:
  owner: # set to your GitHub organization or user
  repo:  # set to your GitHub repository name

issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state

test_suites:
  - name: jest
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
    retries: 3
`
	if err := os.WriteFile(dir+"/.quarantine/config.yml", []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestInitRerunOnPartialConfigPreservesFileAndExitsTwo(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	preCreatePartialConfigWithSentinel(t, dir)
	originalConfig, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	if readErr != nil {
		t.Fatalf("read original config: %v", readErr)
	}

	// Tripwire server: records request count. owner/repo are empty, so init
	// must not call any GitHub API. Any request increments the counter.
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
		Given:    "init re-runs on a partial config with empty owner/repo",
		Should:   "make zero GitHub API calls",
		Actual:   int(atomic.LoadInt32(&apiCallCount)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "init re-runs on a partial config with empty owner/repo",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init re-runs on a partial config with empty owner/repo and a token set",
		Should:   "print the phase-1 hand-edit message (token-set variant) on stdout",
		Actual:   strings.Contains(stdout, formatPhase1ExitMessage(true)),
		Expected: true,
	})

	currentConfig, currentReadErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init re-runs on a partial config",
		Should:   "leave .quarantine/config.yml readable on disk",
		Actual:   currentReadErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "init re-runs on a partial config with a sentinel comment",
		Should:   "leave .quarantine/config.yml byte-for-byte unchanged",
		Actual:   string(currentConfig),
		Expected: string(originalConfig),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init re-runs on a partial config containing a user-edit sentinel",
		Should:   "preserve the sentinel comment in the config file",
		Actual:   strings.Contains(string(currentConfig), "# user-edit-marker-do-not-overwrite"),
		Expected: true,
	})
}
