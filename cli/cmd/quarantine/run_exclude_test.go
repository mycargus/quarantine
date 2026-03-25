package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)

// --- Scenario 46: --exclude patterns (integration) ---

func TestRunExcludePatternMatchingConfigPatternSuppressesRetry(t *testing.T) {
	dir := t.TempDir()

	// Test fails on first run — matches exclude pattern from config.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="test/integration/api_test.js" tests="1" failures="1">
    <testcase classname="ApiTest" name="should connect"
              file="test/integration/api_test.js" time="0.5">
      <failure message="connection refused">connection refused</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	// Rerun script always exits 0 — but it should NEVER be called for an excluded test.
	rerunCallCount := 0
	rerunScriptPath := filepath.Join(dir, "rerun")
	rerunScript := fmt.Sprintf("#!/bin/sh\necho RERUN_CALLED >> %s/rerun_calls.log\nexit 0\n", dir)
	if err := os.WriteFile(rerunScriptPath, []byte(rerunScript), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	_ = rerunCallCount

	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	// Config includes exclude pattern that matches the test's file path.
	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
exclude:
  - "test/integration/**"
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
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
		Given:    "test matches exclude pattern from config, test fails",
		Should:   "exit with code 1 (excluded test failure counts toward exit code)",
		Actual:   exitCode,
		Expected: 1,
	})

	// The rerun log should NOT exist — excluded tests must not be retried.
	rerunLogPath := filepath.Join(dir, "rerun_calls.log")
	_, statErr := os.Stat(rerunLogPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test matches exclude pattern from config",
		Should:   "not retry the excluded test",
		Actual:   os.IsNotExist(statErr),
		Expected: true,
	})
}

func TestRunExcludeFlagPatternSuppressesRetryAndQuarantine(t *testing.T) {
	dir := t.TempDir()

	// Test fails on first run — matches exclude pattern from --exclude flag.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/slow.test.js" tests="1" failures="1">
    <testcase classname="SlowServiceTest" name="should timeout"
              file="src/slow.test.js" time="5.0">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	// Rerun always passes — but should never be called for excluded test.
	rerunScriptPath := filepath.Join(dir, "rerun")
	rerunScript := fmt.Sprintf("#!/bin/sh\necho RERUN_CALLED >> %s/rerun_calls.log\nexit 0\n", dir)
	if err := os.WriteFile(rerunScriptPath, []byte(rerunScript), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}

	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--exclude", "**::SlowServiceTest::*",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "test matches --exclude flag pattern, test fails",
		Should:   "exit with code 1 (excluded test failure counts toward exit code)",
		Actual:   exitCode,
		Expected: 1,
	})

	// The rerun log should NOT exist — excluded test must not be retried.
	rerunLogPath := filepath.Join(dir, "rerun_calls.log")
	_, statErr := os.Stat(rerunLogPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test matches --exclude flag pattern",
		Should:   "not retry the excluded test",
		Actual:   os.IsNotExist(statErr),
		Expected: true,
	})
}

func TestRunExcludePatternNonMatchingTestIsStillRetried(t *testing.T) {
	dir := t.TempDir()

	// Test fails — does NOT match exclude pattern → should be retried and detected as flaky.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/unit.test.js" tests="1" failures="1">
    <testcase classname="UnitService" name="should compute"
              file="src/unit.test.js" time="0.1">
      <failure message="unexpected value">unexpected value</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}

	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	// Exclude only integration tests — unit test should still be retried.
	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
exclude:
  - "test/integration/**"
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
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
		Given:    "test does not match exclude pattern, fails then passes on retry",
		Should:   "exit with code 0 (flaky — not a genuine failure)",
		Actual:   exitCode,
		Expected: 0,
	})
}

func TestRunExcludePatternMergesConfigAndFlagPatterns(t *testing.T) {
	dir := t.TempDir()

	// Two tests fail: one matches config pattern, one matches --exclude flag.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2" failures="2">
  <testsuite name="test/integration/api_test.js" tests="1" failures="1">
    <testcase classname="ApiTest" name="should connect"
              file="test/integration/api_test.js" time="0.5">
      <failure message="connection refused">connection refused</failure>
    </testcase>
  </testsuite>
  <testsuite name="src/slow.test.js" tests="1" failures="1">
    <testcase classname="SlowServiceTest" name="should timeout"
              file="src/slow.test.js" time="5.0">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")

	// Rerun always passes — but neither excluded test should be retried.
	rerunScriptPath := filepath.Join(dir, "rerun")
	rerunScript := fmt.Sprintf("#!/bin/sh\necho RERUN_CALLED >> %s/rerun_calls.log\nexit 0\n", dir)
	if err := os.WriteFile(rerunScriptPath, []byte(rerunScript), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}

	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	// Config excludes integration tests; flag excludes SlowServiceTest.
	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
exclude:
  - "test/integration/**"
`, rerunScriptPath))

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--exclude", "**::SlowServiceTest::*",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two tests fail, both match exclude patterns (config + flag)",
		Should:   "exit with code 1 (both excluded failures count toward exit code)",
		Actual:   exitCode,
		Expected: 1,
	})

	// Neither excluded test should have triggered a rerun.
	rerunLogPath := filepath.Join(dir, "rerun_calls.log")
	_, statErr := os.Stat(rerunLogPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "both failing tests match exclude patterns",
		Should:   "not retry any excluded test",
		Actual:   os.IsNotExist(statErr),
		Expected: true,
	})
}

// TestRunExcludePatternTestIDFormatsMatch verifies that the pattern
// "test/integration/**" correctly matches the test_id format used in runner
// (file_path::classname::name).
func TestRunExcludePatternDoesNotQuarantineExcludedTest(t *testing.T) {
	dir := t.TempDir()

	// Test matches exclude pattern and always fails (never passes on retry).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="test/integration/db_test.js" tests="1" failures="1">
    <testcase classname="DbTest" name="should connect to database"
              file="test/integration/db_test.js" time="2.0">
      <failure message="connection refused">connection refused</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
exclude:
  - "test/integration/**"
`)

	// Track whether any PUT (quarantine write) was called.
	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, quarantine.NewEmptyState(), []int{}, &putCalled)
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
		Given:    "test matches exclude pattern and always fails",
		Should:   "exit with code 1 (excluded failure counts toward exit code)",
		Actual:   exitCode,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "excluded test fails (no retry, no flaky detection)",
		Should:   "not write quarantine state (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled) == 0,
		Expected: true,
	})
}

