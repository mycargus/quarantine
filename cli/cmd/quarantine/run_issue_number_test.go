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

// --- Scenario 107: results.json includes issue_number for a newly created GitHub Issue ---

// fakeNewIssueGitHubAPI creates a test server that handles all GitHub API endpoints
// and returns a newly created issue with the given number.
func fakeNewIssueGitHubAPI(t *testing.T, issueNumber int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — empty (404 = no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusOK)

		// All search requests — return empty (no existing issues for new-issue test).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Create issue — POST /repos/.../issues (not a PR comment path).
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   issueNumber,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d", issueNumber),
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunResultsJSONIncludesIssueNumberForNewlyCreatedIssue(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: one test fails initially, passes on retry → flaky.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="2" failures="1" errors="0" time="1.0">
  <testsuite name="src/billing.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="BillingService" name="should charge card"
              file="src/billing.test.js" time="0.5">
      <failure message="Connection reset" type="Error">Connection reset</failure>
    </testcase>
  </testsuite>
  <testsuite name="src/billing.test.js" tests="1" failures="0" time="0.5">
    <testcase classname="BillingService" name="should generate receipt"
              file="src/billing.test.js" time="0.5"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script-107")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	issueNumber := 42
	server := fakeNewIssueGitHubAPI(t, issueNumber)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test detected and a new GitHub issue created",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON in results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	if len(tests) == 0 {
		t.Fatal("expected at least 1 test entry in results.json")
	}

	// Find flaky test and passing test entries.
	var flakyEntry map[string]interface{}
	var passingEntry map[string]interface{}
	for _, raw := range tests {
		entry, _ := raw.(map[string]interface{})
		status, _ := entry["status"].(string)
		switch status {
		case "flaky":
			flakyEntry = entry
		case "passed":
			passingEntry = entry
		}
	}

	if flakyEntry == nil {
		t.Fatal("expected a flaky test entry in results.json")
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test detected and GitHub issue #42 created",
		Should:   "have a non-null issue_number in results.json",
		Actual:   flakyEntry["issue_number"] != nil,
		Expected: true,
	})

	if flakyEntry["issue_number"] != nil {
		riteway.Assert(t, riteway.Case[float64]{
			Given:    "a flaky test detected and GitHub issue #42 created",
			Should:   "have issue_number: 42 in results.json",
			Actual:   flakyEntry["issue_number"].(float64),
			Expected: float64(issueNumber),
		})
	}

	if passingEntry != nil {
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "a passing test with no corresponding GitHub issue",
			Should:   "have issue_number: null in results.json",
			Actual:   passingEntry["issue_number"] == nil,
			Expected: true,
		})
	}
}

// --- Scenario 108: results.json includes issue_number from a deduplicated existing issue ---

// fakeDedupExistingIssueGitHubAPI creates a test server where dedup search returns
// an existing open issue with the given number (no new issue created).
func fakeDedupExistingIssueGitHubAPI(t *testing.T, existingIssueNumber int, existingIssueURL string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — empty (404 = no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusOK)

		// All search requests — dispatch based on decoded query.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			q := r.URL.Query().Get("q")
			if strings.Contains(q, "is:open") {
				// Dedup search — returns existing open issue.
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total_count": 1,
					"items": []interface{}{
						map[string]interface{}{
							"number":   existingIssueNumber,
							"html_url": existingIssueURL,
						},
					},
				})
			} else {
				// Closed-issue search or other — return empty.
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total_count": 0,
					"items":       []interface{}{},
				})
			}

		// Issue creation — should NOT be called when dedup finds an existing issue.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			t.Fatal("unexpected issue creation call — dedup should have found existing issue")
			w.WriteHeader(http.StatusUnprocessableEntity)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunResultsJSONIncludesIssueNumberFromDedupExistingIssue(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: one test fails initially, passes on retry → flaky.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/inventory.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="InventoryService" name="should restock items"
              file="src/inventory.test.js" time="0.5">
      <failure message="lock timeout" type="Error">lock timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script-108")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	existingIssueNumber := 173
	existingIssueURL := "https://github.com/test-owner/test-repo/issues/173"
	server := fakeDedupExistingIssueGitHubAPI(t, existingIssueNumber, existingIssueURL)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test detected and dedup search finds existing issue #173",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON in results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	if len(tests) == 0 {
		t.Fatal("expected at least 1 test entry in results.json")
	}

	testEntry, _ := tests[0].(map[string]interface{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a flaky test detected",
		Should:   "have status: flaky",
		Actual:   testEntry["status"].(string),
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test and dedup finds existing issue #173",
		Should:   "have a non-null issue_number in results.json",
		Actual:   testEntry["issue_number"] != nil,
		Expected: true,
	})

	if testEntry["issue_number"] != nil {
		riteway.Assert(t, riteway.Case[float64]{
			Given:    "a flaky test and dedup finds existing issue #173",
			Should:   "have issue_number: 173 in results.json (from deduplicated issue, not null)",
			Actual:   testEntry["issue_number"].(float64),
			Expected: float64(existingIssueNumber),
		})
	}
}

// --- Scenario 109: results.json has null issue_number when issue creation fails ---

// fakeIssue503GitHubAPI creates a test server where POST /repos/.../issues
// always returns 503 Service Unavailable. It captures the output to verify the warning.
func fakeIssue503GitHubAPI(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — empty (404 = no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusOK)

		// All search requests — return empty (no existing issues).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Create issue — returns 503 Service Unavailable.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Service Unavailable",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunResultsJSONHasNullIssueNumberWhenIssueCreationFails(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: one test fails initially, passes on retry → flaky.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/notify.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="NotifyService" name="should send email"
              file="src/notify.test.js" time="0.5">
      <failure message="SMTP timeout" type="Error">SMTP timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script-109")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	server := fakeIssue503GitHubAPI(t)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")

	combinedOutput, runErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := 0
	if runErr != nil {
		if code, ok := runErr.(exitCodeError); ok {
			exitCode = int(code)
		} else {
			exitCode = 2
		}
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test detected and issue creation fails with 503",
		Should:   "exit with code 0 (issue creation failure is a warning, not fatal)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue creation fails with 503",
		Should:   "log a warning about the failure",
		Actual:   strings.Contains(combinedOutput, "[quarantine] WARNING"),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON in results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	if len(tests) == 0 {
		t.Fatal("expected at least 1 test entry in results.json")
	}

	testEntry, _ := tests[0].(map[string]interface{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a flaky test detected with 503 on issue creation",
		Should:   "have status: flaky",
		Actual:   testEntry["status"].(string),
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue creation fails with 503 for the flaky test",
		Should:   "have issue_number: null in results.json",
		Actual:   testEntry["issue_number"] == nil,
		Expected: true,
	})
}
