package main

import (
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// fakeUnreachableAPI returns an httptest server URL that always returns 503.
// The server is started, then closed immediately so all connections fail.
// We use a fake URL instead so no TCP connection is even attempted.
func fakeUnreachableAPIURL() string {
	// Use a port that is not listening — TCP connection refused.
	return "http://127.0.0.1:19999"
}

// --- Scenario 35: No GitHub token — degraded mode ---

func TestRunDegradedNoToken(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token set",
		Should:   "exit with code 0 when tests pass",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token set",
		Should:   "log WARNING about no GitHub token",
		Actual:   strings.Contains(output, "[quarantine] WARNING:") && strings.Contains(output, "No GitHub token"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token set",
		Should:   "mention degraded mode in warning",
		Actual:   strings.Contains(output, "degraded mode"),
		Expected: true,
	})
}

// --- Scenario 30: API unreachable — degraded mode ---

func TestRunDegradedAPIUnreachable(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API is unreachable and tests pass",
		Should:   "exit with code 0 (not exit 2)",
		Actual:   exitCode,
		Expected: 0,
	})
}

// TestRunBranchCheckWarnMsgIsPrinted verifies that check.warnMsg (from
// checkBranchExists) is printed at line 105. When the API is unreachable but
// the token is valid, GetRef fails with an API error and sets warnMsg to
// "Could not check branch '...'". This string only appears if line 105's guard
// fires; it is distinct from the loadQuarantineState warning.
// Kills mutation: `check.warnMsg != ""` → `check.warnMsg == ""`.
func TestRunBranchCheckWarnMsgIsPrinted(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
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

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
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

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
		"GITHUB_ACTIONS":                 "true",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API unreachable with GITHUB_ACTIONS=true",
		Should:   "emit GHA ::warning annotation to stderr",
		Actual:   strings.Contains(output, "::warning title=Quarantine Degraded Mode::"),
		Expected: true,
	})
}

// --- Scenario 33: No API + empty cache — run all tests without exclusions ---

func TestRunDegradedNoAPINoCache(t *testing.T) {
	dir := t.TempDir()

	// JUnit XML with 2 tests (including one that "was quarantined" — but with no
	// state available, it runs normally and passes).
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2" failures="0">
  <testsuite name="suite" tests="2" failures="0">
    <testcase classname="S" name="test one" time="0.1"/>
    <testcase classname="S" name="test two" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
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

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(),
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

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Normal GitHub API (branch exists, no quarantine state).
	server := fakeM4GitHubAPI(t, nil, nil)
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
		// Dashboard URL intentionally NOT set — it doesn't exist in CLI config.
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "CLI running with normal GitHub API, dashboard not accessible",
		Should:   "exit with code 0 (dashboard has no effect on CLI)",
		Actual:   exitCode,
		Expected: 0,
	})
}
