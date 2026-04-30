package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 180: doctor rejects partial config (missing github.owner / github.repo) ---
//
// Per ADR-037, when `.quarantine/config.yml` exists but `github.owner` or
// `github.repo` is missing or empty, `quarantine doctor` MUST fail fast — exit
// 2 with `Error [config]: ...` BEFORE making any GitHub API call. This catches
// the partial config left by `quarantine init` phase 1 and forces the developer
// to complete it. The error and remediation hint match the canonical messages
// shared with `quarantine run` (Scenario 177).

func TestDoctorFailsFastWhenGitHubOwnerOrRepoMissing(t *testing.T) {
	dir := t.TempDir()
	// Origin is irrelevant for this scenario — owner/repo come from config only.
	setupFakeGitRepo(t, dir, "https://github.com/anything/anything.git")

	// Pre-create .quarantine/config.yml with explicitly empty github fields
	// (mirrors what `quarantine init` phase 1 leaves on disk before the
	// developer adds owner/repo).
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := `version: 1
github:
  owner: ""
  repo: ""
`
	if err := os.WriteFile(filepath.Join(suiteDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	// Tripwire HTTP server: fails the test if any GitHub API call is made.
	var apiCallCount int32
	tripwireAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(tripwireAPI.Close)

	output, err := executeDoctorCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": tripwireAPI.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "config exists with empty github.owner and github.repo",
		Should:   "exit with code 2",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has missing github.owner / github.repo",
		Should:   "print the canonical missing-fields error message",
		Actual:   strings.Contains(output, formatRunMissingGitHubFieldsError()),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "config validation rejects doctor before any reachability call",
		Should:   "make zero GitHub API calls",
		Actual:   atomic.LoadInt32(&apiCallCount),
		Expected: 0,
	})
}
