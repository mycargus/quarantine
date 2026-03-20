package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 1: First-time setup with Jest ---

func TestInitJestFirstTime(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"jest\n\n\n", // framework=jest, retries=default, junitxml=default
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
		Should:   "print success message",
		Actual:   strings.Contains(stdout, "Quarantine initialized successfully"),
		Expected: true,
	})

	// Verify quarantine.yml was created.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "create quarantine.yml",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml written by init",
		Should:   "contain version: 1",
		Actual:   strings.Contains(string(content), "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml written by init",
		Should:   "contain framework: jest",
		Actual:   strings.Contains(string(content), "framework: jest"),
		Expected: true,
	})

	// Jest-specific recommendation should be printed.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest framework selected",
		Should:   "print jest-junit recommendation",
		Actual:   strings.Contains(stdout, "jest-junit"),
		Expected: true,
	})
}

// --- Scenario 2: quarantine init with RSpec ---

func TestInitRSpec(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"rspec\n\n\n", // framework=rspec, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "write framework: rspec to quarantine.yml",
		Actual:   strings.Contains(string(content), "framework: rspec"),
		Expected: true,
	})

	// No jest-junit recommendation for rspec.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "NOT print jest-junit recommendation",
		Actual:   !strings.Contains(stdout, "jest-junit"),
		Expected: true,
	})

	// Workflow snippet should be rspec-specific.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "print rspec workflow snippet",
		Actual:   strings.Contains(stdout, "RspecJunitFormatter") || strings.Contains(stdout, "rspec"),
		Expected: true,
	})
}

// --- Scenario 3: quarantine init with Vitest ---

func TestInitVitest(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"vitest\n\n\n", // framework=vitest, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "write framework: vitest to quarantine.yml",
		Actual:   strings.Contains(string(content), "framework: vitest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "print vitest workflow snippet with --reporter=junit",
		Actual:   strings.Contains(stdout, "--reporter=junit"),
		Expected: true,
	})
}

// --- Scenario 5: quarantine/state branch already exists ---

func TestInitBranchAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t, withExistingBranch())

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
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
		Should:   "print message about skipping branch creation",
		Actual:   strings.Contains(stdout, "already exists") && strings.Contains(stdout, "Skipping"),
		Expected: true,
	})
}

// --- Scenario 10: invalid framework input ---

func TestInitInvalidFrameworkRePrompts(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", mockServer.server.URL)

	// Enter invalid framework first, then valid one.
	stdout, _ := executeInitCmd(t,
		"pytest\njest\n\n\n",
		dir,
		nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user enters 'pytest' then 'jest' at the framework prompt",
		Should:   "print exact invalid framework message for 'pytest'",
		Actual:   strings.Contains(stdout, "Invalid framework 'pytest'. Supported: rspec, jest, vitest."),
		Expected: true,
	})
}
