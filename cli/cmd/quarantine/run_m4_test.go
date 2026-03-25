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
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)

// fakeM4GitHubAPI creates a test server that handles all M4 GitHub API endpoints.
// quarantineState is marshaled and served from GET /contents/quarantine.json.
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

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			// Read quarantine state.
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

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			// CAS write — always succeed.
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

// --- Scenario 21: Quarantined test exclusion — Jest/Vitest (pre-execution) ---

func TestRunJestWithQuarantinedTestExcluded(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: one quarantined test with an open issue.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		99, // issue #99 — open (not in closedIssueNumbers)
	))

	// The fake test command produces a JUnit XML with only the non-quarantined tests.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="0" errors="0" time="0.5">
  <testsuite name="src/other.test.js" tests="1" failures="0">
    <testcase classname="OtherService" name="should work" file="src/other.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// No closed issues — issue #99 is still open.
	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	var capturedArgs string
	scriptContent := fmt.Sprintf(
		"#!/bin/sh\necho \"$@\" > %s/captured-args.txt\ncp %s %s\nexit 0\n",
		dir, xmlPath, xmlPath,
	)
	_ = os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run with one quarantined test (issue open)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	// Read captured args to verify --testNamePattern was passed.
	argsBytes, readErr := os.ReadFile(filepath.Join(dir, "captured-args.txt"))
	if readErr == nil {
		capturedArgs = string(argsBytes)
	}
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with one quarantined test",
		Should:   "augment the command with --testNamePattern exclusion flag",
		Actual:   strings.Contains(capturedArgs, "--testNamePattern"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest with quarantined test 'should handle charge timeout'",
		Should:   "include the test name in the exclusion pattern",
		Actual:   strings.Contains(capturedArgs, "should handle charge timeout"),
		Expected: true,
	})

	// Verify results.json was written and exit code is 0.
	_, readErr = os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run completes successfully",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})
}

// --- Scenario 22: Quarantined test exclusion — RSpec (post-execution filtering) ---

func TestRunRSpecWithQuarantinedTestFailureSuppressed(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: one quarantined test.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::User::valid? returns true for valid attributes",
		"valid? returns true for valid attributes",
		"spec/models/user_spec.rb",
		42, // issue #42 — open (not in closedIssueNumbers)
	))

	// JUnit XML: quarantined test FAILED + one other test passes.
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
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1) // exits 1 (failure)

	configPath := writeTempConfig(t, `
version: 1
framework: rspec
junitxml: `+xmlPath+`
`)

	// No closed issues — issue #42 is still open.
	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "RSpec run where quarantined test failed (issue open)",
		Should:   "exit with code 0 (quarantined failure suppressed)",
		Actual:   exitCode,
		Expected: 0,
	})

	// Verify results.json has the quarantined test with status="quarantined".
	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec run with suppressed quarantined failure",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var results map[string]interface{}
	_ = json.Unmarshal(resultsData, &results)

	tests, _ := results["tests"].([]interface{})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec results with quarantined failure suppressed",
		Should:   "include at least one test entry",
		Actual:   len(tests) > 0,
		Expected: true,
	})

	// Find the quarantined test entry.
	var quarantinedStatus string
	for _, raw := range tests {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if name, _ := entry["name"].(string); strings.Contains(name, "valid? returns true") {
			quarantinedStatus, _ = entry["status"].(string)
			break
		}
	}

	riteway.Assert(t, riteway.Case[string]{
		Given:    "RSpec quarantined test that failed",
		Should:   "have status 'quarantined' in results.json",
		Actual:   quarantinedStatus,
		Expected: "quarantined",
	})

	summary, _ := results["summary"].(map[string]interface{})
	failedCount, _ := summary["failed"].(float64)
	riteway.Assert(t, riteway.Case[float64]{
		Given:    "RSpec run with one quarantined failure suppressed",
		Should:   "report 0 failures in summary",
		Actual:   failedCount,
		Expected: 0,
	})
}

// --- Scenario 25: Unquarantine on issue close ---

func TestRunUnquarantinesTestWhenIssueIsClosed(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: test linked to issue #42.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		42, // issue #42 — CLOSED
	))

	// JUnit XML: the previously-quarantined test now passes.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Issue #42 is closed.
	server := fakeM4GitHubAPI(t, qs, []int{42})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
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

	// Verify test runs normally — the test should appear in results as "passed",
	// not excluded (because issue was closed, test is unquarantined, so it ran).
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

	var results map[string]interface{}
	_ = json.Unmarshal(resultsData, &results)
	tests, _ := results["tests"].([]interface{})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "unquarantined test passes after issue close",
		Should:   "include the test in results",
		Actual:   len(tests) > 0,
		Expected: true,
	})
}

