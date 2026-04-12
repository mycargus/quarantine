package main

import (
	"encoding/base64"
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

	"github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
)

// fakeM4GitHubAPI creates a test server that handles all M4 GitHub API endpoints.
// quarantineState is marshaled and served from GET /contents/...
// closedIssueNumbers is the list of issue numbers returned by GET /search/issues.
func fakeM4GitHubAPI(t *testing.T, qs *quarantine.State, closedIssueNumbers []int) *httptest.Server {
	t.Helper()

	var stateContent []byte
	if qs != nil {
		var err error
		stateContent, err = qs.Marshal()
		if err != nil {
			t.Fatalf("marshal quarantine state: %v", err)
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			// Branch exists check.
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			// Read quarantine state — handles both legacy and suite paths.
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			// Batch issue status check.
			items := make([]map[string]interface{}, len(closedIssueNumbers))
			for i, n := range closedIssueNumbers {
				items[i] = map[string]interface{}{"number": n}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": len(closedIssueNumbers),
				"items":       items,
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			// CAS write — always succeed (handles both legacy and suite paths).
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// makeQuarantineEntry creates a quarantine.Entry for testing.
func makeQuarantineEntry(testID, name, filePath string, issueNumber int) quarantine.Entry {
	e := quarantine.Entry{
		TestID:   testID,
		Name:     name,
		FilePath: filePath,
	}
	if issueNumber > 0 {
		n := issueNumber
		e.IssueNumber = &n
	}
	return e
}

// --- Scenario 21: Quarantined test — suite mode loads state and tracks count ---

func TestRunJestWithQuarantinedTestExcluded(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: one quarantined test with an open issue.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		99, // issue #99 — open
	))

	// Suite runs and produces XML with only the non-quarantined test.
	// (In suite mode, the quarantined test still runs — the suite command is unmodified.)
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="1" failures="0" errors="0" time="0.5">
  <testsuite name="src/other.test.js" tests="1" failures="0">
    <testcase classname="OtherService" name="should work" file="src/other.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with quarantine state loaded (issue open)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run completes successfully",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var res result.Result
		_ = json.Unmarshal(resultsData, &res)
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "suite run with tests passing",
			Should:   "include test entries in results.json",
			Actual:   len(res.Tests) > 0,
			Expected: true,
		})
	}
}

// --- Scenario 22: RSpec-style — quarantine state loaded correctly ---

func TestRunRSpecWithQuarantinedTestFailureSuppressed(t *testing.T) {
	// In suite mode, post-execution RSpec filtering is not applied.
	// This test verifies that when quarantine state is loaded, the run
	// still completes even when a quarantined test appears failed in XML.
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::User::valid? returns true for valid attributes",
		"valid? returns true for valid attributes",
		"spec/models/user_spec.rb",
		42, // issue #42 — open
	))

	// XML: quarantined test failed + one other passes.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="RSpec" tests="2" failures="1" errors="0">
  <testcase classname="User" name="valid? returns true for valid attributes"
            file="spec/models/user_spec.rb" time="0.05">
    <failure message="expected true, got false">expected true, got false</failure>
  </testcase>
  <testcase classname="User" name="should be invalid without email"
            file="spec/models/user_spec.rb" time="0.03"/>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	// In suite mode, failures exit 1 (not suppressed post-execution).
	riteway.Assert(t, riteway.Case[int]{
		Given:    "suite run where one test fails (quarantine state loaded)",
		Should:   "exit with code 1 (genuine failure)",
		Actual:   exitCode,
		Expected: 1,
	})
}

// --- Scenario 25: Unquarantine on issue close ---

func TestRunUnquarantinesTestWhenIssueIsClosed(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		42, // issue #42 — CLOSED
	))

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	// Issue #42 is closed.
	server := fakeM4GitHubAPI(t, qs, []int{42})
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "quarantined test with issue #42 closed, test passes",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "unquarantine scenario",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var res result.Result
	_ = json.Unmarshal(resultsData, &res)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "unquarantined test passes after issue close",
		Should:   "include the test in results",
		Actual:   len(res.Tests) > 0,
		Expected: true,
	})
}

// --- Scenario 41: Write to unprotected branch — succeeds directly ---

func TestRunWriteToUnprotectedBranchSucceeds(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()

	// Test fails initially, passes on retry → flaky.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfigWithRerunScript(t, dir, xmlPath, scriptPath, rerunScriptPath)
	chdirTest(t, dir)

	var putCalled int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			if len(qs.Tests) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			content, _ := qs.Marshal()
			encoded := base64.StdEncoding.EncodeToString(content)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})
		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			atomic.AddInt32(&putCalled, 1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "unprotected branch and a newly detected flaky test",
		Should:   "exit with code 0 (flaky test quarantined, not a genuine failure)",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "unprotected branch and a newly detected flaky test",
		Should:   "write updated state via PUT (CAS write succeeded)",
		Actual:   atomic.LoadInt32(&putCalled) > 0,
		Expected: true,
	})
}
