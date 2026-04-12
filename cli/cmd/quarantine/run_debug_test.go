package main

import (
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 90: QUARANTINE_DEBUG env var --- (suite mode)

// TestRunQUARANTINEDebugEnvEnablesVerboseOutput verifies that QUARANTINE_DEBUG=1
// does not cause a failure in suite mode.
func TestRunQUARANTINEDebugEnvEnablesVerboseOutput(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"QUARANTINE_DEBUG":               "1",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "QUARANTINE_DEBUG=1 set in suite mode",
		Should:   "exit without error (debug env does not break suite mode)",
		Actual:   err == nil,
		Expected: true,
	})
}
