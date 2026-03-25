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

// --- Scenario 20: CI run detects a new flaky test (issue creation + PR comment) ---

// fakeM5GitHubAPI creates a test server that handles all M5 GitHub API endpoints:
// M4 endpoints (branch, contents, search-closed) + new issue creation + PR comments.
// It captures whether POST /repos/.../issues and POST .../issues/{pr}/comments were called.
func fakeM5GitHubAPI(
	t *testing.T,
	prNumber int,
	issueCreatedPtr *int32,
	prCommentCreatedPtr *int32,
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

		// Dedup search — open issue check for the flaky test (no existing issue).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// List PR comments (none exist yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Create issue — POST /repos/.../issues (not a PR comment path).
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(issueCreatedPtr, 1)
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   101,
				"html_url": "https://github.com/test-owner/test-repo/issues/101",
				"title":    req["title"],
			})

		// Post PR comment — POST /repos/.../issues/{pr}/comments.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       999,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d#issuecomment-999", prNumber),
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunDetectsFlakyTestCreatesIssueAndPRComment(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML: PaymentService > should handle charge timeout fails initially.
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

	prNumber := 99
	var issueCreated int32
	var prCommentCreated int32
	server := fakeM5GitHubAPI(t, prNumber, &issueCreated, &prCommentCreated)
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
		Given:    "a newly detected flaky test",
		Should:   "exit with code 0 (flaky is not a genuine failure)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a newly detected flaky test with no existing GitHub issue",
		Should:   "create a GitHub issue via POST /repos/.../issues",
		Actual:   atomic.LoadInt32(&issueCreated) > 0,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a newly detected flaky test with --pr flag set",
		Should:   "post a PR comment via POST /repos/.../issues/{pr}/comments",
		Actual:   atomic.LoadInt32(&prCommentCreated) > 0,
		Expected: true,
	})
}

// --- Scenario 24: Multiple flaky tests detected — issue + PR comment counts ---

// fakeMultiFlakyGitHubAPI creates a test server for the multiple-flaky-tests scenario.
// It tracks issue creation count and PR comment count, and captures the PR comment body.
func fakeMultiFlakyGitHubAPI(
	t *testing.T,
	prNumber int,
	issueCreateCount *int32,
	prCommentCount *int32,
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

		// Create issue — POST /repos/.../issues (not a PR comment path).
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			n := atomic.AddInt32(issueCreateCount, 1)
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   100 + n,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d", 100+n),
				"title":    req["title"],
			})

		// Post PR comment.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentCount, 1)
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			if s, ok := req["body"].(string); ok {
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

func TestRunMultipleFlakyTestsCreatesIssuesAndSinglePRComment(t *testing.T) {
	dir := t.TempDir()

	// Two failing tests in the initial run.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="2" failures="2" errors="0" time="1.0">
  <testsuite name="__tests__/search.test.js" tests="1" failures="1" time="0.5">
    <testcase classname="SearchService" name="should fuzzy match"
              file="__tests__/search.test.js" time="0.5">
      <failure message="fuzzy match failed" type="Error">fuzzy match failed</failure>
    </testcase>
  </testsuite>
  <testsuite name="__tests__/api.test.js" tests="1" failures="1" time="0.5">
    <testcase classname="ApiService" name="should handle rate limit"
              file="__tests__/api.test.js" time="0.5">
      <failure message="rate limit error" type="Error">rate limit error</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)

	// Rerun script: always exits 0 (both tests pass on retry).
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 88
	var issueCreateCount int32
	var prCommentCount int32
	var prCommentBody string
	server := fakeMultiFlakyGitHubAPI(t, prNumber, &issueCreateCount, &prCommentCount, &prCommentBody)
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
		Given:    "two newly detected flaky tests",
		Should:   "exit with code 0 (flaky only, no genuine failures)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "two newly detected flaky tests with no existing GitHub issues",
		Should:   "create exactly two GitHub issues",
		Actual:   atomic.LoadInt32(&issueCreateCount),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "two newly detected flaky tests with --pr flag set",
		Should:   "post exactly one PR comment summarizing both tests",
		Actual:   atomic.LoadInt32(&prCommentCount),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for two flaky tests",
		Should:   "include 'should fuzzy match' in the comment body",
		Actual:   strings.Contains(prCommentBody, "should fuzzy match"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for two flaky tests",
		Should:   "include 'should handle rate limit' in the comment body",
		Actual:   strings.Contains(prCommentBody, "should handle rate limit"),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two flaky tests detected",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		if err := json.Unmarshal(resultsData, &results); err == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "two tests are flaky",
				Should:   "report flaky_detected: 2",
				Actual:   summary["flaky_detected"].(float64),
				Expected: 2,
			})
		}
	}
}
