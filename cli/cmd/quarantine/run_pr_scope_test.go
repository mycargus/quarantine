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

// --- Scenario 72: Flaky test in new-to-PR file — no issue, no quarantine, warning PR comment ---

// fakePRScopeGitHubAPI returns a test server that tracks issue creation and CAS writes
// separately, so tests can assert that neither happened for new-to-PR tests.
func fakePRScopeGitHubAPI(
	t *testing.T,
	prNumber int,
	issueCreatedPtr *int32,
	casWrittenPtr *int32,
	prCommentBodyPtr *string,
	prCommentCreatedPtr *int32,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state — empty.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)

		// CAS write — track it (should NOT be called for new-to-PR tests).
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			atomic.AddInt32(casWrittenPtr, 1)
			w.WriteHeader(http.StatusOK)

		// Closed-issue search.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})

		// Open issue dedup search.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})

		// List PR comments.
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Create issue — should NOT be called for new-to-PR tests.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(issueCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   101,
				"html_url": "https://github.com/test-owner/test-repo/issues/101",
			})

		// Post PR comment.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			atomic.AddInt32(prCommentCreatedPtr, 1)
			body := map[string]interface{}{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if s, ok := body["body"].(string); ok {
				*prCommentBodyPtr = s
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 999})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 73: Flaky test in pre-existing file — issue created, CAS written, PR comment with issue URL ---

func TestRunPreExistingTestInPreExistingFileScopedToPR(t *testing.T) {
	// Override PR scope check: src/payment.test.js is NOT new to the PR, and
	// "should handle charge timeout" is NOT in added lines — both conditions miss,
	// so classifyPRScope returns "" (pre-existing). The empty map means no skip reasons.
	origCheck := checkPRScopeForTests
	checkPRScopeForTests = func(_ string, tests []prScopeInput) map[string]string {
		// Return empty map — no tests are PR-scoped, all should follow normal path.
		return make(map[string]string)
	}
	t.Cleanup(func() { checkPRScopeForTests = origCheck })

	dir := t.TempDir()

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="Timeout" type="Error">Timeout after 5000ms</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-scope-73")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 77
	var issueCreated int32
	var casWritten int32
	var prCommentCreated int32
	var prCommentBody string
	server := fakePRScopeGitHubAPI(t, prNumber, &issueCreated, &casWritten, &prCommentBody, &prCommentCreated)
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
		"GITHUB_BASE_REF":                "main",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test in a pre-existing file (file and test pre-existed before this PR)",
		Should:   "exit with code 0 (flaky is forgiven — passed on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a flaky test in a pre-existing file",
		Should:   "create a GitHub Issue for the flaky test",
		Actual:   atomic.LoadInt32(&issueCreated),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a flaky test in a pre-existing file",
		Should:   "write quarantine.json via CAS",
		Actual:   atomic.LoadInt32(&casWritten),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a flaky test in a pre-existing file",
		Should:   "post a PR comment",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 1,
	})

	// Close server before reading prCommentBody: httptest.Server.Close() waits
	// for all handler goroutines to complete, providing the synchronization the
	// Go race detector requires before the non-atomic string read below.
	server.Close()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for pre-existing flaky test",
		Should:   "include the GitHub Issue URL",
		Actual:   strings.Contains(prCommentBody, "https://github.com/test-owner/test-repo/issues/"),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test in a pre-existing file",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("unmarshal results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	var found bool
	for _, tRaw := range tests {
		tMap, _ := tRaw.(map[string]interface{})
		if tMap["name"] == "should handle charge timeout" {
			found = true
			riteway.Assert(t, riteway.Case[interface{}]{
				Given:    "flaky test in a pre-existing file",
				Should:   "NOT have issue_skipped_reason in results.json",
				Actual:   tMap["issue_skipped_reason"],
				Expected: nil,
			})
		}
	}
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "results.json test entries",
		Should:   "include the flaky test entry",
		Actual:   found,
		Expected: true,
	})
}

func TestRunFlakyTestInNewFileScopedToPR(t *testing.T) {
	// Override PR scope check: src/payment-refund.test.js is new to the PR.
	origCheck := checkPRScopeForTests
	checkPRScopeForTests = func(_ string, tests []prScopeInput) map[string]string {
		reasons := make(map[string]string)
		for _, inp := range tests {
			if inp.FilePath == "src/payment-refund.test.js" {
				reasons[inp.TestID] = "new_file_in_pr"
			}
		}
		return reasons
	}
	t.Cleanup(func() { checkPRScopeForTests = origCheck })

	dir := t.TempDir()

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment-refund.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should process refund"
              file="src/payment-refund.test.js" time="0.5">
      <failure message="Refund failed" type="Error">Refund failed</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-scope-72")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 55
	var issueCreated int32
	var casWritten int32
	var prCommentCreated int32
	var prCommentBody string
	server := fakePRScopeGitHubAPI(t, prNumber, &issueCreated, &casWritten, &prCommentBody, &prCommentCreated)
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
		"GITHUB_BASE_REF":                "main",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test whose file is new to this PR",
		Should:   "exit with code 0 (flaky is forgiven — passed on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a flaky test whose file is new to this PR",
		Should:   "NOT create a GitHub Issue",
		Actual:   atomic.LoadInt32(&issueCreated),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a new-to-PR flaky test (no persistent quarantine without GitHub Issue)",
		Should:   "NOT write quarantine.json",
		Actual:   atomic.LoadInt32(&casWritten),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a flaky test whose file is new to this PR",
		Should:   "post a PR comment warning the developer",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 1,
	})

	// Close server before reading prCommentBody: httptest.Server.Close() waits
	// for all handler goroutines to complete, providing the synchronization the
	// Go race detector requires before the non-atomic string read below.
	server.Close()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new-to-PR flaky test",
		Should:   "include the test name in the PR comment",
		Actual:   strings.Contains(prCommentBody, "should process refund"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new-to-PR flaky test",
		Should:   "NOT include a GitHub Issue link",
		Actual:   !strings.Contains(prCommentBody, "https://github.com/test-owner/test-repo/issues/"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new-to-PR flaky test",
		Should:   "advise developer to fix before merging",
		Actual:   strings.Contains(prCommentBody, "Please fix the flakiness before merging."),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test whose file is new to this PR",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("unmarshal results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	var found bool
	for _, tRaw := range tests {
		tMap, _ := tRaw.(map[string]interface{})
		if tMap["name"] == "should process refund" {
			found = true
			riteway.Assert(t, riteway.Case[interface{}]{
				Given:    "flaky test in a new-to-PR file",
				Should:   "have issue_skipped_reason 'new_file_in_pr' in results.json",
				Actual:   tMap["issue_skipped_reason"],
				Expected: "new_file_in_pr",
			})
		}
	}
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "results.json test entries",
		Should:   "include the flaky test entry",
		Actual:   found,
		Expected: true,
	})
}
