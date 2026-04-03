package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// --- Mutation guard: line 165 qState != nil ---

// TestRunRemoveUnquarantinedTestsCalledWhenQStateNonNil kills the mutation
// `qState == nil` on line 165.
//
// The original condition is `if qState != nil` — removeUnquarantinedTests is
// called only when state is loaded. With the mutation the condition flips and
// removeUnquarantined is NEVER called for a loaded state.
//
// Observable effect: when an issue is closed, stateChanged = true only if
// removeUnquarantinedTests fires and returns a non-empty removedEntries list.
// If it doesn't fire, removedTestIDs is empty, and with no new flaky tests
// stateChanged=false, so the PUT (state write) never happens.
// This test verifies the PUT IS called (state updated) when the only change is
// a removed-by-closed-issue entry.
func TestRunRemoveUnquarantinedTestsCalledWhenQStateNonNil(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: test linked to issue #42 (will be closed).
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should process payment",
		"should process payment",
		"src/payment.test.js",
		42,
	))

	// JUnit XML: the formerly-quarantined test passes (no new flaky).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Issue #42 is closed — removeUnquarantinedTests should fire and detect the removal.
	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, qs, []int{42}, &putCalled)
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
		Given:    "non-nil qState with one test whose issue is closed",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	// removeUnquarantinedTests must have fired: it returns removedEntries=[entry42],
	// making stateChanged=true and triggering the PUT write.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "qState is non-nil and issue #42 is closed (removeUnquarantinedTests must be called)",
		Should:   "write updated state via PUT (removedTestIDs is non-empty)",
		Actual:   atomic.LoadInt32(&putCalled) > 0,
		Expected: true,
	})
}

// --- Mutation guard: line 209 qState != nil && framework == RSpec ---

// TestRunRSpecFilteringSkippedWhenQStateNil kills the mutation
// `qState != nil || runner.Framework(cfg.Framework) == runner.RSpec` on line 209.
//
// Original: both conditions must be true (qState loaded AND RSpec framework).
// Mutation: either condition suffices (OR).
//
// With mutation: when qState is nil but framework=rspec, FilterQuarantinedFailures
// is called with a nil state pointer, causing a nil-pointer panic and a test failure.
// This test provides qState=nil (no token, degraded mode) + framework=rspec.
// The original exits cleanly with code 1 (failure not suppressed).
// The mutation panics.
func TestRunRSpecFilteringSkippedWhenQStateNil(t *testing.T) {
	dir := t.TempDir()

	// RSpec JUnit XML: one test fails (quarantined-looking but no state loaded).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="RSpec" tests="1" failures="1" errors="0">
  <testcase classname="User" name="valid? returns true for valid attributes"
            file="spec/models/user_spec.rb" time="0.05">
    <failure message="expected true, got false">expected true, got false</failure>
  </testcase>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	rerunScript := writeAlwaysFailScript(t, dir, "rerun-rspec-filtering")

	// Config: framework=rspec, but NO token → ghClient=nil → qState=nil.
	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: rspec
github:
  owner: testowner
  repo: testrepo
rerun_command: %s
`, rerunScript))

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		// No token → degraded mode, qState=nil.
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	// With original code: no post-filtering (qState is nil), failure counts, exit 1.
	// With mutation:      FilterQuarantinedFailures called with nil state → panic.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "RSpec framework with nil qState (no token, degraded mode)",
		Should:   "exit 1 (failure is not filtered when qState is nil)",
		Actual:   exitCode,
		Expected: 1,
	})
}

// TestRunJestFilteringSkippedEvenWhenQStateNonNil kills the mutation from the
// other direction: when qState is non-nil but framework is NOT RSpec, no
// post-filtering must happen.
//
// With the mutation (`||`): qState != nil satisfies the OR, so
// FilterQuarantinedFailures is called for Jest too. A quarantined test that
// appears as failed in the XML would be wrongly suppressed (exit 0 instead of
// exit 1).
func TestRunJestFilteringSkippedEvenWhenQStateNonNil(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: the failing test IS in state (as if it was quarantined).
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		55,
	))

	// JUnit XML (Jest): the quarantined test FAILS in the XML output.
	// For Jest this should NOT trigger post-execution filtering — the failure stands.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="src/payment.test.js" time="0.5">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	rerunScript := writeAlwaysFailScript(t, dir, "rerun-jest-filtering")

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
rerun_command: %s
`, rerunScript))

	// Issue #55 is open.
	server := fakeM4GitHubAPI(t, qs, []int{})
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

	// Original: no post-filtering for Jest (&&), failure stands → exit 1.
	// Mutation: post-filtering runs (|| with qState != nil = true) → exit 0 (wrong).
	riteway.Assert(t, riteway.Case[int]{
		Given:    "Jest framework with non-nil qState: quarantined test appears as failed in XML",
		Should:   "exit 1 (RSpec post-filtering must NOT apply to Jest)",
		Actual:   exitCode,
		Expected: 1,
	})
}
