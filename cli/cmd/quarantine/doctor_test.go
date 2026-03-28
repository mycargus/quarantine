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

// --- resolveDisplayOwnerRepo unit tests (lines 98–111) ---

func TestResolveDisplayOwnerRepo(t *testing.T) {
	tests := []struct {
		name          string
		cfgOwner      string
		cfgRepo       string
		detectedOwner string
		detectedRepo  string
		wantOwner     string
		wantRepo      string
		wantOwnerNote string
		wantRepoNote  string
	}{
		{
			name:          "config owner set uses config value with no note",
			cfgOwner:      "myorg",
			cfgRepo:       "",
			detectedOwner: "detected-org",
			detectedRepo:  "",
			wantOwner:     "myorg",
			wantOwnerNote: "",
			wantRepo:      "",
			wantRepoNote:  "",
		},
		{
			name:          "config owner empty detected available uses detected with note",
			cfgOwner:      "",
			cfgRepo:       "",
			detectedOwner: "detected-org",
			detectedRepo:  "",
			wantOwner:     "detected-org",
			wantOwnerNote: " (auto-detected)",
			wantRepo:      "",
			wantRepoNote:  "",
		},
		{
			name:          "config owner empty detected empty yields empty with no note",
			cfgOwner:      "",
			cfgRepo:       "",
			detectedOwner: "",
			detectedRepo:  "",
			wantOwner:     "",
			wantOwnerNote: "",
			wantRepo:      "",
			wantRepoNote:  "",
		},
		{
			name:          "config repo set uses config value with no note",
			cfgOwner:      "",
			cfgRepo:       "myrepo",
			detectedOwner: "",
			detectedRepo:  "detected-repo",
			wantOwner:     "",
			wantOwnerNote: "",
			wantRepo:      "myrepo",
			wantRepoNote:  "",
		},
		{
			name:          "config repo empty detected available uses detected with note",
			cfgOwner:      "",
			cfgRepo:       "",
			detectedOwner: "",
			detectedRepo:  "detected-repo",
			wantOwner:     "",
			wantOwnerNote: "",
			wantRepo:      "detected-repo",
			wantRepoNote:  " (auto-detected)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ownerNote, repoNote := resolveDisplayOwnerRepo(
				tt.cfgOwner, tt.cfgRepo, tt.detectedOwner, tt.detectedRepo,
			)

			riteway.Assert(t, riteway.Case[string]{
				Given:    tt.name,
				Should:   "return correct owner",
				Actual:   owner,
				Expected: tt.wantOwner,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    tt.name,
				Should:   "return correct ownerNote",
				Actual:   ownerNote,
				Expected: tt.wantOwnerNote,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    tt.name,
				Should:   "return correct repo",
				Actual:   repo,
				Expected: tt.wantRepo,
			})

			riteway.Assert(t, riteway.Case[string]{
				Given:    tt.name,
				Should:   "return correct repoNote",
				Actual:   repoNote,
				Expected: tt.wantRepoNote,
			})
		})
	}
}

// --- junitxml "(default)" annotation (lines 55–56) ---

func TestDoctorJUnitXMLDefaultAnnotation(t *testing.T) {
	// jest default junitxml is "junit.xml"; omitting junitxml in config triggers the default.
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest config with default junitxml (junit.xml)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest config using the default junitxml",
		Should:   "print '(default)' annotation next to junitxml",
		Actual:   strings.Contains(stdout, "junit.xml (default)"),
		Expected: true,
	})
}

func TestDoctorJUnitXMLNonDefaultNoAnnotation(t *testing.T) {
	// Custom junitxml must NOT show "(default)".
	path := writeTempConfig(t, `
version: 1
framework: jest
junitxml: my-results.xml
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest config with custom junitxml",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a jest config with custom junitxml",
		Should:   "not print '(default)' annotation next to custom junitxml",
		Actual:   !strings.Contains(stdout, "my-results.xml (default)"),
		Expected: true,
	})
}

// --- storage.branch "(default)" annotation (lines 81–82) ---

func TestDoctorStorageBranchDefaultAnnotation(t *testing.T) {
	// Omitting storage.branch triggers the default "quarantine/state".
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with default storage.branch (quarantine/state)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config using the default storage.branch",
		Should:   "print '(default)' annotation next to storage.branch",
		Actual:   strings.Contains(stdout, "quarantine/state (default)"),
		Expected: true,
	})
}

func TestDoctorStorageBranchNonDefaultNoAnnotation(t *testing.T) {
	// Custom branch must NOT show "(default)".
	path := writeTempConfig(t, `
version: 1
framework: jest
storage:
  branch: my-custom-branch
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with custom storage.branch",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with custom storage.branch",
		Should:   "not print '(default)' annotation next to custom branch",
		Actual:   !strings.Contains(stdout, "my-custom-branch (default)"),
		Expected: true,
	})
}

// --- No spurious "Warnings:" in valid config with token (line 86) ---

func TestDoctorValidConfigNoSpuriousWarnings(t *testing.T) {
	path := writeTempConfig(t, `
version: 1
framework: jest
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config with a token present",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a valid config with a token present and no warnings",
		Should:   "not print a 'Warnings:' section",
		Actual:   !strings.Contains(stdout, "Warnings:"),
		Expected: true,
	})
}

// --- No spurious "Warnings:" in error block when there are no warnings (line 38) ---

func TestDoctorErrorBlockNoSpuriousWarnings(t *testing.T) {
	// retries: -1 triggers a validation error. With a token present no warnings
	// are generated, so "Warnings:" must not appear in the output.
	path := writeTempConfig(t, `
version: 1
framework: jest
retries: -1
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an invalid config (retries: -1) with token present",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an invalid config with no warnings",
		Should:   "not print a 'Warnings:' section in error output",
		Actual:   !strings.Contains(stdout, "Warnings:"),
		Expected: true,
	})
}

// --- || vs && for git detection (line 62) ---

func TestDoctorGitDetectionWithOnlyOwnerSet(t *testing.T) {
	// github.owner is set but github.repo is empty. The || condition means git
	// detection should still fire so the missing repo can be auto-detected.
	// We verify the owner appears without an "(auto-detected)" note, proving the
	// config value was used rather than the detected one.
	path := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: myorg
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with github.owner set but github.repo empty",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with github.owner explicitly set",
		Should:   "print the configured owner without '(auto-detected)'",
		Actual:   strings.Contains(stdout, "myorg") && !strings.Contains(stdout, "myorg (auto-detected)"),
		Expected: true,
	})

	// When || is correct, git detection fires because repo is empty, so
	// the auto-detected repo value should appear with an annotation.
	// If || were changed to &&, detection would be skipped and repo would be blank.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with github.repo empty and running inside a git repo",
		Should:   "auto-detect and print the repo with '(auto-detected)' note",
		Actual:   strings.Contains(stdout, "(auto-detected)"),
		Expected: true,
	})
}
