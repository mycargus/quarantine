package main

import (
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 118: single suite configured + no name arg → runs it automatically ---

func TestRunSingleSuiteAutoSelected(t *testing.T) {
	dir := t.TempDir()

	// Pre-write a JUnit XML with 1 passing test.
	xmlPath := filepath.Join(dir, "unit.xml")
	unitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="unit" tests="1" skipped="0" failures="0" errors="0" time="0.050">
  <testcase classname="pkg.foo" name="TestFoo" file="./foo_test.go" time="0.050"></testcase>
</testsuite>`
	if err := os.WriteFile(xmlPath, []byte(unitXML), 0644); err != nil {
		t.Fatalf("write unit.xml: %v", err)
	}

	// Fake binary that exits 0 and creates a sentinel file to prove it was executed.
	sentinelPath := filepath.Join(dir, "command-ran")
	fakeBin := filepath.Join(dir, "fake-runner")
	script := "#!/bin/sh\ntouch " + sentinelPath + "\nexit 0\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-runner: %v", err)
	}

	// Create .quarantine/config.yml with exactly one suite named "unit".
	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["` + fakeBin + `"]
    junitxml: "` + xmlPath + `"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	// Run with NO suite name argument — should auto-select the only suite.
	_, err := executeRunCmd(t, []string{
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "exactly one suite named 'unit' in config and no suite name arg",
		Should:   "auto-select the suite and exit 0",
		Actual:   err == nil,
		Expected: true,
	})

	// Verify the suite command was actually executed (not just a no-op exit 0).
	_, statErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "exactly one suite auto-selected",
		Should:   "have executed the suite command (sentinel file created by fake-runner)",
		Actual:   statErr == nil,
		Expected: true,
	})
}

// --- Scenario 119: multiple suites + no name arg → exit 2 with list ---

func TestRunMultipleSuitesNoArgExitsTwo(t *testing.T) {
	dir := t.TempDir()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
  - name: frontend
    command: ["jest", "--ci"]
    junitxml: "jest.xml"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	// Run with NO suite name argument.
	_, err := executeRunCmd(t, []string{
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	// Exit code 2: unit test of selectSuite already verifies the message content.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suites ('backend', 'frontend') and no suite name arg",
		Should:   "return a non-zero error (exit 2)",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Scenario 120: no suites configured → exit 2 with guidance ---

func TestRunNoSuitesConfiguredExitsTwo(t *testing.T) {
	dir := t.TempDir()

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	// Empty test_suites: [] sets hasSuitesKey=true so the CLI enters suite mode
	// and selectSuite returns an error.
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	// Exit code 2: unit test of selectSuite verifies the message content.
	_, err := executeRunCmd(t, []string{
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with empty test_suites array and no suite name arg",
		Should:   "return a non-zero error (exit 2)",
		Actual:   err != nil,
		Expected: true,
	})
}
