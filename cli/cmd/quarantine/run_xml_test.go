package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 54: No JUnit XML produced (crash detection in suite mode) ---

func TestRunNoXMLProduced(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	// Script exits non-zero and writes no XML — crash detected.
	scriptPath := writeTestScript(t, dir, "", "", 1)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	// In suite mode, non-zero exit with no XML = crash → exit 2.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "test runner exits non-zero with no JUnit XML produced (suite crash detection)",
		Should:   "exit with code 2 (crash detected)",
		Actual:   exitCode,
		Expected: 2,
	})
}

// --- Scenario 55: Malformed JUnit XML ---

func TestRunMalformedXML(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	// Write truncated/invalid XML.
	if err := os.WriteFile(xmlPath, []byte(`<?xml version="1.0"?><testsuites`), 0644); err != nil {
		t.Fatal(err)
	}
	// Script exits 0 (so crash detection doesn't fire).
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed JUnit XML with exit 0",
		Should:   "log a warning that references the file name",
		Actual:   strings.Contains(output, "WARNING") && strings.Contains(output, "junit.xml"),
		Expected: true,
	})

	_ = err
}

// --- Scenario 56: Multiple XML files, some malformed ---

func TestRunMultipleXMLSomeMalformed(t *testing.T) {
	dir := t.TempDir()

	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2" failures="0">
  <testsuite name="shard-1.test.js" tests="2">
    <testcase classname="Suite" name="test 1" file="shard-1.test.js" time="0.01"></testcase>
    <testcase classname="Suite" name="test 2" file="shard-1.test.js" time="0.01"></testcase>
  </testsuite>
</testsuites>`

	if err := os.WriteFile(filepath.Join(dir, "shard-1.xml"), []byte(validXML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shard-2.xml"), []byte(validXML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shard-3.xml"), []byte(`<?xml truncated`), 0644); err != nil {
		t.Fatal(err)
	}

	xmlGlob := filepath.Join(dir, "shard-*.xml")
	scriptPath := writeTestScript(t, dir, "", "", 0)

	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "` + xmlGlob + `"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, ".quarantine", "unit", "results.json")
	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "3 XML files with 1 malformed",
		Should:   "exit without error (valid results available)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "3 XML files with 1 malformed",
		Should:   "log a warning about the malformed file",
		Actual:   strings.Contains(output, "WARNING"),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "2 valid XML files each with 2 tests",
				Should:   "merge results from valid files (total 4 tests)",
				Actual:   summary["total"].(float64),
				Expected: 4,
			})
		}
	}
}

// --- Invalid JUnit XML glob pattern ---

func TestRunInvalidJUnitXMLGlob(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeTestScript(t, dir, "", "", 0)

	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "[bad"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, _ := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid glob pattern '[bad'",
		Should:   "print a WARNING about the invalid glob",
		Actual:   strings.Contains(output, "WARNING"),
		Expected: true,
	})
}

// --- Scenario 94: No JUnit XML produced and test runner exited 0 ---

func TestRunNoXMLProducedExitZero(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	// Script exits 0 and writes no XML.
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, exitErr := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test runner exits 0 with no JUnit XML produced",
		Should:   "exit without error (exit code 0)",
		Actual:   exitErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test runner exits 0 with no JUnit XML produced",
		Should:   "log a warning about missing XML",
		Actual:   strings.Contains(output, "WARNING") && strings.Contains(output, "junit.xml"),
		Expected: true,
	})
}
