package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 125: two parallel suite runs operate on separate state files ---
// Path format is already unit-tested in TestSuiteStatePath (run_suite_state_test.go).
// This integration test verifies the isolation property end-to-end.

// fakeTwoSuiteAPI creates a server that tracks separate PUT requests to
// backend and frontend state file paths. It handles branch checks, 404 for
// state reads, and records CAS write paths.
func fakeTwoSuiteAPI(t *testing.T, backendPutCalled, frontendPutCalled *int32) *httptest.Server {
	t.Helper()
	backendStatePath := suiteStatePath("backend")
	frontendStatePath := suiteStatePath("frontend")

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// State file GETs: return 404 (first run, no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write for backend state.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"+backendStatePath):
			atomic.AddInt32(backendPutCalled, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha-backend"}}`)

		// CAS write for frontend state.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"+frontendStatePath):
			atomic.AddInt32(frontendPutCalled, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha-frontend"}}`)

		// Closed-issue search.
		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Issue creation.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   201,
				"html_url": "https://github.com/test-owner/test-repo/issues/201",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// buildFlakySuiteConfig writes a config.yml with two suites (backend, frontend)
// and returns the config path. Each suite has its own flaky command and xml path.
func buildFlakySuiteConfig(
	t *testing.T,
	dir string,
	backendBin, backendXML string,
	frontendBin, frontendXML string,
) string {
	t.Helper()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["true"]
    retries: 1
  - name: frontend
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["true"]
    retries: 1
`, backendBin, backendXML, frontendBin, frontendXML)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
	return configPath
}

// buildFlakySuiteCommand writes a script that fails on first run (producing
// failing JUnit XML), then passes on retry.
func buildFlakySuiteCommand(t *testing.T, dir, suiteName, xmlPath string) string {
	t.Helper()

	counterFile := filepath.Join(dir, suiteName+"-run-count.txt")
	failureXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="%s" tests="1" skipped="0" failures="1" errors="0" time="0.1">
  <testcase classname="spec.%s_spec"
            name="%s validates something"
            file="./spec/%s_spec.rb"
            time="0.1">
    <failure type="Error" message="expected true but got false"/>
  </testcase>
</testsuite>`, suiteName, suiteName, suiteName, suiteName)

	fakeBin := filepath.Join(dir, "fake-"+suiteName)
	script := fmt.Sprintf(`#!/bin/sh
if [ ! -f %s ]; then
  touch %s
  cat > %s << 'XMLEOF'
%s
XMLEOF
  exit 1
fi
exit 0
`, counterFile, counterFile, xmlPath, failureXML)

	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write %s script: %v", suiteName, err)
	}
	return fakeBin
}

// TestRunParallelSuitesUseSeparateStatePaths verifies that two sequential
// invocations of `quarantine run` with different suite names each write to
// their own separate state file path. This simulates concurrent CI jobs that
// would use separate files and therefore never create a CAS conflict.
func TestRunParallelSuitesUseSeparateStatePaths(t *testing.T) {
	dir := t.TempDir()

	backendXML := filepath.Join(dir, "backend.xml")
	frontendXML := filepath.Join(dir, "frontend.xml")

	backendBin := buildFlakySuiteCommand(t, dir, "backend", backendXML)
	frontendBin := buildFlakySuiteCommand(t, dir, "frontend", frontendXML)

	configPath := buildFlakySuiteConfig(t, dir, backendBin, backendXML, frontendBin, frontendXML)

	var backendPutCalled, frontendPutCalled int32
	server := fakeTwoSuiteAPI(t, &backendPutCalled, &frontendPutCalled)
	defer server.Close()

	backendResultsPath := filepath.Join(dir, "backend-results.json")
	_, backendErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", backendResultsPath,
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	frontendResultsPath := filepath.Join(dir, "frontend-results.json")
	_, frontendErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", frontendResultsPath,
		"frontend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with a flaky test",
		Should:   "exit without error (code 0)",
		Actual:   backendErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a frontend suite with a flaky test",
		Should:   "exit without error (code 0)",
		Actual:   frontendErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "backend suite detected a flaky test",
		Should:   "write to '.quarantine/backend/state.json' (backend PUT called exactly once)",
		Actual:   atomic.LoadInt32(&backendPutCalled),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "frontend suite detected a flaky test",
		Should:   "write to '.quarantine/frontend/state.json' (frontend PUT called exactly once)",
		Actual:   atomic.LoadInt32(&frontendPutCalled),
		Expected: 1,
	})
}
