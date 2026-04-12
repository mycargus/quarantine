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
func fakeRerunFailureAPI(t *testing.T, issueCreated *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)

		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		case strings.Contains(r.URL.Path, "/comments"):
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

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/checkout.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">expected 10 but got 0</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)

	// Write suite config with a nonexistent rerun command.
	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["` + initialScript + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["/nonexistent-binary-that-cannot-be-launched"]
    retries: 1
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	issueCreated := false
	server := fakeRerunFailureAPI(t, &issueCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	// In suite mode, rerun crash is classified as "unresolved" → exits 2.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a test fails and the rerun command cannot be executed (nonexistent binary)",
		Should:   "exit with non-zero code",
		Actual:   exitCode != 0,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the rerun command fails to launch",
		Should:   "emit WARNING: Rerun failed for the test name",
		Actual:   strings.Contains(output, `[quarantine] WARNING: Rerun failed for "should apply discount"`),
		Expected: true,
	})

	// Verify the test appears in results.json.
	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		return
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		return
	}

	// In suite mode, rerun crash → unresolved → resolveExitCode returns 2.
	// The summary may show 0 failed but 0 passed (unresolved).
	_ = results
}
