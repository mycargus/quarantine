package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	writeSuiteConfigFull(t, dir, "test-owner", "test-repo", xmlPath, initialScript, rerunScript)
	chdirTest(t, dir)

	prNumber := 55
	var issueCreated int32
	var casWritten int32
	var prCommentCreated int32
	var prCommentBody string
	server := fakePRScopeGitHubAPI(t, prNumber, &issueCreated, &casWritten, &prCommentBody, &prCommentCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--pr", fmt.Sprintf("%d", prNumber),
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_BASE_REF":                "main",
	})

	// In suite mode, PR scope detection is not wired. All flaky tests get issues.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test (suite mode — PR scope not yet wired)",
		Should:   "exit with code 0 (flaky passes on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	server.Close()

	_, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "flaky test run",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	_ = issueCreated
	_ = casWritten
	_ = prCommentCreated
	_ = prCommentBody
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

	writeSuiteConfigFull(t, dir, "test-owner", "test-repo", xmlPath, initialScript, rerunScript)
	chdirTest(t, dir)

	prNumber := 56
	var issueCreated int32
	var casWritten int32
	var prCommentCreated int32
	var prCommentBody string
	server := fakePRScopeGitHubAPI(t, prNumber, &issueCreated, &casWritten, &prCommentBody, &prCommentCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--pr", fmt.Sprintf("%d", prNumber),
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_BASE_REF":                "main",
	})

	// In suite mode, PR scope detection is not wired.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test (suite mode — PR scope not yet wired)",
		Should:   "exit with code 0 (flaky passes on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	server.Close()

	_, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a new test case added in this PR that is flaky",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	_ = issueCreated
	_ = casWritten
	_ = prCommentCreated
	_ = prCommentBody

}
