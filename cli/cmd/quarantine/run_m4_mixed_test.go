package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)

// --- Scenario 25 (variant): unquarantined test fails → exits 1 ---

func TestRunUnquarantinedTestFailsExits1(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: test linked to issue #42 (closed).
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		42,
	))

	// JUnit XML: the previously-quarantined test now FAILS (genuine failure).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.1">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
retries: 1
`)

	// Issue #42 closed — test runs but fails all retries.
	// The retry will also fail.
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
		Given:    "unquarantined test (issue closed) still fails",
		Should:   "exit with code 1 (genuine failure)",
		Actual:   exitCode,
		Expected: 1,
	})
}

// --- Scenario 26: Mixed results — flaky, quarantined, real failures, passes ---

func TestRunMixedResults(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state:
	// - Test A: quarantined, issue open (stays quarantined, excluded from run)
	// - Test B: quarantined, issue closed (to unquarantine — will run)
	testAID := "src/payment.test.js::PaymentService::should handle charge timeout"
	testBID := "src/auth.test.js::AuthService::should validate token"

	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(testAID, "should handle charge timeout", "src/payment.test.js", 10))
	qs.AddTest(makeQuarantineEntry(testBID, "should validate token", "src/auth.test.js", 20))

	// Issue #20 (test B) is closed — test B is unquarantined.
	// Issue #10 (test A) is open — test A stays quarantined.
	// Tests that run: B, C (pass), D (flaky — fails then passes on retry), E (genuine failure)
	// Test A is excluded from execution (Jest pre-execution exclusion).

	// JUnit XML produced by the run (test A is excluded, so not in XML):
	// B passes, C passes, D fails, E fails
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
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1) // exits 1 (has failures)

	// Create a counter-based rerun script:
	// - First rerun call (for D) → exits 0 (D is flaky)
	// - Second rerun call (for E) → exits 1 (E is genuine)
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

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, qs, []int{20})
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

	// E (genuine failure) should cause exit 1.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "mixed results: A quarantined (excluded), B unquarantined+passes, C passes, D flaky, E genuine failure",
		Should:   "exit with code 1 (genuine failure E)",
		Actual:   exitCode,
		Expected: 1,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "mixed results run",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr != nil {
		return
	}

	var results map[string]interface{}
	_ = json.Unmarshal(resultsData, &results)

	summary, _ := results["summary"].(map[string]interface{})
	failedCount, _ := summary["failed"].(float64)
	flakyCount, _ := summary["flaky_detected"].(float64)

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "mixed results: genuine failure E, flaky D",
		Should:   "report 1 failed test in summary",
		Actual:   failedCount,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[float64]{
		Given:    "mixed results: D is flaky",
		Should:   "report 1 flaky test in summary",
		Actual:   flakyCount,
		Expected: 1,
	})
}
