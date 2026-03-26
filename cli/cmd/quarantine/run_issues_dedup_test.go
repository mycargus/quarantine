package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 27: Concurrent CI builds — second build skips issue creation via dedup ---

// fakeDedupFoundGitHubAPI creates a test server where the dedup search for open issues
// returns an existing issue (simulating the second concurrent build finding the issue
// the first build already created).
func fakeDedupFoundGitHubAPI(
	t *testing.T,
	prNumber int,
	issueCreatedPtr *int32,
	prCommentCreatedPtr *int32,
	existingIssueNumber int,
	existingIssueURL string,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — return empty (404 = no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusOK)

		// Batch closed-issue search (for unquarantine check).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Dedup search — returns existing open issue (simulates Build B finding Build A's issue).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items": []interface{}{
					map[string]interface{}{
						"number":   existingIssueNumber,
						"html_url": existingIssueURL,
					},
				},
			})

		// List PR comments (none exist yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Create issue — should NOT be called when dedup finds an existing issue.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(issueCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   999,
				"html_url": "https://github.com/test-owner/test-repo/issues/999",
			})

		// Post PR comment.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       888,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d#issuecomment-888", prNumber),
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunDedupSkipsIssueCreationWhenIssueAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: CacheService > should handle eviction fails initially.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/cache.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="CacheService" name="should handle eviction"
              file="src/cache.test.js" time="0.5">
      <failure message="eviction failed" type="Error">eviction failed</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-dedup-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 77
	existingIssueNumber := 42
	existingIssueURL := "https://github.com/test-owner/test-repo/issues/42"
	var issueCreated int32
	var prCommentCreated int32
	server := fakeDedupFoundGitHubAPI(
		t, prNumber,
		&issueCreated, &prCommentCreated,
		existingIssueNumber, existingIssueURL,
	)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--pr", fmt.Sprintf("%d", prNumber),
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test whose issue was already created by a concurrent build",
		Should:   "exit with code 0 (flaky is not a genuine failure)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "dedup search finds an existing open issue for the flaky test",
		Should:   "skip issue creation (CreateIssue never called)",
		Actual:   atomic.LoadInt32(&issueCreated),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test with an existing issue and --pr flag set",
		Should:   "still post a PR comment referencing the existing issue",
		Actual:   atomic.LoadInt32(&prCommentCreated) > 0,
		Expected: true,
	})
}
