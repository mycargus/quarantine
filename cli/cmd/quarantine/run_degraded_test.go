package main

import (
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// fakeUnreachableAPI returns a URL for a port that is not listening.
func fakeUnreachableAPIURL(t *testing.T) string {
	t.Helper()
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	return "http://127.0.0.1:19999"
}

func suiteXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`
}

// --- Scenario 178 (supersedes Scenario 35 for no-token case): No GitHub token — fail-fast ---
//
// Per ADR-037, when neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN is set,
// `quarantine run` MUST exit 2 with an explicit error before executing the
// test command or making any GitHub API call. The original Scenario 35
// degraded-mode behavior for the no-token case is replaced by this fail-fast
// gate. Scenario 35 still applies for runtime API failures with a valid
// token (covered by TestRunDegradedAPIUnreachable below).

func TestRunDegradedNoToken(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "no GitHub token set",
		Should:   "exit 2 (fail-fast per Scenario 178 / ADR-037)",
		Actual:   exitCode,
		Expected: 2,
	})
}

// --- Scenario 30: API unreachable — degraded mode ---

func TestRunDegradedAPIUnreachable(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API is unreachable and tests pass",
		Should:   "exit with code 0 (not exit 2)",
		Actual:   exitCode,
		Expected: 0,
	})
}

// TestRunBranchCheckWarnMsgIsPrinted verifies the branch check warning is printed.
func TestRunBranchCheckWarnMsgIsPrinted(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "valid token but API unreachable (GetRef fails with connection refused)",
		Should:   "print the branch check warning 'Could not check branch'",
		Actual:   strings.Contains(output, "Could not check branch"),
		Expected: true,
	})
}

func TestRunDegradedAPIUnreachableLogsWarning(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API is unreachable",
		Should:   "log a [quarantine] WARNING about degraded mode",
		Actual:   strings.Contains(output, "[quarantine] WARNING:"),
		Expected: true,
	})
}

func TestRunDegradedAPIUnreachableGHAAnnotation(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
		"GITHUB_ACTIONS":                 "true",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API unreachable with GITHUB_ACTIONS=true",
		Should:   "emit GHA ::warning annotation to stderr",
		Actual:   strings.Contains(output, "::warning title=Quarantine Degraded Mode::"),
		Expected: true,
	})
}

// --- Scenario 33: No API + empty cache ---

func TestRunDegradedNoAPINoCache(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "API unreachable and no cached state, tests pass",
		Should:   "exit based on test results (exit 0)",
		Actual:   exitCode,
		Expected: 0,
	})
}

func TestRunDegradedNoAPINoCacheLogsMessage(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API unreachable and no cached quarantine state",
		Should:   "log warning about unable to reach GitHub API and no cached state",
		Actual:   strings.Contains(output, "Unable to reach GitHub API") && strings.Contains(output, "no cached quarantine state"),
		Expected: true,
	})
}

// --- Scenario 31: Dashboard unreachable — CLI unaffected ---

func TestRunDashboardUnreachableHasNoEffect(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, suiteXML(), 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, nil, nil)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "CLI running with normal GitHub API, dashboard not accessible",
		Should:   "exit with code 0 (dashboard has no effect on CLI)",
		Actual:   exitCode,
		Expected: 0,
	})
}
