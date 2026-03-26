package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// fakePRFlagsGitHubAPI creates a test server for PR flag scenarios.
// It tracks PR comment creation count (POST) and update count (PATCH).
func fakePRFlagsGitHubAPI(
	t *testing.T,
	prNumber int,
	existingCommentBody string, // non-empty → return existing comment with this body in list
	prCommentCreatedPtr *int32,
	prCommentUpdatedPtr *int32,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — empty (no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusOK)

		// Batch closed-issue search (unquarantine check).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Dedup search — no existing open issue.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Create issue — POST /repos/.../issues (not a PR comment path).
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   201,
				"html_url": "https://github.com/test-owner/test-repo/issues/201",
				"title":    req["title"],
			})

		// List PR comments.
		case r.Method == "GET" && prNumber > 0 &&
			strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			if existingCommentBody != "" {
				_ = json.NewEncoder(w).Encode([]interface{}{
					map[string]interface{}{
						"id":   int64(555),
						"body": existingCommentBody,
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode([]interface{}{})
			}

		// Post PR comment — POST /repos/.../issues/{pr}/comments.
		case r.Method == "POST" && prNumber > 0 &&
			strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 999,
			})

		// Update PR comment — PATCH /repos/.../issues/comments/{id}.
		case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/issues/comments/"):
			atomic.AddInt32(prCommentUpdatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 555,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 47 case 3: non-PR build → no PR comment posted ---

func TestRunNoPRFlagNoEventPathSkipsPRComment(t *testing.T) {
	dir := t.TempDir()

	// A flaky test so the run would normally try to post a PR comment.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="Timeout exceeded" type="Error">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	// Use prNumber=0 so the fake server never receives PR comment calls.
	// The server tracks POST/PATCH counts even if they would be on unknown paths.
	var prCommentCreated int32
	var prCommentUpdated int32
	server := fakePRFlagsGitHubAPI(t, 0, "", &prCommentCreated, &prCommentUpdated)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")

	// No --pr flag and GITHUB_EVENT_PATH intentionally not set (non-PR build).
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_EVENT_PATH":              "", // explicitly absent
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "no --pr flag and no GITHUB_EVENT_PATH on a non-PR build",
		Should:   "exit with code 0 (flaky test, not a genuine failure)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "no PR number available (non-PR build)",
		Should:   "not create a PR comment (POST count stays 0)",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "no PR number available (non-PR build)",
		Should:   "not update any PR comment (PATCH count stays 0)",
		Actual:   atomic.LoadInt32(&prCommentUpdated),
		Expected: 0,
	})
}

// --- Scenario 48: PR comment suppressed via config ---

func TestRunPRCommentSuppressedByConfig(t *testing.T) {
	dir := t.TempDir()

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="Timeout exceeded" type="Error">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	// Config has github_pr_comment: false.
	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
notifications:
  github_pr_comment: false
rerun_command: %s
`, rerunScript))

	prNumber := 77
	var prCommentCreated int32
	var prCommentUpdated int32
	server := fakePRFlagsGitHubAPI(t, prNumber, "", &prCommentCreated, &prCommentUpdated)
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
		Given:    "config has github_pr_comment: false and a flaky test is detected",
		Should:   "exit with code 0 (quarantine succeeds)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "config has github_pr_comment: false",
		Should:   "not post a PR comment even though --pr is set",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "config has github_pr_comment: false",
		Should:   "not update any existing PR comment",
		Actual:   atomic.LoadInt32(&prCommentUpdated),
		Expected: 0,
	})

	// Verify results.json is still written (other behavior continues normally).
	_, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has github_pr_comment: false",
		Should:   "still write results.json (only PR comment is suppressed)",
		Actual:   readErr == nil,
		Expected: true,
	})
}

// --- Scenario 49: PR comment updated on second run ---

func TestRunSecondRunUpdatesPRCommentInsteadOfCreating(t *testing.T) {
	dir := t.TempDir()

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="Timeout exceeded" type="Error">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 42
	// Simulate an existing quarantine-bot comment from a previous run.
	existingBody := "<!-- quarantine-bot -->\n## Quarantine Summary\n\nPrevious run content."

	var prCommentCreated int32
	var prCommentUpdated int32
	server := fakePRFlagsGitHubAPI(t, prNumber, existingBody, &prCommentCreated, &prCommentUpdated)
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
		Given:    "second CI run on same PR with an existing quarantine-bot comment",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "existing quarantine-bot comment found in PR comment list",
		Should:   "update the existing comment via PATCH (not create a new one)",
		Actual:   atomic.LoadInt32(&prCommentUpdated),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "existing quarantine-bot comment found in PR comment list",
		Should:   "not create a duplicate comment via POST",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 0,
	})
}
