package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- formatPhase2Summary unit tests ---

func TestFormatPhase2SummaryNewBranch(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "phase 2 with newly created quarantine/state branch",
		Should:   "render the M20 setup-complete summary with all owner/repo/branch lines",
		Actual:   formatPhase2Summary("my-org", "my-project", false),
		Expected: "\nQuarantine initialized.\n  github.owner:  my-org (from config)\n  github.repo:   my-project (from config)\n  Branch:        quarantine/state (created)\n",
	})
}

func TestFormatPhase2SummaryExistingBranch(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "phase 2 with quarantine/state branch already present (idempotent re-run)",
		Should:   "render the M20 setup-complete summary with branch noted as 'already exists'",
		Actual:   formatPhase2Summary("my-org", "my-project", true),
		Expected: "\nQuarantine initialized.\n  github.owner:  my-org (from config)\n  github.repo:   my-project (from config)\n  Branch:        quarantine/state (already exists)\n",
	})
}

// --- formatInitConfig unit tests ---

func TestFormatInitConfigJestOnly(t *testing.T) {
	cfg := formatInitConfig("my-org", "my-repo", []string{"jest"})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain version: 1",
		Actual:   strings.Contains(cfg, "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain github owner",
		Actual:   strings.Contains(cfg, "owner: my-org"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain github repo",
		Actual:   strings.Contains(cfg, "repo: my-repo"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain test_suites key",
		Actual:   strings.Contains(cfg, "test_suites:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain name: jest",
		Actual:   strings.Contains(cfg, "name: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest only",
		Should:   "contain npx jest --ci command tokens",
		Actual:   strings.Contains(cfg, "npx") && strings.Contains(cfg, "--ci"),
		Expected: true,
	})
}

func TestFormatInitConfigRSpecOnly(t *testing.T) {
	cfg := formatInitConfig("my-org", "my-repo", []string{"rspec"})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec only",
		Should:   "contain name: rspec",
		Actual:   strings.Contains(cfg, "name: rspec"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec only",
		Should:   "contain bundle exec rspec command tokens",
		Actual:   strings.Contains(cfg, "bundle") && strings.Contains(cfg, "exec"),
		Expected: true,
	})
}

func TestFormatInitConfigJestAndRSpec(t *testing.T) {
	cfg := formatInitConfig("my-org", "my-repo", []string{"jest", "rspec"})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest and rspec",
		Should:   "contain name: jest",
		Actual:   strings.Contains(cfg, "name: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest and rspec",
		Should:   "contain name: rspec",
		Actual:   strings.Contains(cfg, "name: rspec"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "frameworks slice was [jest, rspec]",
		Should:   "render jest before rspec (preserve input order)",
		Actual:   strings.Index(cfg, "name: jest") < strings.Index(cfg, "name: rspec"),
		Expected: true,
	})
}

// --- joinQuoted unit tests ---

func TestJoinQuotedEmpty(t *testing.T) {
	result := joinQuoted([]string{})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "empty slice",
		Should:   "return empty string",
		Actual:   result,
		Expected: "",
	})
}

func TestJoinQuotedSingleItem(t *testing.T) {
	result := joinQuoted([]string{"npx"})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "single-item slice",
		Should:   "return a double-quoted string with no comma",
		Actual:   result,
		Expected: `"npx"`,
	})
}

func TestJoinQuotedMultipleItems(t *testing.T) {
	result := joinQuoted([]string{"npx", "jest", "--ci"})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three-item slice",
		Should:   "return comma-separated double-quoted strings",
		Actual:   result,
		Expected: `"npx", "jest", "--ci"`,
	})
}

// --- formatNoFrameworkConfig unit tests ---

func TestFormatNoFrameworkConfigContainsCommentedExample(t *testing.T) {
	cfg := formatNoFrameworkConfig("my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain version: 1",
		Actual:   strings.Contains(cfg, "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain github owner",
		Actual:   strings.Contains(cfg, "owner: my-org"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain github repo",
		Actual:   strings.Contains(cfg, "repo: my-repo"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain test_suites: key",
		Actual:   strings.Contains(cfg, "test_suites:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain commented example suite entry",
		Actual:   strings.Contains(cfg, "# Add your test suites here"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "include suite-entry markers only inside commented examples",
		Actual:   uncommentedSuiteEntryCount(cfg) == 0,
		Expected: true,
	})
}

// uncommentedSuiteEntryCount counts lines that look like a live YAML suite
// entry (`- name:` after optional whitespace, NOT preceded by a `#` on the
// same line). This is a structural check that survives indentation changes
// in the formatter — it asserts the rule "no live suites under test_suites"
// independently of how many spaces the formatter uses.
// This is a pure function — no I/O.
func uncommentedSuiteEntryCount(cfg string) int {
	count := 0
	for _, line := range strings.Split(cfg, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- name:") {
			count++
		}
	}
	return count
}

// --- formatQuarantineGitignore unit tests ---

func TestFormatQuarantineGitignore(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "formatQuarantineGitignore called",
		Should:   "return the .quarantine/.gitignore body that ignores all runtime files except config.yml and .gitignore",
		Actual:   formatQuarantineGitignore(),
		Expected: "# Ignore all runtime files. Only config.yml is source-controlled.\n*\n!.gitignore\n!config.yml\n",
	})
}
