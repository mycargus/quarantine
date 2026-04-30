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

// --- Scenario 178: run rejects missing GitHub token (no QUARANTINE_GITHUB_TOKEN, no GITHUB_TOKEN) ---
//
// Per ADR-037, when `.quarantine/config.yml` has valid `github.owner` and
// `github.repo` but neither `QUARANTINE_GITHUB_TOKEN` nor `GITHUB_TOKEN` is set
// in the environment, `quarantine run` MUST fail fast — exit 2 with an explicit
// error BEFORE executing the test command or making any GitHub API call. This
// surfaces a misconfiguration on non-GitHub-Actions CI (e.g. Jenkins) where the
// token is not provided automatically.

func TestRunFailsFastWhenGitHubTokenMissing(t *testing.T) {
	dir := t.TempDir()
	// Origin is irrelevant for this scenario — owner/repo come from config only.
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Tripwire test command: writes a sentinel file when executed. The test
	// asserts the sentinel does NOT exist after the run, proving the suite
	// command was never executed.
	sentinelPath := filepath.Join(dir, "sentinel.txt")
	scriptPath := filepath.Join(dir, "tripwire.sh")
	script := "#!/bin/sh\ntouch " + sentinelPath + "\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write tripwire script: %v", err)
	}

	// Pre-create .quarantine/config.yml with VALID github.owner and github.repo
	// so we exercise the no-token path, not the missing-fields path.
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := `version: 1
github:
  owner: my-org
  repo: my-project
test_suites:
  - name: backend
    command: ["` + scriptPath + `"]
    junitxml: "rspec.xml"
    rerun_command: ["false"]
    retries: 1
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

	output, err := executeRunCmd(t, []string{"backend"}, map[string]string{
		// Both token sources explicitly empty — overrides any inherited token.
		"QUARANTINE_GITHUB_TOKEN":        "",
		"GITHUB_TOKEN":                   "",
		"QUARANTINE_GITHUB_API_BASE_URL": tripwireAPI.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN is set",
		Should:   "exit with code 2",
		Actual:   extractExitCode(t, err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "neither GitHub token env var is set",
		Should:   "print the canonical missing-token error",
		Actual:   strings.Contains(output, "Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."),
		Expected: true,
	})

	_, sentinelStatErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "missing-token validation rejects the run before test execution",
		Should:   "not execute the suite command (no sentinel file written)",
		Actual:   os.IsNotExist(sentinelStatErr),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "missing-token validation rejects the run before any API call",
		Should:   "make zero GitHub API calls",
		Actual:   atomic.LoadInt32(&apiCallCount),
		Expected: 0,
	})
}

// --- Pure unit tests for tokenMissingError ---

func TestTokenMissingErrorWhenTokenEmpty(t *testing.T) {
	err := tokenMissingError("")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty token string",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestTokenMissingErrorWhenTokenPresent(t *testing.T) {
	err := tokenMissingError("ghp_something")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-empty token string",
		Should:   "return nil",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestTokenMissingErrorMessageMatchesScenario(t *testing.T) {
	err := tokenMissingError("")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an empty token string",
		Should:   "return the canonical missing-token error message verbatim",
		Actual:   err.Error(),
		Expected: "Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.",
	})
}

// --- Pure unit test for formatRunMissingTokenError ---

func TestFormatRunMissingTokenErrorReturnsCanonicalMessage(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "the run command needs to print a missing-token error",
		Should:   "return the canonical message verbatim",
		Actual:   formatRunMissingTokenError(),
		Expected: "Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.",
	})
}
