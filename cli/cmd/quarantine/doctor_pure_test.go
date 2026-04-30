package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- storage.branch "(default)" annotation (lines 81–82) ---

func TestDoctorStorageBranchDefaultAnnotation(t *testing.T) {
	t.Skip("M20: superseded by ADR-037 — doctor now requires github.owner / github.repo and a 200 reachability response. The 'quarantine/state (default)' annotation behavior is exercised through TestDoctorReadsConfigAndCallsReachabilityOnce and the existing storage default tests in config validation.")
	// Omitting storage.branch triggers the default "quarantine/state".
	writeTempConfig(t, `
version: 1
`)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
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
	t.Skip("M20: superseded by ADR-037 — doctor now requires github.owner / github.repo and a 200 reachability response. The non-default branch handling is covered by config validation tests.")
	// Custom branch must NOT show "(default)".
	writeTempConfig(t, `
version: 1
storage:
  branch: my-custom-branch
`)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
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
	t.Skip("M20: superseded by ADR-037 — doctor's 'Warnings:' section was emitted by the legacy missing-token warning path, which is now a hard error. The error path's spurious-warnings behavior is still asserted by TestDoctorErrorBlockNoSpuriousWarnings.")
	writeTempConfig(t, `
version: 1
`)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
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
	writeTempConfig(t, `
version: 1
retries: -1
`)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
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

// --- detectRetryTimes pure unit tests (Scenarios 115/116) ---

func TestDetectRetryTimes(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantHits []string
	}{
		{
			name: "config-style retryTimes with non-zero value is detected",
			files: map[string]string{
				"jest.config.js": "module.exports = {\n  retryTimes: 2,\n};\n",
			},
			wantHits: []string{"jest.config.js"},
		},
		{
			name: "call-style retryTimes with non-zero value is detected",
			files: map[string]string{
				"example.test.js": "jest.retryTimes(3);\n",
			},
			wantHits: []string{"example.test.js"},
		},
		{
			name: "retryTimes(0) call-style zero value is not detected",
			files: map[string]string{
				"example.test.js": "jest.retryTimes(0);\n",
			},
			wantHits: []string{},
		},
		{
			name: "retryTimes: 0 config-style zero value is not detected",
			files: map[string]string{
				"jest.config.js": "module.exports = { retryTimes: 0 };\n",
			},
			wantHits: []string{},
		},
		{
			name:     "empty files map returns no hits",
			files:    map[string]string{},
			wantHits: []string{},
		},
		{
			name: "multiple files only matching ones returned",
			files: map[string]string{
				"jest.config.js":  "module.exports = { retryTimes: 5 };\n",
				"jest.config.ts":  "export default { retries: 3 };\n",
				"example.test.js": "jest.retryTimes(0);\n",
			},
			wantHits: []string{"jest.config.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := detectRetryTimes(tt.files)

			riteway.Assert(t, riteway.Case[int]{
				Given:    tt.name,
				Should:   "return the correct number of matching files",
				Actual:   len(actual),
				Expected: len(tt.wantHits),
			})

			for _, want := range tt.wantHits {
				found := false
				for _, got := range actual {
					if got == want {
						found = true
						break
					}
				}
				riteway.Assert(t, riteway.Case[bool]{
					Given:    tt.name,
					Should:   "include expected file path '" + want + "' in hits",
					Actual:   found,
					Expected: true,
				})
			}
		})
	}
}

// --- || vs && for git detection (line 62) ---

func TestDoctorGitDetectionWithOnlyOwnerSet(t *testing.T) {
	t.Skip("M20: superseded by ADR-037 — doctor no longer auto-detects owner/repo from git origin. A missing github.repo is now a hard error (covered by Scenario 180 in a later commit).")
	// github.owner is set but github.repo is empty. The || condition means git
	// detection should still fire so the missing repo can be auto-detected.
	// We verify the owner appears without an "(auto-detected)" note, proving the
	// config value was used rather than the detected one.
	dir := t.TempDir()
	// Create a fake git repo with a remote so doctor can auto-detect repo.
	gitInit(t, dir, "git@github.com:myorg/detected-repo.git")
	suiteDir := dir + "/.quarantine"
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(suiteDir+"/config.yml", []byte(`
version: 1
github:
  owner: myorg
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
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
