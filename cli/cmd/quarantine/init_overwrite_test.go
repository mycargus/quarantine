package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// TestInitConfigAlreadyExistsOverwrites verifies that re-running init when
// .quarantine/config.yml already exists overwrites it with newly detected suites.
func TestInitConfigAlreadyExistsOverwrites(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Pre-create .quarantine directory with an existing config.
	if err := os.MkdirAll(dir+"/.quarantine", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dir+"/.quarantine/config.yml", []byte("version: 1\n# old config\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Add jest to package.json so init can detect it.
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	mockServer := newInitTestServer(t)

	_, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml exists and jest is in package.json",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	// File should now contain jest suite entries.
	content, readErr := os.ReadFile(dir + "/.quarantine/config.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init overwrites existing .quarantine/config.yml",
		Should:   "be readable without error",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init overwrites existing .quarantine/config.yml",
		Should:   "config now contains name: jest",
		Actual:   strings.Contains(string(content), "name: jest"),
		Expected: true,
	})
}

// TestInitMkdirAllFailure verifies that init returns an error when the
// .quarantine directory cannot be created because the path is already a file.
// Creating a file at the .quarantine path causes os.MkdirAll to return ENOTDIR
// on all platforms without requiring elevated permissions.
func TestInitMkdirAllFailure(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Pre-create a regular file where .quarantine directory should be.
	// os.MkdirAll(".quarantine", ...) will fail with ENOTDIR.
	if err := os.WriteFile(dir+"/.quarantine", []byte("not a directory"), 0644); err != nil {
		t.Fatalf("setup: write file at .quarantine path: %v", err)
	}

	// Add jest so the framework detection succeeds and reaches the mkdir step.
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("setup: write package.json: %v", err)
	}

	mockServer := newInitTestServer(t)

	_, err := executeInitCmd(t,
		"",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a file exists at the .quarantine path",
		Should:   "return an error (os.MkdirAll fails with ENOTDIR)",
		Actual:   err != nil,
		Expected: true,
	})
}
