package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 175: phase 2 reads owner/repo from config, completes setup, exits 0 ---
//
// Phase 2 must NOT scan the git origin URL — it reads `github.owner` and
// `github.repo` from the existing `.quarantine/config.yml` and uses those
// values to construct the GitHub client. We use a Gerrit origin throughout
// this file to prove that explicitly: a github.com origin would let the legacy
// `git.ParseRemote` path produce a working answer by accident.

// preCreateValidPhase2Config writes a `.quarantine/config.yml` with valid
// `github.owner: my-org` and `github.repo: my-project` plus a single jest
// suite. Mirrors the shape of `formatInitConfig(...)` so doctor + run
// validators accept it.
func preCreateValidPhase2Config(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir+"/.quarantine", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `version: 1

github:
  owner: my-org
  repo: my-project

issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state

test_suites:
  - name: jest
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
    retries: 3
`
	if err := os.WriteFile(dir+"/.quarantine/config.yml", []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestInitPhase2ReadsOwnerRepoFromConfigAndCreatesBranch(t *testing.T) {
	dir := t.TempDir()
	// Gerrit origin proves init does NOT scan origin in phase 2: a Gerrit URL
	// would fail `ParseRemote`'s `u.Host == "github.com"` check and surface as
	// "not a GitHub URL" if the legacy path were still in use.
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	preCreateValidPhase2Config(t, dir)
	originalConfig, _ := os.ReadFile(dir + "/.quarantine/config.yml")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "phase 2 with valid config (github.owner, github.repo set) and a Gerrit origin",
		Should:   "exit with code 0 (success)",
		Actual:   exitCodeFromError(err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 2 finished after creating the quarantine/state branch",
		Should:   "print the M20 setup-complete summary on stdout",
		Actual:   strings.Contains(stdout, formatPhase2Summary("my-org", "my-project", false)),
		Expected: true,
	})

	currentConfig, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 2 ran against an existing user-edited config",
		Should:   "leave .quarantine/config.yml readable",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "phase 2 ran against an existing user-edited config",
		Should:   "leave .quarantine/config.yml byte-for-byte unchanged",
		Actual:   string(currentConfig),
		Expected: string(originalConfig),
	})
}

func TestInitPhase2BranchAlreadyExistsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	preCreateValidPhase2Config(t, dir)

	mockServer := newInitTestServer(t, withExistingBranch())

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "phase 2 with valid config and an already-existing quarantine/state branch",
		Should:   "exit with code 0 (idempotent re-run)",
		Actual:   exitCodeFromError(err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 2 finished against a pre-existing quarantine/state branch",
		Should:   "print the M20 setup-complete summary noting the branch already exists",
		Actual:   strings.Contains(stdout, formatPhase2Summary("my-org", "my-project", true)),
		Expected: true,
	})
}
