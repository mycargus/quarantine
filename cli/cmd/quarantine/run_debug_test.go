package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 90: QUARANTINE_DEBUG env var enables debug output without --verbose ---

func TestRunQUARANTINEDebugEnvEnablesVerboseOutput(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)
	// Use a nonexistent command so the run exits at LookPath (lines 94-97 in run.go),
	// which is AFTER the config resolution trace at lines 85-89. The verbose output
	// IS printed before LookPath, so this command reliably triggers the trace.
	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		"QUARANTINE_DEBUG":        "1",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "QUARANTINE_DEBUG=1 set without --verbose flag",
		Should:   "print config resolution trace (debug output equivalent to --verbose)",
		Actual:   strings.Contains(output, "[quarantine] config:"),
		Expected: true,
	})
}

// --- Scenario 92 (precedence): QUARANTINE_DEBUG with --quiet suppresses debug output ---

func TestRunQUARANTINEDebugWithQuietSuppressesVerboseOutput(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)
	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--quiet",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		"QUARANTINE_DEBUG":        "1",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "QUARANTINE_DEBUG=1 set AND --quiet flag passed",
		Should:   "suppress config resolution trace (--quiet takes precedence over QUARANTINE_DEBUG)",
		Actual:   strings.Contains(output, "[quarantine] config:"),
		Expected: false,
	})
}
