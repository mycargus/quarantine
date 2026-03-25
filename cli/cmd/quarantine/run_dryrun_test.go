package main

import (
	"bytes"
	"encoding/base64"
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

	"github.com/mycargus/quarantine/internal/quarantine"
)

// fakeM4GitHubAPIWithPUTTracking creates a test server like fakeM4GitHubAPI
// but additionally tracks whether any PUT to /contents/quarantine.json occurred.
func fakeM4GitHubAPIWithPUTTracking(t *testing.T, qs *quarantine.State, closedIssueNumbers []int, putCalled *int32) *httptest.Server {
	t.Helper()

	var stateContent []byte
	if qs != nil {
		var err error
		stateContent, err = qs.Marshal()
		if err != nil {
			t.Fatalf("marshal quarantine state: %v", err)
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			items := make([]map[string]interface{}, len(closedIssueNumbers))
			for i, n := range closedIssueNumbers {
				items[i] = map[string]interface{}{"number": n}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": len(closedIssueNumbers),
				"items":       items,
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			atomic.AddInt32(putCalled, 1)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// executeRunCmdCaptureBoth runs the run command, returning both the combined
// output buffer and the exit error (so callers can inspect stderr for dry-run messages).
func executeRunCmdCaptureBoth(t *testing.T, args []string, env map[string]string) (output string, err error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"run"}, args...))
	err = rootCmd.Execute()
	return buf.String(), err
}

// --- Scenario 45: --dry-run flag ---

func TestRunDryRunDoesNotWriteQuarantineState(t *testing.T) {
	dir := t.TempDir()

	// Start with an empty quarantine state so a new flaky detection can occur.
	qs := quarantine.NewEmptyState()

	// JUnit XML: one test fails initially → flaky when it passes on retry.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="__tests__/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="__tests__/payment.test.js" time="0.5">
      <failure message="Timeout exceeded">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	// Rerun script: always exits 0 (test passes on retry → flaky).
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
`, rerunScriptPath))

	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, qs, []int{}, &putCalled)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmdCaptureBoth(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--dry-run",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with a flaky test detected",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with a flaky test detected",
		Should:   "not write quarantine state (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled) == 0,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with a flaky test detected",
		Should:   "print DRY RUN message to stderr",
		Actual:   strings.Contains(output, "DRY RUN"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with a flaky test detected",
		Should:   "print the name of the would-be quarantined test",
		Actual:   strings.Contains(output, "should handle charge timeout"),
		Expected: true,
	})

	// results.json must still be written.
	_, statErr := os.Stat(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with a flaky test detected",
		Should:   "still write results.json",
		Actual:   statErr == nil,
		Expected: true,
	})
}

func TestRunDryRunWithNoFlakyTestsExitsZeroWithoutDryRunMessage(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: all tests pass.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="__tests__/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should process payment"
              file="__tests__/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmdCaptureBoth(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--dry-run",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with all tests passing (no flaky)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with no flaky tests",
		Should:   "not print DRY RUN message (nothing would have happened)",
		Actual:   strings.Contains(output, "DRY RUN"),
		Expected: false,
	})
}

func TestRunDryRunWouldQuarantineListsEachFlakyTest(t *testing.T) {
	dir := t.TempDir()

	// Two flaky tests.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2" failures="2">
  <testsuite name="__tests__/search.test.js" tests="1" failures="1">
    <testcase classname="SearchService" name="should fuzzy match"
              file="__tests__/search.test.js" time="0.5">
      <failure message="fuzzy match failed">fuzzy match failed</failure>
    </testcase>
  </testsuite>
  <testsuite name="__tests__/api.test.js" tests="1" failures="1">
    <testcase classname="ApiService" name="should handle rate limit"
              file="__tests__/api.test.js" time="0.5">
      <failure message="rate limit error">rate limit error</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmdCaptureBoth(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--dry-run",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with two flaky tests",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with two flaky tests",
		Should:   "list first would-be quarantined test name",
		Actual:   strings.Contains(output, "should fuzzy match"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with two flaky tests",
		Should:   "list second would-be quarantined test name",
		Actual:   strings.Contains(output, "should handle rate limit"),
		Expected: true,
	})
}
