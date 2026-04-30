package main

import (
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Strict mode: no token (suite mode) ---

// TestRunDegradedNoTokenStrictPrintsError: per Scenario 178 / ADR-037, the
// no-token case fails fast with exit 2 regardless of --strict — the missing
// token is detected before strict-mode handling even runs.
func TestRunDegradedNoTokenStrictPrintsError(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--strict",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "no GitHub token and --strict flag set (suite mode)",
		Should:   "exit 2 (fail-fast per Scenario 178 / ADR-037)",
		Actual:   exitCode,
		Expected: 2,
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
