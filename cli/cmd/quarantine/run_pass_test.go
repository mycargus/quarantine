package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 19: Normal CI run — all tests pass ---

func TestRunAllPass(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="3" failures="0" errors="0" time="1.234">
  <testsuite name="__tests__/auth/login.test.js" tests="3" errors="0" failures="0" skipped="0"
             timestamp="2026-03-15T14:22:10" time="1.234">
    <testcase classname="LoginForm validates input" name="should reject empty email"
              file="__tests__/auth/login.test.js" time="0.045">
    </testcase>
    <testcase classname="LoginForm validates input" name="should reject invalid email format"
              file="__tests__/auth/login.test.js" time="0.032">
    </testcase>
    <testcase classname="LoginForm submits" name="should call onSubmit with credentials"
              file="__tests__/auth/login.test.js" time="0.078">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all tests pass",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all tests pass",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		jsonErr := json.Unmarshal(resultsData, &results)

		riteway.Assert(t, riteway.Case[bool]{
			Given:    "all tests pass",
			Should:   "write valid JSON to results.json",
			Actual:   jsonErr == nil,
			Expected: true,
		})

		if jsonErr == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all tests pass",
				Should:   "report total of 3 tests",
				Actual:   summary["total"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all tests pass",
				Should:   "report 3 passed",
				Actual:   summary["passed"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all tests pass",
				Should:   "report 0 failed",
				Actual:   summary["failed"].(float64),
				Expected: 0,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    "all tests pass",
				Should:   "set suite_name to unit",
				Actual:   results["suite_name"].(string),
				Expected: "unit",
			})
		}
	}

	_ = output
}

// --- Scenario 69: CI run with test failures — exit 1 ---

func TestRunTestFailuresExitOne(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="2" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/checkout.test.js" tests="2" failures="1" time="1.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">
        Error: expected 10 but got 0
      </failure>
    </testcase>
    <testcase classname="CheckoutService" name="should calculate total"
              file="__tests__/checkout.test.js" time="0.5">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	err := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one test fails",
		Should:   "exit with code 1 (test failure)",
		Actual:   err,
		Expected: 1,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "one test fails",
				Should:   "report 1 failed in results.json",
				Actual:   summary["failed"].(float64),
				Expected: 1,
			})
		}
	}
}

// --- Scenario 51 (verbose output): branch not found logs 404 ---

func TestRunBranchNotFound(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, false) // branch does not exist
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch does not exist (404)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch does not exist",
		Should:   "print 'not initialized' error message",
		Actual:   strings.Contains(output, "Run 'quarantine init' first"),
		Expected: true,
	})
}

// --- Scenario 51 (verbose output): degraded mode logs a [quarantine] WARNING ---
//
// Degraded mode is triggered by an unreachable API after Scenario 178 /
// ADR-037 — the no-token case now exits 2 before any output, so degraded
// mode warnings can only originate from runtime API failures.
func TestRunVerboseNoToken(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0" encoding="UTF-8"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "valid token but API unreachable",
		Should:   "exit without error (degraded mode continues)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "valid token but API unreachable",
		Should:   "print WARNING with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] WARNING:"),
		Expected: true,
	})
}
