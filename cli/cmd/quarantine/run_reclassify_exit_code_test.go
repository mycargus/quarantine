package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// --- Scenario 154: quarantined failure + unresolved → exit 2 ---
//
// When a previously quarantined test ends up "unresolved" (rerun command
// crashes), reclassification must NOT suppress the unresolved status.
// resolveExitCode sees Unresolved>0 and returns 2.

func TestRunQuarantinedUnresolvedExitsTwo(t *testing.T) {
	dir := t.TempDir()

	// Quarantine state: one quarantined test with an open issue.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::User::validates email",
		"validates email",
		"spec/models/user_spec.rb",
		50, // issue #50 — open
	))

	// Main test command: writes XML with the quarantined test failing, exits 1.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="RSpec" tests="1" failures="1" errors="0">
  <testcase classname="User" name="validates email"
            file="spec/models/user_spec.rb" time="0.05">
    <failure message="expected true, got false">expected true, got false</failure>
  </testcase>
</testsuite>`

	xmlPath := filepath.Join(dir, "rspec.xml")

	// Main script: writes XML and exits 1.
	mainScript := filepath.Join(dir, "fake-rspec")
	mainScriptContent := fmt.Sprintf("#!/bin/sh\ncat > %s << 'EOF'\n%s\nEOF\nexit 1\n", xmlPath, junitXML)
	if err := os.WriteFile(mainScript, []byte(mainScriptContent), 0755); err != nil {
		t.Fatalf("write main script: %v", err)
	}

	// Rerun command: non-existent binary → crashes with exit 127.
	writeSuiteConfig(t, dir, "backend", mainScript, xmlPath, "/nonexistent-rerun-binary-xyz")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, qs, []int{})
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	// The quarantined test's rerun crashes → "unresolved". ReclassifyQuarantinedTests
	// leaves "unresolved" unchanged. resolveExitCode sees Unresolved>0 → exit 2.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined test whose rerun command crashes (unresolved), no genuine failures",
		Should:   "exit 2 (infrastructure error not suppressed by reclassification)",
		Actual:   exitCode,
		Expected: 2,
	})
}
