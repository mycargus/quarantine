package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

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
