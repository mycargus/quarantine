package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// setupJestRepo creates a temp dir with a git repo and package.json containing jest.
func setupJestRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, remoteURL)
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	return dir
}

// setupRSpecRepo creates a temp dir with a git repo and Gemfile containing rspec.
func setupRSpecRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, remoteURL)
	gemfile := "gem 'rspec', '~> 3.12'\n"
	if err := os.WriteFile(dir+"/Gemfile", []byte(gemfile), 0644); err != nil {
		t.Fatalf("write Gemfile: %v", err)
	}
	return dir
}

// setupVitestRepo creates a temp dir with a git repo and package.json containing vitest.
func setupVitestRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, remoteURL)
	pkgJSON := `{"devDependencies":{"vitest":"^1.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	return dir
}

// --- Scenario 1: First-time setup with Jest ---

func TestInitJestFirstTime(t *testing.T) {
	t.Skip("Legacy phase-2 happy path — superseded by ADR-037 / M20 two-phase init flow. Will be re-implemented per Scenario 175 (re-run after hand-edit) once phase 2 reads owner/repo from config.")
	dir := setupJestRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup with valid GitHub token and repo",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "print Quarantine initialized message",
		Actual:   strings.Contains(stdout, "Quarantine initialized."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "mention detected jest framework in output",
		Actual:   strings.Contains(stdout, "jest"),
		Expected: true,
	})

	// Verify .quarantine/config.yml was created.
	content, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "create .quarantine/config.yml",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml written by init",
		Should:   "contain version: 1",
		Actual:   strings.Contains(string(content), "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml written by init",
		Should:   "contain jest suite name",
		Actual:   strings.Contains(string(content), "name: jest"),
		Expected: true,
	})
}

// --- Scenario 2: quarantine init with RSpec ---

func TestInitRSpec(t *testing.T) {
	t.Skip("Legacy phase-2 happy path — superseded by ADR-037 / M20. Will be re-implemented per Scenario 175.")
	dir := setupRSpecRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec detected from Gemfile",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec detected from Gemfile",
		Should:   "write name: rspec to .quarantine/config.yml",
		Actual:   strings.Contains(string(content), "name: rspec"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec detected from Gemfile",
		Should:   "print detected rspec framework",
		Actual:   strings.Contains(stdout, "rspec"),
		Expected: true,
	})
}

// --- Scenario 3: quarantine init with Vitest ---

func TestInitVitest(t *testing.T) {
	t.Skip("Legacy phase-2 happy path — superseded by ADR-037 / M20. Will be re-implemented per Scenario 175.")
	dir := setupVitestRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest detected from package.json",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest detected from package.json",
		Should:   "write name: vitest to .quarantine/config.yml",
		Actual:   strings.Contains(string(content), "name: vitest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest detected from package.json",
		Should:   "print detected vitest framework",
		Actual:   strings.Contains(stdout, "vitest"),
		Expected: true,
	})
}

// --- Scenario 5: quarantine/state branch already exists ---

func TestInitBranchAlreadyExists(t *testing.T) {
	t.Skip("Legacy phase-2 idempotency — superseded by ADR-037 / M20. Will be re-implemented per Scenario 175.")
	dir := setupJestRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t, withExistingBranch())

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "print 'already exists' in output",
		Actual:   strings.Contains(stdout, "already exists"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "print 'skipping' in output",
		Actual:   strings.Contains(stdout, "skipping"),
		Expected: true,
	})
}

// --- Scenario 111: no frameworks detected writes commented example ---

func TestInitNoFrameworksDetectedWritesCommentedConfig(t *testing.T) {
	t.Skip("Legacy phase-2 happy path — superseded by ADR-037 / M20. Will be re-implemented per Scenario 175.")
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")
	// No package.json and no Gemfile — zero frameworks detected.
	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no package.json and no Gemfile",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no package.json and no Gemfile",
		Should:   "print 'No test frameworks detected.'",
		Actual:   strings.Contains(stdout, "No test frameworks detected."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no package.json and no Gemfile",
		Should:   "print 'Quarantine initialized.'",
		Actual:   strings.Contains(stdout, "Quarantine initialized."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no package.json and no Gemfile",
		Should:   "print next step hint to edit config.yml",
		Actual:   strings.Contains(stdout, "edit .quarantine/config.yml"),
		Expected: true,
	})

	// Verify .quarantine/config.yml was created with commented example.
	content, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "create .quarantine/config.yml",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml written with no frameworks",
		Should:   "contain commented example suite entry",
		Actual:   strings.Contains(string(content), "# Add your test suites here"),
		Expected: true,
	})

	// Verify .quarantine/.gitignore was created.
	_, gitignoreErr := os.Stat(dir + "/.quarantine/.gitignore")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "create .quarantine/.gitignore",
		Actual:   gitignoreErr == nil,
		Expected: true,
	})
}

// --- Scenario 112: quarantine init is idempotent ---

func TestInitIdempotentSkipsExistingArtifacts(t *testing.T) {
	dir := setupJestRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t, withExistingBranch())

	// Pre-create .quarantine directory with user-customized config.
	if err := os.MkdirAll(dir+"/.quarantine", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	customConfig := "version: 1\n# my-custom-suite\n"
	if err := os.WriteFile(dir+"/.quarantine/config.yml", []byte(customConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(dir+"/.quarantine/.gitignore", []byte("*\n!.gitignore\n!config.yml\n"), 0644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all artifacts already exist (config, gitignore, branch)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	// Config must be unchanged — custom content preserved.
	content, configReadErr := os.ReadFile(dir + "/.quarantine/config.yml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config already exists with user-customized content",
		Should:   "read config without error",
		Actual:   configReadErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config already exists with user-customized content",
		Should:   "preserve custom config content (not overwrite)",
		Actual:   strings.Contains(string(content), "my-custom-suite"),
		Expected: true,
	})

	// .gitignore must be unchanged — original content preserved.
	gitignore, gitignoreReadErr := os.ReadFile(dir + "/.quarantine/.gitignore")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "gitignore already exists",
		Should:   "read .gitignore without error",
		Actual:   gitignoreReadErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "gitignore already exists",
		Should:   "preserve .gitignore content (not overwrite)",
		Actual:   strings.Contains(string(gitignore), "!config.yml"),
		Expected: true,
	})

	// Output should mention each skipped artifact.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config already exists",
		Should:   "mention config.yml already exists in output",
		Actual:   strings.Contains(stdout, "config.yml") && strings.Contains(stdout, "skipping"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "gitignore already exists",
		Should:   "mention .gitignore already exists in output",
		Actual:   strings.Contains(stdout, ".gitignore") && strings.Contains(stdout, "skipping"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "mention branch already exists in output",
		Actual:   strings.Contains(stdout, "branch") && strings.Contains(stdout, "skipping"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all artifacts already exist",
		Should:   "print token validated in output",
		Actual:   strings.Contains(stdout, "token validated") || strings.Contains(stdout, "GitHub token"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all artifacts already exist",
		Should:   "print already initialized message",
		Actual:   strings.Contains(stdout, "already initialized"),
		Expected: true,
	})
}

// --- Scenario 113: recovery when branch is missing but config and gitignore exist ---

func TestInitRecreatesMissingStateBranch(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Pre-create .quarantine/config.yml and .quarantine/.gitignore.
	if err := os.MkdirAll(dir+"/.quarantine", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	customConfig := "version: 1\n# my-custom-suite\n"
	if err := os.WriteFile(dir+"/.quarantine/config.yml", []byte(customConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(dir+"/.quarantine/.gitignore", []byte("*\n!.gitignore\n!config.yml\n"), 0644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}

	// newInitTestServer without withExistingBranch() — branch check returns 404.
	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config and gitignore exist but quarantine/state branch is missing",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	// Config must be unchanged.
	content, configReadErr := os.ReadFile(dir + "/.quarantine/config.yml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config exists before re-running init",
		Should:   "read config without error",
		Actual:   configReadErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config exists before re-running init",
		Should:   "leave config content unchanged",
		Actual:   strings.Contains(string(content), "my-custom-suite"),
		Expected: true,
	})

	// Recovery summary must appear in output.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "state branch was missing and has been recreated",
		Should:   "print 'recovered' in output",
		Actual:   strings.Contains(stdout, "recovered"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "state branch was deleted (previous state lost)",
		Should:   "print 'not recoverable' data-loss warning in output",
		Actual:   strings.Contains(stdout, "not recoverable"),
		Expected: true,
	})
}

// --- Empty defaultBranch fallback ---

func TestInitEmptyDefaultBranchFallsBackToMain(t *testing.T) {
	t.Skip("Legacy phase-2 default-branch fallback — superseded by ADR-037 / M20. Will be re-implemented per Scenario 175.")
	dir := setupJestRepo(t, "https://github.com/my-org/my-project.git")
	mockServer := newInitTestServer(t, withEmptyDefaultBranch())

	_, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns empty default_branch field",
		Should:   "fall back to 'main' and initialize successfully",
		Actual:   err == nil,
		Expected: true,
	})
}
