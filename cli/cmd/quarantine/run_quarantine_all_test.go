package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
)

// --- False positive guard: no quarantine state + empty results should NOT warn ---

func TestRunEmptyResultsWithoutQuarantineStateDoesNotWarn(t *testing.T) {
	dir := t.TempDir()

	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="0" failures="0" errors="0" time="0">
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(emptyXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	// Degraded mode is triggered by an unreachable API (a valid token is
	// required after Scenario 178 / ADR-037 — the no-token case now exits 2).
	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run with no quarantine state (degraded mode) and 0 test results",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run with no quarantine state and 0 test results",
		Should:   "NOT emit the all-tests-quarantined warning",
		Actual:   strings.Contains(output, "All tests are quarantined"),
		Expected: false,
	})
}

// --- Scenario 57: ALL tests quarantined (suite mode) ---

func TestRunJestAllTestsQuarantinedEmitsWarning(t *testing.T) {
	dir := t.TempDir()

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

	// Empty XML (0 tests) — suite ran but no tests executed.
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="0" failures="0" errors="0" time="0">
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(emptyXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with all tests quarantined (0 tests ran)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with all tests quarantined (0 tests ran)",
		Should:   "emit all-tests-quarantined WARNING",
		Actual:   strings.Contains(output, "WARNING: All tests are quarantined"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with all tests quarantined (0 tests ran)",
		Should:   "warning message mentions reviewing and closing issues",
		Actual:   strings.Contains(output, "Review and close resolved quarantine issues"),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var res result.Result
		_ = json.Unmarshal(resultsData, &res)
		riteway.Assert(t, riteway.Case[int]{
			Given:    "suite run with all tests quarantined",
			Should:   "report total=0",
			Actual:   res.Summary.Total,
			Expected: 0,
		})
	}
}

// --- Scenario 58: ALL tests quarantined — RSpec style (suite mode) ---

func TestRunRSpecAllTestsQuarantinedEmitsWarning(t *testing.T) {
	dir := t.TempDir()

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

	// Empty XML — suite ran but quarantined tests weren't included.
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="tests" tests="0" failures="0" errors="0" time="0">
</testsuites>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	if err := os.WriteFile(xmlPath, []byte(emptyXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with all tests quarantined (0 tests in XML)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite run with all tests quarantined",
		Should:   "emit all-tests-quarantined WARNING",
		Actual:   strings.Contains(output, "WARNING: All tests are quarantined"),
		Expected: true,
	})
}

