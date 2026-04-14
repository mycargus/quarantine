package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// executeSuiteRemoveCmd is a test helper that runs "quarantine suite remove <name>"
// in the current working directory, with the given stdin content, and returns
// stdout and the exit error.
func executeSuiteRemoveCmd(t *testing.T, name string, stdin string) (stdout string, exitErr error) {
	t.Helper()

	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(stdin))

	rootCmd.SetArgs([]string{"suite", "remove", name})
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// --- Scenario 127: quarantine suite remove ---

// TestSuiteRemoveConfirmedRemovesSuiteFromConfig verifies that when the user
// confirms with "y", the named suite is removed from config and the other
// suite is preserved.
func TestSuiteRemoveConfirmedRemovesSuiteFromConfig(t *testing.T) {
	dir := writeTempConfig(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
`)

	stdout, err := executeSuiteRemoveCmd(t, "backend", "y\n")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with backend and frontend suites and user confirms with 'y'",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with backend and frontend suites and user confirms with 'y'",
		Should:   "print confirmation message",
		Actual:   strings.Contains(stdout, "Suite 'backend' removed from .quarantine/config.yml."),
		Expected: true,
	})

	// Read the written config and verify backend is gone but frontend remains.
	configBytes, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	if readErr != nil {
		t.Fatalf("could not read config after removal: %v", readErr)
	}
	configContents := string(configBytes)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite remove confirmed with 'y'",
		Should:   "remove 'backend' entry from config file",
		Actual:   strings.Contains(configContents, "backend"),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite remove confirmed with 'y'",
		Should:   "preserve 'frontend' entry in config file",
		Actual:   strings.Contains(configContents, "frontend"),
		Expected: true,
	})
}

// TestSuiteRemoveAbortedLeavesConfigUnchanged verifies that when the user types
// "n", no changes are made to the config file.
func TestSuiteRemoveAbortedLeavesConfigUnchanged(t *testing.T) {
	originalContent := `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
`
	dir := writeTempConfig(t, originalContent)

	stdout, err := executeSuiteRemoveCmd(t, "backend", "n\n")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with backend and frontend suites and user types 'n'",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with backend and frontend suites and user types 'n'",
		Should:   "print abort message",
		Actual:   strings.Contains(stdout, "Aborted. No changes made."),
		Expected: true,
	})

	configBytes, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	if readErr != nil {
		t.Fatalf("could not read config after abort: %v", readErr)
	}
	configContents := string(configBytes)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite remove aborted with 'n'",
		Should:   "still contain 'backend' in config file",
		Actual:   strings.Contains(configContents, "backend"),
		Expected: true,
	})
}

// --- Pure function tests ---

// TestSuiteRemoveRamificationMessage verifies the pure function that builds
// the ramification message for suite remove.
func TestSuiteRemoveRamificationMessage(t *testing.T) {
	result := suiteRemoveRamificationMessage("backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "contain the suite name in the header",
		Actual:   strings.Contains(result, "Removing suite 'backend':"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "mention removing the entry from config.yml",
		Actual:   strings.Contains(result, ".quarantine/config.yml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "mention the state file path",
		Actual:   strings.Contains(result, ".quarantine/backend/state.json"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "convey that quarantined tests remain quarantined",
		Actual:   strings.Contains(result, "quarantined tests remain quarantined"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "mention GitHub issues remaining open",
		Actual:   strings.Contains(result, "GitHub issues"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "warn about updating CI workflow",
		Actual:   strings.Contains(result, "your CI workflow"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name 'backend'",
		Should:   "end with the confirmation prompt",
		Actual:   strings.Contains(result, "Are you sure? [y/N]"),
		Expected: true,
	})
}

// TestRemoveSuiteFromSlice verifies the pure function that removes a suite
// entry by name from a slice of TestSuites.
func TestRemoveSuiteFromSlice(t *testing.T) {
	suites := []suiteNameEntry{
		{name: "backend"},
		{name: "frontend"},
		{name: "e2e"},
	}

	result := removeSuiteByName(suites, "backend")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "three suites and removing 'backend'",
		Should:   "return two suites",
		Actual:   len(result),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three suites and removing 'backend'",
		Should:   "first remaining suite should be 'frontend'",
		Actual:   result[0].name,
		Expected: "frontend",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three suites and removing 'backend'",
		Should:   "second remaining suite should be 'e2e'",
		Actual:   result[1].name,
		Expected: "e2e",
	})
}

func TestRemoveSuiteFromSliceRemovesOnlyElement(t *testing.T) {
	result := removeSuiteByName([]suiteNameEntry{{name: "only"}}, "only")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one suite and removing it",
		Should:   "return empty slice",
		Actual:   len(result),
		Expected: 0,
	})
}

func TestRemoveSuiteFromSliceNameNotFound(t *testing.T) {
	suites := []suiteNameEntry{{name: "backend"}, {name: "frontend"}}

	result := removeSuiteByName(suites, "nonexistent")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two suites and removing a name that does not exist",
		Should:   "return all suites unchanged",
		Actual:   len(result),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "two suites and removing a name that does not exist",
		Should:   "first suite is unchanged",
		Actual:   result[0].name,
		Expected: "backend",
	})
}

// --- Error path: suite not found in config ---

func TestSuiteRemoveSuiteNotFoundErrors(t *testing.T) {
	writeTempConfig(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
`)

	_, err := executeSuiteRemoveCmd(t, "nonexistent", "y\n")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with 'backend' suite and removing 'nonexistent'",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Error path: missing config ---

func TestSuiteRemoveErrorsWhenConfigMissing(t *testing.T) {
	chdirTest(t, t.TempDir())

	_, err := executeSuiteRemoveCmd(t, "backend", "y\n")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .quarantine/config.yml in working directory",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}
