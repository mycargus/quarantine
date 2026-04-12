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
		Should:   "print 'Skipping' in output",
		Actual:   strings.Contains(stdout, "Skipping"),
		Expected: true,
	})
}

// --- Empty defaultBranch fallback ---

func TestInitEmptyDefaultBranchFallsBackToMain(t *testing.T) {
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
