package main

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- isRecoveryMode unit tests ---

func TestIsRecoveryModeAllConditionsMet(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config skipped, gitignore skipped, branch does not exist",
		Should:   "return true (recovery mode)",
		Actual:   isRecoveryMode(true, true, false),
		Expected: true,
	})
}

func TestIsRecoveryModeConfigNotSkipped(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config was NOT skipped (new install), gitignore skipped, branch missing",
		Should:   "return false (not recovery mode — config was just created)",
		Actual:   isRecoveryMode(false, true, false),
		Expected: false,
	})
}

func TestIsRecoveryModeGitignoreNotSkipped(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config skipped, gitignore was NOT skipped, branch missing",
		Should:   "return false (not recovery mode — gitignore was just created)",
		Actual:   isRecoveryMode(true, false, false),
		Expected: false,
	})
}

func TestIsRecoveryModeBranchExists(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config skipped, gitignore skipped, but branch already exists",
		Should:   "return false (not recovery mode — already initialized)",
		Actual:   isRecoveryMode(true, true, true),
		Expected: false,
	})
}

// --- buildSuiteEntry unit tests ---

func TestBuildSuiteEntryJest(t *testing.T) {
	entry := buildSuiteEntry("jest")

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "framework jest",
		Should:   "return name jest",
		Actual:   entry["name"],
		Expected: "jest",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework jest",
		Should:   "return junitxml junit.xml",
		Actual:   entry["junitxml"].(string),
		Expected: "junit.xml",
	})
}

func TestBuildSuiteEntryRSpec(t *testing.T) {
	entry := buildSuiteEntry("rspec")

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "framework rspec",
		Should:   "return name rspec",
		Actual:   entry["name"],
		Expected: "rspec",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework rspec",
		Should:   "return junitxml rspec.xml",
		Actual:   entry["junitxml"].(string),
		Expected: "rspec.xml",
	})
}

func TestBuildSuiteEntryVitest(t *testing.T) {
	entry := buildSuiteEntry("vitest")

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "framework vitest",
		Should:   "return name vitest",
		Actual:   entry["name"],
		Expected: "vitest",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework vitest",
		Should:   "return junitxml junit-report.xml",
		Actual:   entry["junitxml"].(string),
		Expected: "junit-report.xml",
	})
}

func TestBuildSuiteEntryUnknownFramework(t *testing.T) {
	entry := buildSuiteEntry("mocha")

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "unknown framework mocha",
		Should:   "return name mocha",
		Actual:   entry["name"],
		Expected: "mocha",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "unknown framework mocha",
		Should:   "return empty command slice",
		Actual:   len(entry["command"].([]string)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "unknown framework mocha",
		Should:   "return empty rerun_command slice",
		Actual:   len(entry["rerun_command"].([]string)),
		Expected: 0,
	})
}
