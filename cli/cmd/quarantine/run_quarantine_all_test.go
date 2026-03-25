package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
	"github.com/mycargus/quarantine/internal/result"
)

// --- Scenario 57: ALL tests quarantined — Jest/Vitest ---

func TestRunJestAllTestsQuarantinedEmitsWarning(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: both tests in the suite are quarantined.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should process payment",
		"should process payment",
		"src/payment.test.js",
		101,
	))
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle refund",
		"should handle refund",
		"src/payment.test.js",
		102,
	))

	// The script writes an empty XML (0 tests) — simulates Jest finding no tests to run.
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="0" failures="0" errors="0" time="0">
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	// Pre-write the empty XML — simulates Jest running with all tests excluded.
	if err := os.WriteFile(xmlPath, []byte(emptyXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// No closed issues — both are open (quarantine remains active).
	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run with all tests quarantined (0 tests ran)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run with all tests quarantined (0 tests ran)",
		Should:   "emit all-tests-quarantined WARNING",
		Actual:   strings.Contains(output, "WARNING: All tests are quarantined"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run with all tests quarantined (0 tests ran)",
		Should:   "warning message mentions reviewing and closing issues",
		Actual:   strings.Contains(output, "Review and close resolved quarantine issues"),
		Expected: true,
	})

	// Verify results.json exists and total == 0.
	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jest run with all tests quarantined",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var res result.Result
		_ = json.Unmarshal(resultsData, &res)
		riteway.Assert(t, riteway.Case[int]{
			Given:    "Jest run with all tests quarantined (0 tests ran)",
			Should:   "report total=0 in results.json",
			Actual:   res.Summary.Total,
			Expected: 0,
		})
		riteway.Assert(t, riteway.Case[int]{
			Given:    "Jest run with all tests quarantined (0 tests ran)",
			Should:   "report failed=0 in results.json",
			Actual:   res.Summary.Failed,
			Expected: 0,
		})
	}
}

// --- Scenario 58: ALL tests quarantined — RSpec ---

func TestRunRSpecAllTestsQuarantinedEmitsWarning(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: all 2 tests quarantined.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::User::is valid with valid attributes",
		"is valid with valid attributes",
		"spec/models/user_spec.rb",
		201,
	))
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::User::is invalid without email",
		"is invalid without email",
		"spec/models/user_spec.rb",
		202,
	))

	// JUnit XML: both tests fail (all quarantined, all suppressed by filtering).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="RSpec" tests="2" failures="2" errors="0">
  <testcase classname="User" name="is valid with valid attributes"
            file="spec/models/user_spec.rb" time="0.05">
    <failure message="expected true, got false">expected true, got false</failure>
  </testcase>
  <testcase classname="User" name="is invalid without email"
            file="spec/models/user_spec.rb" time="0.03">
    <failure message="expected nil, got ValidationError">expected nil, got ValidationError</failure>
  </testcase>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1) // exits 1 (failures)

	configPath := writeTempConfig(t, `
version: 1
framework: rspec
junitxml: `+xmlPath+`
`)

	// No closed issues — both are open.
	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec run with all tests quarantined (all failures suppressed)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec run with all tests quarantined (all failures suppressed)",
		Should:   "emit all-tests-quarantined WARNING",
		Actual:   strings.Contains(output, "WARNING: All tests are quarantined"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec run with all tests quarantined (all failures suppressed)",
		Should:   "warning message mentions reviewing and closing issues",
		Actual:   strings.Contains(output, "Review and close resolved quarantine issues"),
		Expected: true,
	})

	// Verify results.json: failed=0, quarantined==total.
	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "RSpec run with all tests quarantined",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var res result.Result
		_ = json.Unmarshal(resultsData, &res)
		riteway.Assert(t, riteway.Case[int]{
			Given:    "RSpec run with all 2 tests quarantined",
			Should:   "report failed=0 in results.json",
			Actual:   res.Summary.Failed,
			Expected: 0,
		})
		riteway.Assert(t, riteway.Case[int]{
			Given:    "RSpec run with all 2 tests quarantined",
			Should:   "report quarantined=2 in results.json",
			Actual:   res.Summary.Quarantined,
			Expected: 2,
		})
	}
}
