package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 4: quarantine.yml already exists ---

func TestInitConfigAlreadyExistsAbort(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Write existing config.
	if err := os.WriteFile(dir+"/quarantine.yml", []byte("version: 1\nframework: rspec\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// User enters 'n' (or just enter) to not overwrite.
	stdout, err := executeInitCmd(t,
		"n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user declines overwrite",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user declines overwrite",
		Should:   "print 'Aborted'",
		Actual:   strings.Contains(stdout, "Aborted") || strings.Contains(stdout, "preserved"),
		Expected: true,
	})

	// Existing file should be unchanged.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user aborts init",
		Should:   "preserve existing quarantine.yml content",
		Actual:   readErr == nil && strings.Contains(string(content), "rspec"),
		Expected: true,
	})
}

// --- Scenario 4: quarantine.yml already exists — user enters y to overwrite ---

func TestInitConfigAlreadyExistsOverwrite(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Write existing config with rspec.
	if err := os.WriteFile(dir+"/quarantine.yml", []byte("version: 1\nframework: rspec\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mockServer := newInitTestServer(t)

	// User enters 'y' to overwrite, then selects jest.
	stdout, err := executeInitCmd(t,
		"y\njest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user enters 'y' to overwrite",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user enters 'y' to overwrite",
		Should:   "print success message",
		Actual:   strings.Contains(stdout, "Quarantine initialized successfully"),
		Expected: true,
	})

	// File should now contain jest, not rspec.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user overwrites existing config with jest",
		Should:   "quarantine.yml now contains framework: jest",
		Actual:   readErr == nil && strings.Contains(string(content), "framework: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user overwrites existing config with jest",
		Should:   "quarantine.yml no longer contains framework: rspec",
		Actual:   !strings.Contains(string(content), "framework: rspec"),
		Expected: true,
	})
}

// --- Retries prompt: non-default and invalid input ---

func TestInitCustomRetriesInput(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// No token so init fails after writing config — retries is written before GitHub ops.
	_, _ = executeInitCmd(t,
		"jest\n5\n\n", // framework=jest, retries=5, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "",
			"GITHUB_TOKEN":            "",
		},
	)

	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries input '5' provided at the prompt",
		Should:   "write quarantine.yml without error",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries input '5' provided at the prompt",
		Should:   "write 'retries: 5' to quarantine.yml",
		Actual:   strings.Contains(string(content), "retries: 5"),
		Expected: true,
	})
}

func TestInitInvalidRetriesInputUsesDefault(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// No token so init fails after writing config.
	_, _ = executeInitCmd(t,
		"jest\nabc\n\n", // framework=jest, retries=invalid, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "",
			"GITHUB_TOKEN":            "",
		},
	)

	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid retries input 'abc' provided at the prompt",
		Should:   "write quarantine.yml without error",
		Actual:   readErr == nil,
		Expected: true,
	})

	// writeConfig omits the retries key when it equals the default (3).
	// If the mutation fires (err == nil → err != nil), Atoi("abc") returns (0, err);
	// err != nil becomes true → retries is set to 0 → written as "retries: 0".
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid retries input 'abc' (Atoi fails)",
		Should:   "not write a retries key (default 3 is omitted by writeConfig)",
		Actual:   !strings.Contains(string(content), "retries:"),
		Expected: true,
	})
}

// --- writeConfig unit test ---

func TestWriteConfigFailure(t *testing.T) {
	err := writeConfig("/nonexistent/path/quarantine.yml", "jest", 3, "junit.xml", "junit.xml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a path inside a nonexistent directory",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}
