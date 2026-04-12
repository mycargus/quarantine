package main

import (
	"path/filepath"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// --- Mutation guard: qState != nil ---

// TestRunRemoveUnquarantinedTestsCalledWhenQStateNonNil kills the mutation
// `qState == nil` — removeUnquarantinedTests is called only when state is loaded.
func TestRunRemoveUnquarantinedTestsCalledWhenQStateNonNil(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should process payment",
		"should process payment",
		"src/payment.test.js",
		42,
	))

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	// Issue #42 is closed.
	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, qs, []int{42}, &putCalled)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
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

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "qState is non-nil and issue #42 is closed (removeUnquarantinedTests must be called)",
		Should:   "write updated state via PUT (removedTestIDs is non-empty)",
		Actual:   atomic.LoadInt32(&putCalled) > 0,
		Expected: true,
	})
}

// TestRunRSpecFilteringSkippedWhenQStateNil: in suite mode, there is no RSpec
// post-execution filtering. A failing test exits 1 regardless of qState.
func TestRunRSpecFilteringSkippedWhenQStateNil(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="RSpec" tests="1" failures="1" errors="0">
  <testcase classname="User" name="valid? returns true"
            file="spec/models/user_spec.rb" time="0.05">
    <failure message="expected true, got false">expected true, got false</failure>
  </testcase>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "suite with failing test and no token (degraded mode, qState=nil)",
		Should:   "exit 1 (failure not suppressed)",
		Actual:   exitCode,
		Expected: 1,
	})
}

// TestRunJestFilteringSkippedEvenWhenQStateNonNil: in suite mode, there is no
// post-execution filtering for any framework. A failing test that happens to be
// in quarantine state still causes exit 1.
func TestRunJestFilteringSkippedEvenWhenQStateNonNil(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"src/payment.test.js::PaymentService::should handle charge timeout",
		"should handle charge timeout",
		"src/payment.test.js",
		55,
	))

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

	// In suite mode, no post-execution filtering. Failure stands → exit 1.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "suite with failing test that is in quarantine state (no post-filtering in suite mode)",
		Should:   "exit 1 (failure not suppressed)",
		Actual:   exitCode,
		Expected: 1,
	})
}
