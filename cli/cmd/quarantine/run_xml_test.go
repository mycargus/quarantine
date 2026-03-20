package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 54: No JUnit XML produced ---

func TestRunNoXMLProduced(t *testing.T) {
	dir := t.TempDir()
	// Script exits non-zero and writes no XML.
	scriptPath := writeTestScript(t, dir, "", "", 1)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", filepath.Join(dir, "junit.xml"),
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "test runner exits non-zero with no JUnit XML produced",
		Should:   "exit with the runner's exit code (1)",
		Actual:   exitCode,
		Expected: 1,
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
	scriptPath := writeTestScript(t, dir, "", "", 1)

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
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed JUnit XML",
		Should:   "log a warning that references the file name",
		Actual:   strings.Contains(output, "WARNING") && strings.Contains(output, "junit.xml"),
		Expected: true,
	})

	// exitCode should be runner's code (1), not 2
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed JUnit XML",
		Should:   "return an error (non-zero exit code)",
		Actual:   err != nil,
		Expected: true,
	})
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

	// Write 2 valid XML files and 1 malformed.
	if err := os.WriteFile(filepath.Join(dir, "shard-1.xml"), []byte(validXML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shard-2.xml"), []byte(validXML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shard-3.xml"), []byte(`<?xml truncated`), 0644); err != nil {
		t.Fatal(err)
	}

	scriptPath := writeTestScript(t, dir, "", "", 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", filepath.Join(dir, "shard-*.xml"),
		"--output", resultsPath,
		"--", scriptPath,
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

// --- 2D: parseJUnitXML with invalid glob pattern ---

func TestRunInvalidJUnitXMLGlob(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeTestScript(t, dir, "", "", 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", "[bad",
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
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

// --- 2E: empty XML with non-zero exit code ---

func TestRunEmptyXMLNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	// Valid but empty XML: parser returns non-nil empty slice.
	emptyXML := `<testsuites tests="0"></testsuites>`
	scriptPath := writeTestScript(t, dir, xmlPath, emptyXML, 2)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty XML and test script exits with code 2",
		Should:   "propagate runner's exit code (2)",
		Actual:   exitCode,
		Expected: 2,
	})
}
