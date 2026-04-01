package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 72: Flaky test in new-to-PR file — no issue, no quarantine, warning PR comment ---

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

// --- Scenario 74: Flaky test in pre-existing file — new test case added in this PR ---

func TestRunNewTestInPreExistingFileScopedToPR(t *testing.T) {
	// Override PR scope check: src/payment.test.js is pre-existing, but
	// "should handle refund timeout" is a new test added in this PR (appears in diff lines).
	// classifyPRScope returns "new_test_in_pr" for this test.
	origCheck := checkPRScopeForTests
	checkPRScopeForTests = func(_ string, tests []prScopeInput) map[string]string {
		reasons := make(map[string]string)
		for _, inp := range tests {
			if inp.Name == "should handle refund timeout" {
				reasons[inp.TestID] = "new_test_in_pr"
			}
		}
		return reasons
	}
	t.Cleanup(func() { checkPRScopeForTests = origCheck })

	dir := t.TempDir()

	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="src/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle refund timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="Refund timed out" type="Error">Refund timed out after 5000ms</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-scope-74")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
github:
  owner: test-owner
  repo: test-repo
rerun_command: %s
`, rerunScript))

	prNumber := 56
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
		Given:    "a new test case added in this PR to a pre-existing file that is flaky",
		Should:   "exit with code 0 (flaky is forgiven — passed on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a new test case added in this PR (new_test_in_pr)",
		Should:   "NOT create a GitHub Issue",
		Actual:   atomic.LoadInt32(&issueCreated),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a new test case in this PR (no persistent quarantine without GitHub Issue)",
		Should:   "NOT write quarantine.json",
		Actual:   atomic.LoadInt32(&casWritten),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a new test case added in this PR that is flaky",
		Should:   "post a PR comment warning the developer",
		Actual:   atomic.LoadInt32(&prCommentCreated),
		Expected: 1,
	})

	// Close server before reading prCommentBody: httptest.Server.Close() waits
	// for all handler goroutines to complete, providing the synchronization the
	// Go race detector requires before the non-atomic string read below.
	server.Close()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new test case in pre-existing file",
		Should:   "include the test name in the PR comment",
		Actual:   strings.Contains(prCommentBody, "should handle refund timeout"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new test case in pre-existing file",
		Should:   "NOT include a GitHub Issue link",
		Actual:   !strings.Contains(prCommentBody, "https://github.com/test-owner/test-repo/issues/"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for new test case in pre-existing file",
		Should:   "advise developer to fix before merging",
		Actual:   strings.Contains(prCommentBody, "Please fix the flakiness before merging."),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a new test case added in this PR that is flaky",
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
		if tMap["name"] == "should handle refund timeout" {
			found = true
			riteway.Assert(t, riteway.Case[interface{}]{
				Given:    "new test case in pre-existing file",
				Should:   "have issue_skipped_reason 'new_test_in_pr' in results.json",
				Actual:   tMap["issue_skipped_reason"],
				Expected: "new_test_in_pr",
			})
		}
	}
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "results.json test entries (Scenario 74)",
		Should:   "include the flaky test entry",
		Actual:   found,
		Expected: true,
	})
}
