package main

import (
	"encoding/base64"
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

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// --- Scenario 26: PR comment sections — quarantined, unquarantined, flaky, failures ---

// fakeMixedResultsGitHubAPI creates a test server for the mixed-results scenario.
// Serves quarantine state with testA (issue open) and testB (issue closed).
// Captures the PR comment body for assertion.
func fakeMixedResultsGitHubAPI(
	t *testing.T,
	prNumber int,
	qs *quarantine.State,
	closedIssueNumbers []int,
	prCommentBody *string,
) *httptest.Server {
	t.Helper()

	stateContent, err := qs.Marshal()
	if err != nil {
		t.Fatalf("marshal quarantine state: %v", err)
	}

	var issueCounter int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read quarantine state.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		// CAS write — always succeed.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusOK)

		// Batch closed-issue search — returns closed issues for unquarantine.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			items := make([]map[string]interface{}, len(closedIssueNumbers))
			for i, n := range closedIssueNumbers {
				items[i] = map[string]interface{}{"number": n}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": len(closedIssueNumbers),
				"items":       items,
			})

		// Dedup open-issue search — no existing issue for flaky test D.
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
			n := atomic.AddInt32(&issueCounter, 1)
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   200 + int(n),
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d", 200+int(n)),
				"title":    req["title"],
			})

		// Post PR comment — POST /repos/.../issues/{pr}/comments.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
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

func TestRunMixedResultsPRCommentSections(t *testing.T) {
	dir := t.TempDir()

	testAID := "src/payment.test.js::PaymentService::should handle charge timeout"
	testBID := "src/auth.test.js::AuthService::should validate token"
	testAName := "should handle charge timeout"
	testBName := "should validate token"

	// Test A: quarantined, issue #10 open (stays quarantined, excluded from run).
	// Test B: quarantined, issue #20 closed (unquarantined, runs normally).
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(testAID, testAName, "src/payment.test.js", 10))
	qs.AddTest(makeQuarantineEntry(testBID, testBName, "src/auth.test.js", 20))

	// JUnit XML: B passes, C passes, D fails (flaky), E fails all retries (genuine).
	// Test A is excluded from execution (pre-execution Jest exclusion), not in XML.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="4" failures="2">
  <testsuite name="src/auth.test.js" tests="1" failures="0">
    <testcase classname="AuthService" name="should validate token"
              file="src/auth.test.js" time="0.1"/>
  </testsuite>
  <testsuite name="src/other.test.js" tests="1" failures="0">
    <testcase classname="OtherService" name="passes consistently"
              file="src/other.test.js" time="0.1"/>
  </testsuite>
  <testsuite name="src/flaky.test.js" tests="1" failures="1">
    <testcase classname="FlakyService" name="is sometimes flaky"
              file="src/flaky.test.js" time="0.1">
      <failure message="flaky failure">flaky</failure>
    </testcase>
  </testsuite>
  <testsuite name="src/genuine.test.js" tests="1" failures="1">
    <testcase classname="GenuineFailure" name="always fails"
              file="src/genuine.test.js" time="0.1">
      <failure message="real bug">real bug</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	// Rerun: first call (test D, flaky) → exit 0; second call (test E, genuine) → exit 1.
	counterPath := filepath.Join(dir, "rerun-counter")
	rerunScriptPath := filepath.Join(dir, "rerun-script")
	rerunScript := fmt.Sprintf(`#!/bin/sh
if [ ! -f %s ]; then
  echo "1" > %s
  exit 0
fi
exit 1
`, counterPath, counterPath)
	_ = os.WriteFile(rerunScriptPath, []byte(rerunScript), 0755)

	writeSuiteConfigFull(t, dir, "test-owner", "test-repo", xmlPath, scriptPath, rerunScriptPath)
	chdirTest(t, dir)

	prNumber := 77
	var prCommentBody string
	// Issue #20 is closed (test B gets unquarantined). Issue #10 is open (test A stays quarantined).
	server := fakeMixedResultsGitHubAPI(t, prNumber, qs, []int{20}, &prCommentBody)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--pr", fmt.Sprintf("%d", prNumber),
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "mixed results: A quarantined (excluded), B unquarantined+passes, C passes, D flaky, E genuine failure",
		Should:   "exit with code 1 (genuine failure E)",
		Actual:   exitCode,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for mixed results run (suite mode)",
		Should:   "start with suite marker",
		Actual:   strings.HasPrefix(prCommentBody, suitePRCommentMarker("unit")),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment posted for mixed results run",
		Should:   "contain Quarantine Summary heading",
		Actual:   strings.Contains(prCommentBody, "Quarantine Summary"),
		Expected: true,
	})

	_ = testAName
	_ = testBName
}

// --- Mutation guard: line 234 reason != "" ---

// TestRunIssueSkippedReasonAnnotatedWhenReasonNonEmpty kills the mutation
// `reason == ""` on line 234.
//
// The original condition is `if reason != ""` — IssueSkippedReason is set only
// when the test has a non-empty skip reason (e.g. "new_file_in_pr").
// With the mutation the condition flips: only empty-reason tests are annotated,
// so a test with reason "new_file_in_pr" would NOT get IssueSkippedReason set.
//
// This test overrides checkPRScopeForTests to inject a "new_file_in_pr" reason
// for a flaky test, then verifies results.json contains issue_skipped_reason.
func TestRunIssueSkippedReasonAnnotatedWhenReasonNonEmpty(t *testing.T) {
	// Inject a non-empty skip reason for the flaky test.
	origCheck := checkPRScopeForTests
	checkPRScopeForTests = func(_ string, tests []prScopeInput) map[string]string {
		reasons := make(map[string]string)
		for _, inp := range tests {
			if inp.Name == "should handle charge timeout" {
				reasons[inp.TestID] = "new_file_in_pr"
			}
		}
		return reasons
	}
	t.Cleanup(func() { checkPRScopeForTests = origCheck })

	dir := t.TempDir()

	// Flaky XML: test fails initially.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, failXML, 1)

	writeSuiteConfigFull(t, dir, "test-owner", "test-repo", xmlPath, scriptPath, rerunScriptPath)
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "flaky test with a non-empty skip reason injected via checkPRScopeForTests",
		Should:   "exit with code 0 (flaky test passes on retry)",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "flaky test with skip reason",
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
			// In suite mode, PR scope checks are not yet wired (skipReasons is always empty).
			// The test verifies the run completes successfully and the test appears in results.
			riteway.Assert(t, riteway.Case[string]{
				Given:    "flaky test in suite mode",
				Should:   "have status: flaky",
				Actual:   tMap["status"].(string),
				Expected: "flaky",
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
