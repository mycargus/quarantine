package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Strict mode: no token ---

func TestRunDegradedNoTokenStrictExitsTwo(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	scriptPath := writeTestScript(t, dir, "", "", 0)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--strict",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "no GitHub token and --strict flag set",
		Should:   "exit with code 2",
		Actual:   exitCode,
		Expected: 2,
	})
}

func TestRunDegradedNoTokenStrictPrintsError(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)
	scriptPath := writeTestScript(t, dir, "", "", 0)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--strict",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --strict flag set",
		Should:   "print infrastructure failure ERROR message",
		Actual:   strings.Contains(output, "[quarantine] ERROR:") && strings.Contains(output, "infrastructure failure"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --strict flag set",
		Should:   "suggest removing --strict to run in degraded mode",
		Actual:   strings.Contains(output, "Remove --strict"),
		Expected: true,
	})
}

// --- Strict mode: API unreachable ---

func TestRunDegradedAPIUnreachableStrictExitsTwo(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)
	scriptPath := writeTestScript(t, dir, "", "", 0)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--strict",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API unreachable and --strict flag set",
		Should:   "exit with code 2",
		Actual:   exitCode,
		Expected: 2,
	})
}

func TestRunDegradedAPIUnreachableStrictDoesNotRunTests(t *testing.T) {
	dir := t.TempDir()

	// Script writes a marker file if it runs — we verify it didn't.
	markerPath := filepath.Join(dir, "test-ran")
	scriptPath := filepath.Join(dir, "fake-test")
	script := "#!/bin/sh\ntouch " + markerPath + "\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	_ = executeRunCmdWithExitCode(t, []string{
		"--config", configPath,
		"--strict",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	_, statErr := os.Stat(markerPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API unreachable and --strict flag set",
		Should:   "not run tests before exiting with code 2",
		Actual:   os.IsNotExist(statErr),
		Expected: true,
	})
}

func TestRunDegradedAPIUnreachableStrictPrintsError(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)
	scriptPath := writeTestScript(t, dir, "", "", 0)

	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--strict",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API unreachable and --strict flag set",
		Should:   "print [quarantine] ERROR: infrastructure failure (--strict mode)",
		Actual:   strings.Contains(output, "[quarantine] ERROR:") && strings.Contains(output, "--strict mode"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API unreachable and --strict flag set",
		Should:   "print message about exiting with code 2",
		Actual:   strings.Contains(output, "exiting with code 2"),
		Expected: true,
	})
}
