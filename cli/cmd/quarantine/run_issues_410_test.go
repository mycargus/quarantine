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

// --- Scenario 61: Issues disabled on repository (410 Gone) ---

// fakeIssues410GitHubAPI creates a test server where POST /repos/.../issues
// always returns 410 Gone. It tracks how many issue creation attempts were made
// and whether a PR comment was posted.
func fakeIssues410GitHubAPI(
	t *testing.T,
	prNumber int,
	issueAttemptCount *int32,
	prCommentPostedPtr *int32,
	prCommentBody *string,
) *httptest.Server {
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

		// Batch closed-issue search (unquarantine check).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Dedup search — no existing open issue for either test.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// List PR comments (none exist yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Create issue — POST /repos/.../issues (not a PR comment path) — returns 410 Gone.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(issueAttemptCount, 1)
			w.WriteHeader(http.StatusGone)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Issues are disabled for this repo",
			})

		// Post PR comment — POST /repos/.../issues/{pr}/comments.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentPostedPtr, 1)
			body := make(map[string]interface{})
			_ = json.NewDecoder(r.Body).Decode(&body)
			if s, ok := body["body"].(string); ok && prCommentBody != nil {
				*prCommentBody = s
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       999,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d#issuecomment-999", prNumber),
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunIssues410SkipsAllIssueCreationAndContinues(t *testing.T) {
	dir := t.TempDir()

	// Two failing tests — both become flaky on retry.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="2" failures="2" errors="0" time="1.0">
  <testsuite name="src/auth.test.js" tests="1" failures="1" time="0.5">
    <testcase classname="AuthService" name="should refresh token"
              file="src/auth.test.js" time="0.5">
      <failure message="Token expired" type="Error">Token expired</failure>
    </testcase>
  </testsuite>
  <testsuite name="src/cache.test.js" tests="1" failures="1" time="0.5">
    <testcase classname="CacheService" name="should evict stale entries"
              file="src/cache.test.js" time="0.5">
      <failure message="Cache miss" type="Error">Cache miss</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script-410")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 77
	var issueAttemptCount int32
	var prCommentPosted int32
	var prCommentBody string
	server := fakeIssues410GitHubAPI(t, prNumber, &issueAttemptCount, &prCommentPosted, &prCommentBody)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")

	// Capture combined output so we can check warning messages.
	for k, v := range map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	} {
		t.Setenv(k, v)
	}

	var output strings.Builder
	rootCmd := newRootCmd()
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{
		"run",
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--pr", fmt.Sprintf("%d", prNumber),
		"--", initialScript,
	})
	runErr := rootCmd.Execute()

	exitCode := 0
	if runErr != nil {
		if code, ok := runErr.(exitCodeError); ok {
			exitCode = int(code)
		} else {
			exitCode = 2
		}
	}

	combinedOutput := output.String()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two flaky tests detected and GitHub Issues are disabled (410 Gone)",
		Should:   "exit with code 0 (issues-disabled is not a genuine test failure)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub Issues are disabled (410 Gone) on POST /repos/.../issues",
		Should:   "log warning that GitHub Issues are disabled",
		Actual:   strings.Contains(combinedOutput, "[quarantine] WARNING: GitHub Issues are disabled on this repository. Skipping issue creation for all flaky tests in this run."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "GitHub Issues disabled — 410 on first issue creation attempt",
		Should:   "attempt issue creation exactly once (then abort for all subsequent tests)",
		Actual:   atomic.LoadInt32(&issueAttemptCount),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "GitHub Issues disabled, but PR comment is unrelated to Issues API",
		Should:   "still post PR comment",
		Actual:   atomic.LoadInt32(&prCommentPosted),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted after 410 on issue creation",
		Should:   "include flaky test name 'should refresh token' in the PR comment",
		Actual:   strings.Contains(prCommentBody, "should refresh token"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted after 410 on issue creation",
		Should:   "include flaky test name 'should evict stale entries' in the PR comment",
		Actual:   strings.Contains(prCommentBody, "should evict stale entries"),
		Expected: true,
	})

	// Verify quarantine state was written (tests added to quarantine.json without issue_number).
	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two flaky tests detected with Issues disabled",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		if err := json.Unmarshal(resultsData, &results); err == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "two flaky tests detected with Issues disabled",
				Should:   "report flaky_detected: 2 in results.json",
				Actual:   summary["flaky_detected"].(float64),
				Expected: 2,
			})
		}
	}
}
