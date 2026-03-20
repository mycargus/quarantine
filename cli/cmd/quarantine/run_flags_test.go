package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 64: Config resolution order ---

func TestRunJUnitXMLFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()

	// Config says junitxml: custom/*.xml, but --junitxml flag overrides it.
	overrideXMLPath := filepath.Join(dir, "override.xml")
	jestXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
	if err := os.WriteFile(overrideXMLPath, []byte(jestXML), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := writeTempConfig(t, `
version: 1
framework: jest
junitxml: custom/*.xml
`)

	scriptPath := writeTestScript(t, dir, "", "", 0)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", overrideXMLPath, // CLI flag overrides config
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--junitxml flag set, config has different junitxml",
		Should:   "exit without error (used override.xml, not custom/*.xml)",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "--junitxml flag overrides config",
				Should:   "parse override.xml (1 test)",
				Actual:   summary["total"].(float64),
				Expected: 1,
			})
		}
	}
}

// --- Scenario 51: --verbose and --quiet flags ---

func TestRunVerboseAndQuietMutuallyExclusive(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--quiet",
		"--", "echo", "hi",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose and --quiet both set",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose and --quiet both set",
		Should:   "print mutually exclusive error message",
		Actual:   strings.Contains(output, "--verbose and --quiet are mutually exclusive"),
		Expected: true,
	})
}

func TestRunQuietSuppressesInfo(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--quiet",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set with passing tests",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set",
		Should:   "not print [quarantine] Running: line",
		Actual:   strings.Contains(output, "[quarantine] Running:"),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set",
		Should:   "not print [quarantine] Results: summary",
		Actual:   strings.Contains(output, "[quarantine] Results:"),
		Expected: false,
	})
}

// --- Scenario 51 (verbose output): config resolution trace ---

func TestRunVerboseConfigResolution(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a nonexistent command so the run exits at LookPath — no GitHub API needed.
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag and nonexistent test command",
		Should:   "return an error (command not found)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print config resolution block",
		Actual:   strings.Contains(output, "[verbose] Config resolution:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set with jest framework from config",
		Should:   "include framework in resolution output",
		Actual:   strings.Contains(output, "framework = jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print total time even on error exit",
		Actual:   strings.Contains(output, "[verbose] Total time:"),
		Expected: true,
	})
}

// --- Scenario 51 (verbose output): API call details ---

func TestRunVerboseAPICall(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="0" errors="0" time="0.1">
  <testsuite name="__tests__/a.test.js" tests="1" errors="0" failures="0" skipped="0"
             timestamp="2026-03-15T14:22:10" time="0.1">
    <testcase classname="A" name="passes" file="__tests__/a.test.js" time="0.1">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--verbose",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag with all tests passing",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print API call with GET /repos/ endpoint",
		Actual:   strings.Contains(output, "[verbose] API call: GET /repos/"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set with branch existing",
		Should:   "print 200 status for branch check",
		Actual:   strings.Contains(output, "-> 200"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print config resolution",
		Actual:   strings.Contains(output, "[verbose] Config resolution:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print total time",
		Actual:   strings.Contains(output, "[verbose] Total time:"),
		Expected: true,
	})
}

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
