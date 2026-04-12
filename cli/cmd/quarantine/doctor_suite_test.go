package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// writeTempSuiteConfig writes a .quarantine/config.yml to a temp dir, changes to
// that dir, and returns the dir path.
func writeTempSuiteConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	suiteDir := dir + "/.quarantine"
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("writeTempSuiteConfig mkdir: %v", err)
	}
	path := suiteDir + "/config.yml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempSuiteConfig write: %v", err)
	}
	chdirTest(t, dir)
	return dir
}

// --- Scenario 114: quarantine doctor validates test_suites array and rejects invalid configs ---

func TestDoctorRejectsInvalidSuiteConfig(t *testing.T) {
	writeTempSuiteConfig(t, `
version: 1
test_suites:
  - name: "My Backend!"
    command: "bundle exec rspec"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
`)

	stdout, err := executeDoctorCmd(t, nil, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with an invalid suite entry",
		Should:   "return an error (exit 2)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with an invalid name containing uppercase letters and special chars",
		Should:   "report name validation error",
		Actual:   strings.Contains(stdout, `suite "My Backend!": name must match [a-z0-9][a-z0-9-]* (max 30 chars)`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with command as a string instead of a YAML array",
		Should:   "report command-is-string error",
		Actual:   strings.Contains(stdout, `suite "My Backend!": command must be a YAML array, not a string. Use: command: ["bundle", "exec", "rspec"]`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite missing the junitxml field",
		Should:   "report junitxml is required",
		Actual:   strings.Contains(stdout, `suite "My Backend!": junitxml is required`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with a valid rerun_command array",
		Should:   "not report any errors about rerun_command",
		Actual:   !strings.Contains(stdout, "rerun_command"),
		Expected: true,
	})
}

// --- Scenario 115: quarantine doctor warns on detected jest retryTimes but does not error ---

func TestDoctorWarnsOnJestRetryTimes(t *testing.T) {
	dir := writeTempSuiteConfig(t, `
version: 1
`)
	// Write jest.config.js with retryTimes: 2 in the repo root.
	jestConfigPath := filepath.Join(dir, "jest.config.js")
	if err := os.WriteFile(jestConfigPath, []byte("module.exports = {\n  retryTimes: 2,\n};\n"), 0644); err != nil {
		t.Fatalf("write jest.config.js: %v", err)
	}

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid .quarantine/config.yml and jest.config.js containing retryTimes: 2",
		Should:   "exit without error (warning does not make config invalid)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest.config.js with retryTimes: 2",
		Should:   "print a warning containing 'Warning' and 'retryTimes'",
		Actual:   strings.Contains(stdout, "Warning") && strings.Contains(stdout, "retryTimes"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest.config.js with retryTimes: 2",
		Should:   "print the full warning message about framework-level retries hiding failures",
		Actual:   strings.Contains(stdout, "Framework-level retries hide"),
		Expected: true,
	})
}

// --- Scenario 116: quarantine doctor does not warn when retryTimes is set to 0 ---

func TestDoctorNoWarnOnRetryTimesZero(t *testing.T) {
	dir := writeTempSuiteConfig(t, `
version: 1
`)
	// Write a test file with jest.retryTimes(0) — explicit no-op that disables retries.
	testFilePath := filepath.Join(dir, "example.test.js")
	if err := os.WriteFile(testFilePath, []byte("jest.retryTimes(0);\n"), 0644); err != nil {
		t.Fatalf("write example.test.js: %v", err)
	}

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config and a test file containing jest.retryTimes(0)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a test file with jest.retryTimes(0) (zero-value is a no-op)",
		Should:   "not print any retryTimes warning",
		Actual:   !strings.Contains(stdout, "retryTimes"),
		Expected: true,
	})
}
