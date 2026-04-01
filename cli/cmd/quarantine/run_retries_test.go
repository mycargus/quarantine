package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- CLI --retries flag overrides config value ---

func TestRunRetriesFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
retries: 1
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--retries", "3",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 3 flag with config retries: 1",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			cfg, _ := results["config"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "--retries 3 flag overrides config retries: 1",
				Should:   "write retry_count 3 to results.json",
				Actual:   cfg["retry_count"].(float64),
				Expected: 3,
			})
		}
	}
}

// --- Scenario 64 (variant): --retries 0 is treated as "not set" ---

func TestRunRetriesZeroUsesDefault(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a nonexistent command to exit at LookPath — config resolution fires first.
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--retries", "0",
		"--verbose",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	// Exits at LookPath — error expected.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 0 with nonexistent command",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 0 treated as unset",
		Should:   "use default retries (3) reported as default source in verbose output",
		Actual:   strings.Contains(output, "retries   = 3") && strings.Contains(output, "source: default"),
		Expected: true,
	})
}

// --- AC7: --retries flag range validation ---

func TestRunRetriesAboveMaxExitsTwo(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--retries", "11",
		"--", "echo", "hi",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "--retries 11 (above maximum of 10)",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCode,
		Expected: 2,
	})
}

func TestRunRetriesBelowMinExitsTwo(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--retries", "-1",
		"--", "echo", "hi",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "--retries -1 (below minimum of 1)",
		Should:   "exit with code 2 (quarantine error)",
		Actual:   exitCode,
		Expected: 2,
	})
}

func TestRunRetriesOneIsValid(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--retries", "1",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "--retries 1 (minimum valid value)",
		Should:   "exit with code 0 (valid flag)",
		Actual:   exitCode,
		Expected: 0,
	})
}

func TestRunRetriesTenIsValid(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--retries", "10",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "--retries 10 (maximum valid value)",
		Should:   "exit with code 0 (valid flag)",
		Actual:   exitCode,
		Expected: 0,
	})
}

// --- resolveOwnerRepo: git detection fallback ---
// These tests verify that git remote detection fires when owner or repo is
// absent from config. They kill mutations on line 552 (|| → &&).
//
// Tests run from cli/ inside the quarantine git repo (origin: mycargus/quarantine),
// so git.ParseRemote returns "mycargus"/"quarantine" as the detected values.

// --- Mutation guard: line 78 retriesFlag > 10 ---

// TestRunRetriesTenNotRejected kills the mutation retriesFlag >= 10 on line 78.
// The upper bound check is `retriesFlag > 10` (exclusive), so 10 is a valid
// value and must NOT produce a "retries out of range" error.
// The test uses a nonexistent command so execution exits at LookPath — this is
// enough to confirm that the retries validation passed (the error is about the
// command, not the flag value).
func TestRunRetriesTenNotRejected(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--retries", "10",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 10 (the maximum valid value, boundary is > 10 exclusive)",
		Should:   "fail on command-not-found, not on retries-out-of-range",
		Actual:   err != nil && strings.Contains(err.Error(), "command not found"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 10 (maximum valid value)",
		Should:   "not print 'out of range' error",
		Actual:   strings.Contains(output, "out of range"),
		Expected: false,
	})
}

// TestRunRetriesElevenIsRejected is the complementary boundary check:
// 11 must be rejected so the test above has a meaningful contrast.
func TestRunRetriesElevenIsRejected(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--retries", "11",
		"--", "echo", "hi",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 11 (above maximum of 10)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 11 (above maximum of 10)",
		Should:   "print 'out of range' error",
		Actual:   strings.Contains(output, "out of range"),
		Expected: true,
	})
}

// TestRunGitDetectionFillsMissingRepo: config has owner but no repo.
// The || condition means detection fires to supply the repo.
// Kills mutation: `owner == "" || repo == ""` → `owner == "" || repo != ""`.
func TestRunGitDetectionFillsMissingRepo(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Config has owner but no repo — detection must supply "quarantine".
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: mycargus
`)

	// Branch-check mock returns 200 for quarantine/state.
	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has owner but no repo, running inside a git repo",
		Should:   "auto-detect the repo from git remote and succeed",
		Actual:   err == nil,
		Expected: true,
	})
}

// TestRunGitDetectionFillsMissingOwner: config has repo but no owner.
// The || condition means detection fires to supply the owner.
// Kills mutation: `owner == "" || repo == ""` → `owner != "" || repo == ""`.
func TestRunGitDetectionFillsMissingOwner(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Config has repo but no owner — detection must supply "mycargus".
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  repo: quarantine
`)

	// Branch-check mock returns 200 for quarantine/state.
	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has repo but no owner, running inside a git repo",
		Should:   "auto-detect the owner from git remote and succeed",
		Actual:   err == nil,
		Expected: true,
	})
}
