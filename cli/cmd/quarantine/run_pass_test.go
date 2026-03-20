package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 19: Normal CI run with Jest — all tests pass ---

func TestRunJestAllPass(t *testing.T) {
	dir := t.TempDir()

	// Set up JUnit XML that the fake test command will "produce".
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="3" failures="0" errors="0" time="1.234">
  <testsuite name="__tests__/auth/login.test.js" tests="3" errors="0" failures="0" skipped="0"
             timestamp="2026-03-15T14:22:10" time="1.234">
    <testcase classname="LoginForm validates input" name="should reject empty email"
              file="__tests__/auth/login.test.js" time="0.045">
    </testcase>
    <testcase classname="LoginForm validates input" name="should reject invalid email format"
              file="__tests__/auth/login.test.js" time="0.032">
    </testcase>
    <testcase classname="LoginForm submits" name="should call onSubmit with credentials"
              file="__tests__/auth/login.test.js" time="0.078">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Mock GitHub API — branch exists
	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all Jest tests pass",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	// Verify results.json was written.
	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all Jest tests pass",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		jsonErr := json.Unmarshal(resultsData, &results)

		riteway.Assert(t, riteway.Case[bool]{
			Given:    "all Jest tests pass",
			Should:   "write valid JSON to results.json",
			Actual:   jsonErr == nil,
			Expected: true,
		})

		if jsonErr == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all Jest tests pass",
				Should:   "report total of 3 tests",
				Actual:   summary["total"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all Jest tests pass",
				Should:   "report 3 passed",
				Actual:   summary["passed"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "all Jest tests pass",
				Should:   "report 0 failed",
				Actual:   summary["failed"].(float64),
				Expected: 0,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    "all Jest tests pass",
				Should:   "set framework to jest",
				Actual:   results["framework"].(string),
				Expected: "jest",
			})
		}
	}

	_ = output // output checked implicitly via err == nil
}

// --- Scenario 67: Normal CI run with RSpec — all tests pass ---

func TestRunRSpecAllPass(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="3" skipped="0" failures="0" errors="0"
           time="0.284" timestamp="2026-03-15T14:22:10+00:00">
  <testcase classname="spec.models.user_spec"
            name="User#valid? returns true for valid attributes"
            file="./spec/models/user_spec.rb"
            time="0.098">
  </testcase>
  <testcase classname="spec.models.user_spec"
            name="User#valid? requires an email address"
            file="./spec/models/user_spec.rb"
            time="0.045">
  </testcase>
  <testcase classname="spec.models.user_spec"
            name="User#full_name concatenates first and last name"
            file="./spec/models/user_spec.rb"
            time="0.031">
  </testcase>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: rspec
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all RSpec tests pass",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all RSpec tests pass",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "RSpec JUnit XML with 3 passing tests",
				Should:   "report total of 3 tests",
				Actual:   summary["total"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    "RSpec run",
				Should:   "set framework to rspec",
				Actual:   results["framework"].(string),
				Expected: "rspec",
			})
		}
	}
}

// --- Scenario 68: Normal CI run with Vitest — all tests pass ---

func TestRunVitestAllPass(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8" ?>
<testsuites name="vitest tests" tests="3" failures="0" errors="0" time="0.567">
  <testsuite name="src/utils/__tests__/math.test.ts"
             tests="3" failures="0" errors="0" skipped="0" time="0.234">
    <testcase classname="src/utils/__tests__/math.test.ts"
              name="math > add > should add positive numbers" time="0.012">
    </testcase>
    <testcase classname="src/utils/__tests__/math.test.ts"
              name="math > add > should handle negative numbers" time="0.008">
    </testcase>
    <testcase classname="src/utils/__tests__/math.test.ts"
              name="math > multiply > should multiply two numbers" time="0.006">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit-report.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: vitest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all Vitest tests pass",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all Vitest tests pass",
		Should:   "write results.json",
		Actual:   readErr == nil,
		Expected: true,
	})

	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "Vitest JUnit XML with 3 passing tests",
				Should:   "report total of 3 tests",
				Actual:   summary["total"].(float64),
				Expected: 3,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    "Vitest run",
				Should:   "set framework to vitest",
				Actual:   results["framework"].(string),
				Expected: "vitest",
			})
		}
	}
}

// --- Scenario 69: CI run with test failures — exit 1 ---

func TestRunTestFailuresExitOne(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="2" failures="1" errors="0" time="1.0">
  <testsuite name="__tests__/checkout.test.js" tests="2" failures="1" time="1.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">
        Error: expected 10 but got 0
      </failure>
    </testcase>
    <testcase classname="CheckoutService" name="should calculate total"
              file="__tests__/checkout.test.js" time="0.5">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	err := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one Jest test fails",
		Should:   "exit with code 1 (test failure)",
		Actual:   err,
		Expected: 1,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})

			riteway.Assert(t, riteway.Case[float64]{
				Given:    "one test fails",
				Should:   "report 1 failed in results.json",
				Actual:   summary["failed"].(float64),
				Expected: 1,
			})
		}
	}
}

// --- Scenario 43: Framework override from config ---

func TestRunFrameworkFromConfig(t *testing.T) {
	dir := t.TempDir()

	// Vitest XML — using suite name as file path (Vitest format).
	vitestXML := `<?xml version="1.0" encoding="UTF-8" ?>
<testsuites name="vitest tests" tests="2" failures="0">
  <testsuite name="src/calc.test.ts" tests="2">
    <testcase classname="src/calc.test.ts" name="calc > adds" time="0.010"></testcase>
    <testcase classname="src/calc.test.ts" name="calc > subtracts" time="0.010"></testcase>
  </testsuite>
</testsuites>`

	// Config explicitly sets vitest framework — this should be the source of truth.
	configPath := writeTempConfig(t, `
version: 1
framework: vitest
junitxml: junit-report.xml
`)

	xmlPath := filepath.Join(dir, "junit-report.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, vitestXML, 0)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml has framework: vitest",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			riteway.Assert(t, riteway.Case[string]{
				Given:    "quarantine.yml has framework: vitest",
				Should:   "use vitest as the framework in results.json",
				Actual:   results["framework"].(string),
				Expected: "vitest",
			})
		}
	}
}

// --- Scenario 51 (verbose output): branch not found logs 404 ---

func TestRunBranchNotFound(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeTestScript(t, dir, "", "", 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	server := fakeGitHubAPI(t, false) // branch does not exist
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", filepath.Join(dir, "junit.xml"),
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch does not exist (404)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch does not exist",
		Should:   "print 'not initialized' error message",
		Actual:   strings.Contains(output, "Run 'quarantine init' first"),
		Expected: true,
	})
}

// --- Scenario 51 (verbose output): verbose with no token logs skipped ---

func TestRunVerboseNoToken(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0" encoding="UTF-8"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
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
		"--verbose",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token set",
		Should:   "exit without error (degraded mode continues)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token set",
		Should:   "print WARNING about missing token",
		Actual:   strings.Contains(output, "[quarantine] WARNING:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --verbose flag",
		Should:   "print verbose API call skipped message",
		Actual:   strings.Contains(output, "[verbose] API call: skipped (no token)"),
		Expected: true,
	})
}
