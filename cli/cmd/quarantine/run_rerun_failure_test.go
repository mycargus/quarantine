package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// fakeRerunFailureAPI creates a test server that tracks issue creation attempts.
// It responds to branch check and quarantine.json reads, and records any POST to /issues.
func fakeRerunFailureAPI(t *testing.T, issueCreated *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			// Return empty quarantine state (file not found).
			w.WriteHeader(http.StatusNotFound)

		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		case strings.Contains(r.URL.Path, "/comments"):
			// PR comment endpoint — handle without triggering issueCreated.
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode([]interface{}{})
			} else {
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"id":1,"body":"comment"}`)
			}

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			if issueCreated != nil {
				*issueCreated = true
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"number":1,"html_url":"https://github.com/owner/repo/issues/1"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 83: Rerun command fails to execute ---

func TestRunRerunCommandFails(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: one test fails.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/checkout.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">expected 10 but got 0</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)

	// Use a nonexistent binary as the rerun command so runner.Run returns an error.
	configPath := writeTempConfig(t, `
version: 1
framework: jest
rerun_command: /nonexistent-binary-that-cannot-be-launched
retries: 1
`)

	issueCreated := false
	server := fakeRerunFailureAPI(t, &issueCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a test fails and the rerun command cannot be executed (nonexistent binary)",
		Should:   "exit with code 1 (genuine failure)",
		Actual:   exitCode,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the rerun command fails to launch",
		Should:   "emit WARNING: Rerun failed for the test name",
		Actual:   strings.Contains(output, `[quarantine] WARNING: Rerun failed for "should apply discount"`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the rerun command fails to launch",
		Should:   "mention rerun_command in the warning",
		Actual:   strings.Contains(output, "rerun_command"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the rerun command fails to launch",
		Should:   "not create a GitHub Issue (genuine failure, not flaky)",
		Actual:   issueCreated,
		Expected: false,
	})

	// Verify the test is classified as a genuine failure in results.json.
	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	summary, _ := results["summary"].(map[string]interface{})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "test fails and rerun command cannot launch",
		Should:   "report failed: 1 (genuine failure)",
		Actual:   summary["failed"].(float64),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "test fails and rerun command cannot launch",
		Should:   "report flaky_detected: 0 (not flaky)",
		Actual:   summary["flaky_detected"].(float64),
		Expected: 0,
	})
}
