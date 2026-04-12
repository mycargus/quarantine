package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- formatInitSummary unit tests ---

func TestFormatInitSummaryNewBranch(t *testing.T) {
	summary := formatInitSummary("my-org", "my-repo", []string{"jest"}, false)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest detected, new branch",
		Should:   "show config path",
		Actual:   strings.Contains(summary, ".quarantine/config.yml (created)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest detected, new branch",
		Should:   "show branch as created",
		Actual:   strings.Contains(summary, "quarantine/state (created)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest detected, new branch",
		Should:   "contain quarantine doctor next step",
		Actual:   strings.Contains(summary, "quarantine doctor"),
		Expected: true,
	})
}

func TestFormatInitSummaryExistingBranch(t *testing.T) {
	summary := formatInitSummary("my-org", "my-repo", []string{"rspec"}, true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec detected, existing branch",
		Should:   "show branch as already exists",
		Actual:   strings.Contains(summary, "quarantine/state (already exists)"),
		Expected: true,
	})
}

func TestFormatInitSummaryNoFrameworks(t *testing.T) {
	summary := formatInitSummary("my-org", "my-repo", []string{}, false)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected, new branch",
		Should:   "show config path as created",
		Actual:   strings.Contains(summary, ".quarantine/config.yml (created)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "contain 'edit' next-step text (not 'review')",
		Actual:   strings.Contains(summary, "edit .quarantine/config.yml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "not contain 'adjust suite names' review text",
		Actual:   !strings.Contains(summary, "adjust suite names"),
		Expected: true,
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
		Should:   "not contain a real suite name entry (only comments)",
		Actual:   !strings.Contains(cfg, "\n  - name:"),
		Expected: true,
	})
}

// --- formatQuarantineGitignore unit tests ---

func TestFormatQuarantineGitignore(t *testing.T) {
	content := formatQuarantineGitignore()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "formatQuarantineGitignore called",
		Should:   "contain wildcard ignore rule",
		Actual:   strings.Contains(content, "*"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "formatQuarantineGitignore called",
		Should:   "contain !config.yml exception",
		Actual:   strings.Contains(content, "!config.yml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "formatQuarantineGitignore called",
		Should:   "contain !.gitignore exception",
		Actual:   strings.Contains(content, "!.gitignore"),
		Expected: true,
	})
}
