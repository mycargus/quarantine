package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/config"
	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 117: quarantine run executes suite command unmodified ---

func TestRunExecutesSuiteCommandUnmodified(t *testing.T) {
	dir := t.TempDir()

	// Create a fake binary that the suite command will point to.
	// The script writes a sentinel file so we can verify it was actually executed.
	sentinelPath := filepath.Join(dir, "executed.txt")
	xmlPath := filepath.Join(dir, "rspec.xml")

	rspecXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="2" skipped="0" failures="0" errors="0"
           time="0.150" timestamp="2026-04-12T10:00:00+00:00">
  <testcase classname="spec.models.user_spec"
            name="User#valid? returns true for valid attributes"
            file="./spec/models/user_spec.rb"
            time="0.080">
  </testcase>
  <testcase classname="spec.models.user_spec"
            name="User#valid? requires an email"
            file="./spec/models/user_spec.rb"
            time="0.070">
  </testcase>
</testsuite>`

	// The suite command will be our fake binary.
	fakeBin := filepath.Join(dir, "fake-rspec")
	script := "#!/bin/sh\n" +
		"touch " + sentinelPath + "\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	// Pre-write the JUnit XML (simulates rspec having written it).
	if err := os.WriteFile(xmlPath, []byte(rspecXML), 0644); err != nil {
		t.Fatalf("write rspec.xml: %v", err)
	}

	// Create .quarantine/config.yml with a backend suite.
	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
test_suites:
  - name: backend
    command: ["` + fakeBin + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with a backend suite and all tests pass",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend with the suite command pointing to fake-rspec",
		Should:   "execute the suite command (sentinel file exists)",
		Actual:   func() bool { _, statErr := os.Stat(sentinelPath); return statErr == nil }(),
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all suite tests pass",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		jsonErr := json.Unmarshal(resultsData, &results)
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "all suite tests pass",
			Should:   "write valid JSON to results.json",
			Actual:   jsonErr == nil,
			Expected: true,
		})

		if jsonErr == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "rspec.xml with 2 passing tests",
				Should:   "report total of 2 tests",
				Actual:   summary["total"].(float64),
				Expected: 2,
			})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "rspec.xml with 0 failures",
				Should:   "report 0 failed",
				Actual:   summary["failed"].(float64),
				Expected: 0,
			})
		}
	}
}

// --- Scenario 117 (variant): missing suite name exits 2 ---

func TestRunSuiteMissingNameExitstwo(t *testing.T) {
	dir := t.TempDir()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a suite config with no suite name argument",
		Should:   "exit with code 2 (usage error)",
		Actual:   exitCode,
		Expected: 2,
	})
}

// --- Scenario 117 (variant): unknown suite name exits 2 ---

func TestRunSuiteUnknownNameExitsTwo(t *testing.T) {
	dir := t.TempDir()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"frontend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a suite config with an unknown suite name 'frontend'",
		Should:   "exit with code 2 (usage error)",
		Actual:   exitCode,
		Expected: 2,
	})
}

// --- Scenario 117 (variant): -- separator is rejected in suite mode ---

func TestRunRejectsDashDashSeparatorInSuiteMode(t *testing.T) {
	dir := t.TempDir()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec", "--format", "progress"]
    junitxml: "rspec.xml"
    retries: 3
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--", "bundle", "exec", "rspec",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite config and -- separator in args",
		Should:   "exit with a non-zero error (code 2)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite config and -- separator in args",
		Should:   "print an error mentioning the separator is not used with suite configs",
		Actual:   strings.Contains(output, "separator") && strings.Contains(output, "suite"),
		Expected: true,
	})
}

// --- findSuite pure function tests ---

func TestFindSuiteReturnsMatchingSuite(t *testing.T) {
	suites := []config.TestSuite{
		{Name: "frontend", JUnitXML: "frontend.xml"},
		{Name: "backend", JUnitXML: "backend.xml"},
	}

	suite, err := findSuite(suites, "backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a list with 'frontend' and 'backend' suites",
		Should:   "return nil error when suite name matches",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a list with 'frontend' and 'backend' suites",
		Should:   "return the backend suite",
		Actual:   suite.JUnitXML,
		Expected: "backend.xml",
	})
}

func TestFindSuiteReturnsErrorWhenNotFound(t *testing.T) {
	suites := []config.TestSuite{
		{Name: "frontend", JUnitXML: "frontend.xml"},
	}

	_, err := findSuite(suites, "backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a list with only 'frontend' suite",
		Should:   "return an error when 'backend' is not found",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a list with only 'frontend' suite",
		Should:   "error message mentions suite name 'backend'",
		Actual:   strings.Contains(err.Error(), "backend"),
		Expected: true,
	})
}

func TestFindSuiteReturnsErrorOnEmptyList(t *testing.T) {
	_, err := findSuite(nil, "backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty suite list",
		Should:   "return an error when looking up 'backend'",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- suitesString pure function tests ---

func TestSuitesStringEmpty(t *testing.T) {
	result := suitesString(nil)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "nil suite slice",
		Should:   "return '(none)'",
		Actual:   result,
		Expected: "(none)",
	})
}

func TestSuitesStringEmptySlice(t *testing.T) {
	result := suitesString([]config.TestSuite{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "empty suite slice",
		Should:   "return '(none)'",
		Actual:   result,
		Expected: "(none)",
	})
}

func TestSuitesStringMultipleSuites(t *testing.T) {
	suites := []config.TestSuite{
		{Name: "backend"},
		{Name: "frontend"},
	}

	result := suitesString(suites)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suites named 'backend' and 'frontend'",
		Should:   "contain both names",
		Actual:   strings.Contains(result, "backend") && strings.Contains(result, "frontend"),
		Expected: true,
	})
}
