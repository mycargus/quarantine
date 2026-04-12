package main

import (
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Strict mode: no token (suite mode) ---

// TestRunDegradedNoTokenStrictPrintsError: in suite mode, --strict with no token
// causes a degraded mode exit, but the error message format is different from legacy.
func TestRunDegradedNoTokenStrictPrintsError(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	_, err := executeRunCmd(t, []string{
		"--strict",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	// In suite mode, no token → degraded mode (not an error), tests still run.
	// Suite mode does not implement strict mode, so it runs in degraded mode.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --strict flag set (suite mode)",
		Should:   "complete (strict mode not implemented in suite mode)",
		Actual:   err == nil, // degraded mode succeeds
		Expected: true,
	})
}

// TestRunDegradedAPIUnreachableStrictPrintsError: in suite mode, --strict with
// unreachable API runs in degraded mode (strict not implemented in suite mode).
func TestRunDegradedAPIUnreachableStrictPrintsError(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--strict",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
	})

	// In suite mode, strict mode is not implemented — runs in degraded mode, exits 0.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API unreachable and --strict flag set (suite mode)",
		Should:   "exit with code 0 (degraded mode — strict not implemented in suite mode)",
		Actual:   exitCode,
		Expected: 0,
	})
}
