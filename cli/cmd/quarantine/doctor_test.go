package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// executeDoctorCmd is a test helper that runs the doctor command against
// a given config file path, capturing stdout and returning the exit error.
func executeDoctorCmd(t *testing.T, args []string, env map[string]string) (stdout string, exitErr error) {
	t.Helper()

	// Set env vars.
	for k, v := range env {
		t.Setenv(k, v)
	}

	// Build root command and capture output.
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs(append([]string{"doctor"}, args...))
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// writeTempConfig writes a quarantine.yml to a temp dir and returns the path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/quarantine.yml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempConfig: %v", err)
	}
	return path
}

// --- Scenario 12: quarantine doctor — valid configuration ---

func TestDoctorValidConfig(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
retries: 3
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid quarantine.yml",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid quarantine.yml",
		Should:   "print 'quarantine.yml is valid.'",
		Actual:   strings.Contains(stdout, "quarantine.yml is valid."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid quarantine.yml with jest",
		Should:   "print resolved configuration with framework jest",
		Actual:   strings.Contains(stdout, "framework:") && strings.Contains(stdout, "jest"),
		Expected: true,
	})
}

// --- Scenario 13: quarantine doctor — missing config file ---

func TestDoctorMissingConfig(t *testing.T) {
	stdout, err := executeDoctorCmd(t, []string{"--config", "/nonexistent/quarantine.yml"}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml in current directory",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml in current directory",
		Should:   "print error about missing config",
		Actual:   strings.Contains(stdout, "quarantine.yml not found"),
		Expected: true,
	})
}

// --- Scenario 14: quarantine doctor — invalid field values ---

func TestDoctorInvalidRetries(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
retries: -1
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: -1",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: -1",
		Should:   "print error about invalid retries",
		Actual:   strings.Contains(stdout, "Invalid retries value: -1"),
		Expected: true,
	})
}

// --- Scenario 15: quarantine doctor — forward-compatible config value ---

func TestDoctorForwardCompatibleIssueTracker(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
issue_tracker: jira
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with issue_tracker: jira",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with issue_tracker: jira",
		Should:   "print error about unsupported issue_tracker",
		Actual:   strings.Contains(stdout, "Unsupported issue_tracker 'jira'"),
		Expected: true,
	})
}

// --- Scenario 16: quarantine doctor — unknown fields ---

func TestDoctorUnknownFields(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
custom_field: something
notifications:
  github_pr_comment: true
  slack: true
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with custom_field and notifications.slack",
		Should:   "return an error (due to unknown notification channel)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with unknown top-level key",
		Should:   "print warning about unknown field 'custom_field'",
		Actual:   strings.Contains(stdout, "Unknown field 'custom_field'"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with notifications.slack",
		Should:   "print error about unknown notification channel 'slack'",
		Actual:   strings.Contains(stdout, "Unknown notification channel 'slack'"),
		Expected: true,
	})
}

// --- Scenario 17: quarantine doctor — custom config path ---

func TestDoctorCustomConfigPath(t *testing.T) {
	dir := t.TempDir()
	customPath := dir + "/config/quarantine.yml"
	if err := os.MkdirAll(dir+"/config", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(customPath, []byte(`
version: 1
framework: jest
`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	stdout, err := executeDoctorCmd(t, []string{"--config", customPath}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config at a custom path",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config at a custom path",
		Should:   "print 'quarantine.yml is valid.'",
		Actual:   strings.Contains(stdout, "quarantine.yml is valid."),
		Expected: true,
	})
}

// --- Scenario 18: Minimal valid config ---

func TestDoctorMinimalValidConfig(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a minimal quarantine.yml (version + framework only)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a minimal quarantine.yml with jest",
		Should:   "print default retries",
		Actual:   strings.Contains(stdout, "retries:") && strings.Contains(stdout, "3"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a minimal quarantine.yml with jest",
		Should:   "print default junitxml (junit.xml for jest)",
		Actual:   strings.Contains(stdout, "junit.xml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a minimal quarantine.yml",
		Should:   "print default storage branch",
		Actual:   strings.Contains(stdout, "quarantine/state"),
		Expected: true,
	})
}

// --- Scenario 19: Unsupported config version ---

func TestDoctorUnsupportedVersion(t *testing.T) {
	path := writeTempConfig(t, `
version: 2
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with version: 2",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with version: 2",
		Should:   "print error about unsupported version",
		Actual:   strings.Contains(stdout, "Unsupported config version: 2"),
		Expected: true,
	})
}

// --- prComment lines 74–75: initial false default and nil-guard ---
//
// ApplyDefaults always sets GitHubPRComment to &true when it is nil, so the
// nil branch on line 75 is never taken in practice. The tests below cover the
// two observable states: explicit false and explicit true.

func TestDoctorPRCommentExplicitFalseIsReflected(t *testing.T) {
	// When github_pr_comment is explicitly false, the pointer is non-nil and
	// its value (false) must be printed. If line 74 were mutated to
	// `prComment := true` the if-branch on line 75–77 still overwrites with
	// the pointer value (false), so the output is still correct. This test
	// confirms the value flows through the non-nil branch unchanged.
	path := writeTempConfig(t, `
version: 1
framework: jest
notifications:
  github_pr_comment: false
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with notifications.github_pr_comment: false",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with GitHubPRComment set to false",
		Should:   "print github_pr_comment: false",
		Actual:   strings.Contains(stdout, "github_pr_comment: false"),
		Expected: true,
	})
}

func TestDoctorPRCommentNonNilEnabledValueIsUsed(t *testing.T) {
	// When GitHubPRComment is non-nil, the if-branch (line 75) must fire and
	// use *cfg.Notifications.GitHubPRComment. If line 75 were mutated to
	// `== nil`, the condition is false for a non-nil pointer, so prComment
	// stays at the line-74 initial value (false) instead of true — killing
	// the mutation.
	path := writeTempConfig(t, `
version: 1
framework: jest
notifications:
  github_pr_comment: true
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with notifications.github_pr_comment: true",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with GitHubPRComment non-nil and set to true",
		Should:   "print github_pr_comment: true",
		Actual:   strings.Contains(stdout, "github_pr_comment: true"),
		Expected: true,
	})
}

// --- Exact error message when quarantine.yml is not found (line 20) ---

func TestDoctorMissingConfigExactMessage(t *testing.T) {
	stdout, _ := executeDoctorCmd(t, []string{"--config", "/nonexistent/quarantine.yml"}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml at the given path",
		Should:   "print exact guidance message including 'quarantine init'",
		Actual:   strings.Contains(stdout, "quarantine.yml not found in the current directory.\nRun 'quarantine init' to create one."),
		Expected: true,
	})
}

// --- Exact error message when config has validation errors (line 44) ---

func TestDoctorValidationErrorExactMessage(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
retries: -1
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with a validation error",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with validation errors",
		Should:   "print exact header 'quarantine.yml has errors.'",
		Actual:   strings.Contains(stdout, "quarantine.yml has errors."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with validation errors",
		Should:   "return error with message 'quarantine.yml has errors'",
		Actual:   err.Error() == "quarantine.yml has errors",
		Expected: true,
	})
}

func TestDoctorLabelsSingleDefaultInOutput(t *testing.T) {
	// Confirms the labels field is rendered as [quarantine] in the resolved
	// configuration output (exercises the surrounding format string).
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with the default label",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with the default label",
		Should:   "print labels wrapped in brackets as [quarantine]",
		Actual:   strings.Contains(stdout, "[quarantine]"),
		Expected: true,
	})
}

// --- Scenario 12 (no token warning) ---

// --- Git remote detection failure (graceful degradation) ---

func TestDoctorGitRemoteDetectionFailure(t *testing.T) {
	// Write a valid config to a temp dir that is NOT a git repository.
	// doctor should still succeed but show empty owner/repo fields.
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config but running in a non-git directory",
		Should:   "exit without error (git detection failure is non-fatal)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config with no github.owner/repo set and git detection failed",
		Should:   "print github.owner field (empty, no auto-detected note)",
		Actual:   strings.Contains(stdout, "github.owner:"),
		Expected: true,
	})
}

func TestDoctorNoTokenWarning(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Unset any token env vars.
	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token in environment",
		Should:   "still exit 0 (doctor doesn't require a token)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token in environment",
		Should:   "print a warning about missing token",
		Actual:   strings.Contains(stdout, "No GitHub token"),
		Expected: true,
	})
}
