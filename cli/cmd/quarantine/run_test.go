package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mycargus/quarantine/internal/config"
	"github.com/mycargus/quarantine/internal/parser"
	riteway "github.com/mycargus/riteway-golang"
)

// executeRunCmd is a test helper that runs the run command with given args,
// capturing output and returning the exit error.
func executeRunCmd(t *testing.T, args []string, env map[string]string) (output string, exitErr error) {
	t.Helper()

	for k, v := range env {
		t.Setenv(k, v)
	}

	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs(append([]string{"run"}, args...))
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// fakeGitHubAPI returns an httptest server that responds to the branch check
// endpoint. If branchExists is true, it returns 200; otherwise 404.
func fakeGitHubAPI(t *testing.T, branchExists bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			if branchExists {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// writeTestScript creates an executable shell script that exits with exitCode.
// It also pre-writes xmlContent to xmlPath before returning so that the
// CLI can parse it after the script runs.
func writeTestScript(t *testing.T, dir, xmlPath, xmlContent string, exitCode int) string {
	t.Helper()
	// Write the XML file directly — simulates the test runner having written it.
	if xmlContent != "" && xmlPath != "" {
		if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
			t.Fatalf("writeTestScript: write xml: %v", err)
		}
	}
	scriptPath := filepath.Join(dir, "fake-test")
	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("writeTestScript: %v", err)
	}
	return scriptPath
}

// --- Scenario 12: quarantine run without prior init ---

func TestRunWithoutPriorInit(t *testing.T) {
	dir := t.TempDir()
	output, err := executeRunCmd(t, []string{"--config", dir + "/quarantine.yml", "--", "jest", "--ci"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "print message to run quarantine init first",
		Actual:   strings.Contains(output, "Run 'quarantine init' first"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "not contain test runner output (test was not executed)",
		Actual:   strings.Contains(output, "jest"),
		Expected: false,
	})
}

// --- Scenario 52: quarantine run without -- separator ---

func TestRunWithoutSeparator(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{"--config", configPath, "jest", "--ci"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "print error about missing separator",
		Actual:   strings.Contains(output, "missing '--' separator"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "print usage example",
		Actual:   strings.Contains(output, "quarantine run [flags] -- <test command>"),
		Expected: true,
	})
}

// --- Scenario 52 (variant): -- separator present but no command after it ---

func TestRunEmptyArgsAfterSeparator(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{"--config", configPath, "--"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "-- separator present but no command after it",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "-- separator present but no command after it",
		Should:   "print error about missing test command",
		Actual:   strings.Contains(output, "missing '--' separator"),
		Expected: true,
	})
}

// --- Scenario 53: Test command not found ---

func TestRunCommandNotFound(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)
	args := []string{"--config", configPath, "--", "jset", "--ci"}
	env := map[string]string{"QUARANTINE_GITHUB_TOKEN": "ghp_test"}

	exitCode := executeRunCmdWithExitCode(t, args, env)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a typo in the test command (jset instead of jest)",
		Should:   "exit with quarantine error code (2)",
		Actual:   exitCode,
		Expected: 2,
	})

	output, _ := executeRunCmd(t, args, env)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a typo in the test command (jset instead of jest)",
		Should:   "print command not found error with the command name",
		Actual:   strings.Contains(output, `command not found: "jset"`),
		Expected: true,
	})
}

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
		"QUARANTINE_GITHUB_TOKEN":          "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL":   server.URL,
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

// --- Scenario 70: Signal forwarding to child process ---

func TestRunSignalForwardingSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("signal test skipped in short mode")
	}

	dir := t.TempDir()
	// Script: sleep briefly then exit — so it's interruptible.
	// We'll send SIGTERM to the CLI process and verify the child exits cleanly.
	scriptPath := filepath.Join(dir, "slow-test")
	// Script that writes its PID to a file, then sleeps.
	pidFile := filepath.Join(dir, "child.pid")
	script := fmt.Sprintf(`#!/bin/sh
echo $$ > %q
sleep 30
exit 0
`, pidFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	// Run the command in a subprocess so we can send signals to it.
	binary := buildTestBinary(t)
	cmd := exec.Command(binary,
		"run",
		"--config", configPath,
		"--junitxml", filepath.Join(dir, "junit.xml"),
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	)
	cmd.Env = append(os.Environ(),
		"QUARANTINE_GITHUB_TOKEN=ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL="+server.URL,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for child script to write its PID.
	var childPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil && len(data) > 0 {
			_, _ = fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &childPID)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "child process started",
		Should:   "write PID file",
		Actual:   childPID > 0,
		Expected: true,
	})

	// Send SIGTERM to the CLI process.
	_ = cmd.Process.Signal(syscall.SIGTERM)
	waitErr := cmd.Wait()

	// The CLI should have exited — check exit code is 143 (128+SIGTERM) or similar.
	exitCode := 0
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SIGTERM sent to CLI process",
		Should:   "exit with non-zero code (signal-based exit)",
		Actual:   exitCode != 0,
		Expected: true,
	})

	// Child process should no longer be running.
	if childPID > 0 {
		childRunning := isProcessRunning(childPID)
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "SIGTERM forwarded to child",
			Should:   "child process should have exited",
			Actual:   childRunning,
			Expected: false,
		})
	}
}

// buildTestBinary compiles the CLI to a temp binary for subprocess testing.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "quarantine-test")
	out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

// isProcessRunning returns true if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

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

// executeRunCmdWithExitCode runs the run command and returns the numeric exit
// code. Returns 0 for no error, 1 for exitCodeError(1), 2 for other errors.
func executeRunCmdWithExitCode(t *testing.T, args []string, env map[string]string) int {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"run"}, args...))
	err := rootCmd.Execute()
	if err == nil {
		return 0
	}
	if code, ok := err.(exitCodeError); ok {
		return int(code)
	}
	return 2
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

// --- Pure function unit tests ---

func TestConfigResolutionTrace(t *testing.T) {
	cfg := &config.Config{
		Framework: "jest",
		Retries:   3,
		JUnitXML:  "junit.xml",
	}

	lines := configResolutionTrace(cfg, 0, 0, "", "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "jest config with all values at defaults",
		Should:   "return 4 trace lines",
		Actual:   len(lines),
		Expected: 4,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "jest config with all values at defaults",
		Should:   "include framework with config source",
		Actual:   lines[1],
		Expected: "[verbose]   framework = jest (source: config)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries not set in config file or flag",
		Should:   "report retries as default source",
		Actual:   lines[2],
		Expected: "[verbose]   retries   = 3 (source: default)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml not set in config file or flag",
		Should:   "report junitxml as default source",
		Actual:   lines[3],
		Expected: "[verbose]   junitxml  = junit.xml (source: default)",
	})
}

func TestConfigResolutionTraceSourceAttribution(t *testing.T) {
	cfg := &config.Config{
		Framework: "rspec",
		Retries:   5,
		JUnitXML:  "override.xml",
	}

	// Both values came from the CLI flag.
	linesFromFlag := configResolutionTrace(cfg, 5, 0, "override.xml", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries set via --retries flag",
		Should:   "report retries as flag source",
		Actual:   linesFromFlag[2],
		Expected: "[verbose]   retries   = 5 (source: flag)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set via --junitxml flag",
		Should:   "report junitxml as flag source",
		Actual:   linesFromFlag[3],
		Expected: "[verbose]   junitxml  = override.xml (source: flag)",
	})

	// Both values came from the config file (not overridden by flag).
	linesFromConfig := configResolutionTrace(cfg, 0, 5, "", "override.xml")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries set in config file (not flag)",
		Should:   "report retries as config source",
		Actual:   linesFromConfig[2],
		Expected: "[verbose]   retries   = 5 (source: config)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set in config file (not flag)",
		Should:   "report junitxml as config source",
		Actual:   linesFromConfig[3],
		Expected: "[verbose]   junitxml  = override.xml (source: config)",
	})
}

func TestMergeParseResults(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}
	r2 := []parser.TestResult{{TestID: "b", Status: "failed"}}

	merged, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{results: r2},
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "two successful parse attempts",
		Should:   "return all results merged",
		Actual:   len(merged),
		Expected: 2,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "two successful parse attempts",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})

	merged, warnings = mergeParseResults([]parseAttempt{
		{results: r1},
		{warning: "Failed to parse shard-2.xml: unexpected EOF. Skipping."},
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "1 successful and 1 failed parse attempt",
		Should:   "return results from the successful file only",
		Actual:   len(merged),
		Expected: 1,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "1 successful and 1 failed parse attempt",
		Should:   "return file-level warning plus summary warning",
		Actual:   len(warnings),
		Expected: 2,
	})

	merged, warnings = mergeParseResults([]parseAttempt{
		{warning: "Failed to parse a.xml: unexpected EOF. Skipping."},
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all parse attempts failed",
		Should:   "return nil results",
		Actual:   merged == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all parse attempts failed",
		Should:   "return warnings",
		Actual:   len(warnings) > 0,
		Expected: true,
	})

	merged, warnings = mergeParseResults([]parseAttempt{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty attempts slice",
		Should:   "return empty results",
		Actual:   len(merged),
		Expected: 0,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty attempts slice",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})
}

func TestRepoString(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner and repo both present",
		Should:   "return owner/repo",
		Actual:   repoString("acme", "api"),
		Expected: "acme/api",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner is empty",
		Should:   "return empty string",
		Actual:   repoString("", "api"),
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "repo is empty",
		Should:   "return empty string",
		Actual:   repoString("acme", ""),
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "both owner and repo are empty",
		Should:   "return empty string",
		Actual:   repoString("", ""),
		Expected: "",
	})
}
