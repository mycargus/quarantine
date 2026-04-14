package main

import (
	"bytes"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// executeSuiteListCmd is a test helper that runs "quarantine suite list"
// against a pre-configured working directory, captures stdout, and returns
// the output and exit error.
func executeSuiteListCmd(t *testing.T, env map[string]string) (stdout string, exitErr error) {
	t.Helper()

	for k, v := range env {
		t.Setenv(k, v)
	}

	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs([]string{"suite", "list"})
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// --- Scenario 126: quarantine suite list prints configured suites ---

func TestSuiteListPrintsConfiguredSuites(t *testing.T) {
	writeTempConfig(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
  - name: e2e
    command: ["npx", "playwright", "test"]
    junitxml: "test-results/junit.xml"
    rerun_command: ["npx", "playwright", "test", "{file}"]
    retries: 2
`)

	stdout, err := executeSuiteListCmd(t, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with three configured suites",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with three configured suites",
		Should:   "print the SUITE column header",
		Actual:   strings.Contains(stdout, "SUITE"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with three configured suites",
		Should:   "print the COMMAND column header",
		Actual:   strings.Contains(stdout, "COMMAND"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with three configured suites",
		Should:   "print the JUNITXML column header",
		Actual:   strings.Contains(stdout, "JUNITXML"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with backend suite using 'bundle exec rspec'",
		Should:   "print backend suite name",
		Actual:   strings.Contains(stdout, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with backend suite using 'bundle exec rspec'",
		Should:   "print backend command as space-separated string",
		Actual:   strings.Contains(stdout, "bundle exec rspec"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with backend suite junitxml rspec.xml",
		Should:   "print backend junitxml path",
		Actual:   strings.Contains(stdout, "rspec.xml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with frontend suite using 'npx jest --ci'",
		Should:   "print frontend suite name",
		Actual:   strings.Contains(stdout, "frontend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with frontend suite using 'npx jest --ci'",
		Should:   "print frontend command as space-separated string",
		Actual:   strings.Contains(stdout, "npx jest --ci"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with frontend suite junitxml junit.xml",
		Should:   "print frontend junitxml path",
		Actual:   strings.Contains(stdout, "junit.xml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with e2e suite using 'npx playwright test'",
		Should:   "print e2e suite name",
		Actual:   strings.Contains(stdout, "e2e"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with e2e suite using 'npx playwright test'",
		Should:   "print e2e command as space-separated string",
		Actual:   strings.Contains(stdout, "npx playwright test"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with e2e suite junitxml test-results/junit.xml",
		Should:   "print e2e junitxml path",
		Actual:   strings.Contains(stdout, "test-results/junit.xml"),
		Expected: true,
	})
}

// --- formatSuiteRows pure function tests ---

func TestFormatSuiteRowsReturnsHeaderAndRows(t *testing.T) {
	result := formatSuiteRows([]suiteEntry{
		{Name: "backend", Command: "bundle exec rspec", JUnitXML: "rspec.xml"},
		{Name: "frontend", Command: "npx jest --ci", JUnitXML: "junit.xml"},
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries",
		Should:   "include SUITE header",
		Actual:   strings.Contains(result, "SUITE"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries",
		Should:   "include COMMAND header",
		Actual:   strings.Contains(result, "COMMAND"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries",
		Should:   "include JUNITXML header",
		Actual:   strings.Contains(result, "JUNITXML"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with name backend",
		Should:   "include backend suite name",
		Actual:   strings.Contains(result, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with name frontend",
		Should:   "include frontend suite name",
		Actual:   strings.Contains(result, "frontend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with command bundle exec rspec",
		Should:   "include backend command",
		Actual:   strings.Contains(result, "bundle exec rspec"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with command npx jest --ci",
		Should:   "include frontend command",
		Actual:   strings.Contains(result, "npx jest --ci"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with junitxml rspec.xml",
		Should:   "include backend junitxml path",
		Actual:   strings.Contains(result, "rspec.xml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suite entries with junitxml junit.xml",
		Should:   "include frontend junitxml path",
		Actual:   strings.Contains(result, "junit.xml"),
		Expected: true,
	})
}

func TestFormatSuiteRowsEmpty(t *testing.T) {
	result := formatSuiteRows(nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil suite entries",
		Should:   "still include the header line",
		Actual:   strings.Contains(result, "SUITE"),
		Expected: true,
	})
}

// --- Error path: missing config ---

func TestSuiteListErrorsWhenConfigMissing(t *testing.T) {
	// Change to an empty temp dir — no .quarantine/config.yml present.
	chdirTest(t, t.TempDir())

	_, err := executeSuiteListCmd(t, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .quarantine/config.yml in working directory",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}
