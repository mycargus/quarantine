package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// writeAlwaysFailScript creates a script that always exits 1.
func writeAlwaysFailScript(t *testing.T, dir, name string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, name)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("writeAlwaysFailScript: %v", err)
	}
	return scriptPath
}

// writeAlwaysPassScript creates a script that always exits 0.
func writeAlwaysPassScript(t *testing.T, dir, name string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, name)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("writeAlwaysPassScript: %v", err)
	}
	return scriptPath
}

// --- Scenario 23: CI run with a real failure ---

func TestRunGenuineFailure(t *testing.T) {
	dir := t.TempDir()

	// Initial test script: writes failed JUnit XML and exits 1.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/checkout.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">expected 10 but got 0</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)

	// Rerun script: always exits 1 (test always fails).
	rerunScript := writeAlwaysFailScript(t, dir, "rerun-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
rerun_command: %s
`, rerunScript))

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one test fails all 3 retries",
		Should:   "exit with code 1 (genuine failure)",
		Actual:   exitCode,
		Expected: 1,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	summary, _ := results["summary"].(map[string]interface{})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "test fails all retries",
		Should:   "report failed: 1",
		Actual:   summary["failed"].(float64),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "test fails all retries (no flaky)",
		Should:   "report flaky_detected: 0",
		Actual:   summary["flaky_detected"].(float64),
		Expected: 0,
	})

	tests, _ := results["tests"].([]interface{})
	if len(tests) == 0 {
		t.Fatal("expected 1 test entry")
	}
	testEntry, _ := tests[0].(map[string]interface{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test fails all retries",
		Should:   "have status: failed",
		Actual:   testEntry["status"].(string),
		Expected: "failed",
	})

	retries, _ := testEntry["retries"].([]interface{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "test retried 3 times (all fail)",
		Should:   "have 3 retry entries",
		Actual:   len(retries),
		Expected: 3,
	})

	for i, r := range retries {
		entry, _ := r.(map[string]interface{})
		riteway.Assert(t, riteway.Case[float64]{
			Given:    fmt.Sprintf("retry entry %d", i+1),
			Should:   "have correct attempt number",
			Actual:   entry["attempt"].(float64),
			Expected: float64(i + 1),
		})
		riteway.Assert(t, riteway.Case[string]{
			Given:    fmt.Sprintf("retry entry %d", i+1),
			Should:   "have status: failed",
			Actual:   entry["status"].(string),
			Expected: "failed",
		})
	}
}

// --- Scenario 20: CI run detects a new flaky test ---

func TestRunDetectsFlakyTest(t *testing.T) {
	dir := t.TempDir()

	// Initial test script: writes failed JUnit XML and exits 1.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/payment.test.js" tests="1" failures="1" time="1.0">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="__tests__/payment.test.js" time="0.5">
      <failure message="Timeout exceeded" type="Error">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	// writeTestScript writes the XML before the script runs and the script exits 1.
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)

	// Rerun script: always exits 0 (test passes on retry).
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
rerun_command: %s
`, rerunScript))

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one test fails initially but passes on first retry",
		Should:   "exit with code 0 (flaky only, no genuine failures)",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test detected",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var results map[string]interface{}
	jsonErr := json.Unmarshal(resultsData, &results)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test detected",
		Should:   "write valid JSON",
		Actual:   jsonErr == nil,
		Expected: true,
	})

	if jsonErr != nil {
		return
	}

	summary, _ := results["summary"].(map[string]interface{})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "one test is flaky",
		Should:   "report flaky_detected: 1",
		Actual:   summary["flaky_detected"].(float64),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "one test is flaky (no genuine failures)",
		Should:   "report failed: 0",
		Actual:   summary["failed"].(float64),
		Expected: 0,
	})

	tests, _ := results["tests"].([]interface{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "one test in the run",
		Should:   "have 1 test entry",
		Actual:   len(tests),
		Expected: 1,
	})

	if len(tests) == 0 {
		return
	}

	testEntry, _ := tests[0].(map[string]interface{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test failed initially but passed on retry",
		Should:   "have status: flaky",
		Actual:   testEntry["status"].(string),
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test failed initially but passed on retry",
		Should:   "have original_status: failed",
		Actual:   testEntry["original_status"].(string),
		Expected: "failed",
	})

	retries, _ := testEntry["retries"].([]interface{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "test passed on first retry attempt",
		Should:   "have 1 retry entry",
		Actual:   len(retries),
		Expected: 1,
	})

	if len(retries) == 0 {
		return
	}

	retryEntry, _ := retries[0].(map[string]interface{})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "first retry attempt",
		Should:   "have attempt: 1",
		Actual:   retryEntry["attempt"].(float64),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "first retry passed",
		Should:   "have status: passed",
		Actual:   retryEntry["status"].(string),
		Expected: "passed",
	})
}

// --- Scenario 24: Multiple flaky tests ---

func TestRunMultipleFlakyTests(t *testing.T) {
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
rerun_command: %s
`, rerunScript))

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", initialScript,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two tests fail initially but both pass on retry",
		Should:   "exit with code 0 (flaky only, no genuine failures)",
		Actual:   exitCode,
		Expected: 0,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	summary, _ := results["summary"].(map[string]interface{})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "two tests are flaky",
		Should:   "report flaky_detected: 2",
		Actual:   summary["flaky_detected"].(float64),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "two tests are flaky (no genuine failures)",
		Should:   "report failed: 0",
		Actual:   summary["failed"].(float64),
		Expected: 0,
	})
}
