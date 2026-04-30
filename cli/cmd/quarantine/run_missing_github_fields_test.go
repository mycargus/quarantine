package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/config"
	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 177: run rejects partial config (missing github.owner / github.repo) ---
//
// Per ADR-037, when `.quarantine/config.yml` exists but `github.owner` or
// `github.repo` is missing or empty, `quarantine run` MUST fail fast — exit 2
// with `Error [config]: ...` BEFORE executing the test command or making any
// GitHub API call. This catches the partial config left by `quarantine init`
// phase 1 (Scenario 174/175) and forces the developer to complete it.

func TestRunFailsFastWhenGitHubOwnerOrRepoMissing(t *testing.T) {
	dir := t.TempDir()
	// Origin is irrelevant for this scenario — owner/repo come from config only.
	setupFakeGitRepo(t, dir, "https://github.com/anything/anything.git")

	// Tripwire test command: writes a sentinel file when executed. The test
	// asserts the sentinel does NOT exist after the run, proving the suite
	// command was never executed.
	sentinelPath := filepath.Join(dir, "sentinel.txt")
	scriptPath := filepath.Join(dir, "tripwire.sh")
	script := "#!/bin/sh\ntouch " + sentinelPath + "\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write tripwire script: %v", err)
	}

	// Pre-create .quarantine/config.yml with explicitly empty github fields
	// (mirrors what `quarantine init` phase 1 leaves on disk).
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := `version: 1
github:
  owner: ""
  repo: ""
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
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": tripwireAPI.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "config exists with empty github.owner and github.repo",
		Should:   "exit with code 2",
		Actual:   extractExitCode(t, err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has missing github.owner / github.repo",
		Should:   "print the typed config error explaining required fields",
		Actual:   strings.Contains(output, "Error [config]: github.owner and github.repo are required in .quarantine/config.yml."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has missing github.owner / github.repo",
		Should:   "print the remediation hint pointing at quarantine init",
		Actual:   strings.Contains(output, "Run 'quarantine init' or edit the config to add them."),
		Expected: true,
	})

	_, sentinelStatErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config validation rejects the run before test execution",
		Should:   "not execute the suite command (no sentinel file written)",
		Actual:   os.IsNotExist(sentinelStatErr),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "config validation rejects the run before any API call",
		Should:   "make zero GitHub API calls",
		Actual:   atomic.LoadInt32(&apiCallCount),
		Expected: 0,
	})
}

// --- Pure unit tests for validateGitHubFields ---

func TestValidateGitHubFieldsBothEmpty(t *testing.T) {
	cfg := &config.Config{}
	err := validateGitHubFields(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with both github.owner and github.repo empty",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateGitHubFieldsOwnerEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitHub.Repo = "my-project"
	err := validateGitHubFields(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with github.owner empty and github.repo set",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateGitHubFieldsRepoEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitHub.Owner = "my-org"
	err := validateGitHubFields(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with github.owner set and github.repo empty",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateGitHubFieldsBothPresent(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitHub.Owner = "my-org"
	cfg.GitHub.Repo = "my-project"
	err := validateGitHubFields(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with both github.owner and github.repo set",
		Should:   "return nil",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestValidateGitHubFieldsErrorMessageMatchesScenario(t *testing.T) {
	cfg := &config.Config{}
	err := validateGitHubFields(cfg)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with empty github fields",
		Should:   "include the canonical error preamble",
		Actual:   strings.Contains(err.Error(), "Error [config]: github.owner and github.repo are required in .quarantine/config.yml."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with empty github fields",
		Should:   "include the remediation hint pointing at quarantine init",
		Actual:   strings.Contains(err.Error(), "Run 'quarantine init' or edit the config to add them."),
		Expected: true,
	})
}
