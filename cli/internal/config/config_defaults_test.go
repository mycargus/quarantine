package config_test

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/config"
)

// --- ApplyDefaults does not overwrite already-set values ---

func TestApplyDefaultsDoesNotOverwriteRetries(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
retries: 5
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a config with retries already set to a non-zero value",
		Should:   "not overwrite retries with the default (3)",
		Actual:   cfg.Retries,
		Expected: 5,
	})
}

func TestApplyDefaultsUnknownFramework(t *testing.T) {
	cfg := &config.Config{Framework: "pytest"}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a config with unknown framework 'pytest'",
		Should:   "leave JUnitXML empty (no default for unknown framework)",
		Actual:   cfg.JUnitXML,
		Expected: "",
	})
}

func TestApplyDefaultsDoesNotOverwriteJUnitXML(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
junitxml: my-custom-junit.xml
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a config with junitxml already set",
		Should:   "not overwrite junitxml with the framework default",
		Actual:   cfg.JUnitXML,
		Expected: "my-custom-junit.xml",
	})
}

func TestApplyDefaultsDoesNotOverwriteIssueTracker(t *testing.T) {
	// Use a non-default value so the mutated condition (issue_tracker != "")
	// would visibly overwrite it with "github".
	cfg := parseYAML(t, `
version: 1
framework: jest
issue_tracker: jira
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a config with issue_tracker already set to a non-default value",
		Should:   "not overwrite issue_tracker with the default",
		Actual:   cfg.IssueTracker,
		Expected: "jira",
	})
}

func TestApplyDefaultsDoesNotOverwriteLabels(t *testing.T) {
	// Use a non-default label so the mutated condition (len(labels) != 0)
	// would visibly overwrite it with ["quarantine"].
	cfg := parseYAML(t, `
version: 1
framework: jest
labels:
  - flaky
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a config with labels already set to a non-default value",
		Should:   "not overwrite labels",
		Actual:   len(cfg.Labels),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a config with labels already set to a non-default value",
		Should:   "preserve the existing label value",
		Actual:   cfg.Labels[0],
		Expected: "flaky",
	})
}

func TestApplyDefaultsDoesNotOverwriteGitHubPRCommentFalse(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
notifications:
  github_pr_comment: false
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config with github_pr_comment explicitly set to false",
		Should:   "not overwrite the value with the default (true)",
		Actual:   *cfg.Notifications.GitHubPRComment,
		Expected: false,
	})
}

func TestApplyDefaultsDoesNotOverwriteStorageBranch(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
storage:
  branch: my-custom-branch
`)

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a config with storage.branch already set",
		Should:   "not overwrite storage.branch",
		Actual:   cfg.Storage.Branch,
		Expected: "my-custom-branch",
	})
}

// --- ApplyDefaults sets exact default values on a zero-value Config ---

func TestApplyDefaultsSetsRetriesToThree(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a zero-value config",
		Should:   "set Retries to exactly 3",
		Actual:   cfg.Retries,
		Expected: 3,
	})
}

func TestApplyDefaultsSetsIssueTrackerToGitHub(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a zero-value config",
		Should:   "set IssueTracker to exactly \"github\"",
		Actual:   cfg.IssueTracker,
		Expected: "github",
	})
}

func TestApplyDefaultsSetsLabelToQuarantine(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a zero-value config",
		Should:   "set Labels[0] to exactly \"quarantine\"",
		Actual:   cfg.Labels[0],
		Expected: "quarantine",
	})
}

func TestApplyDefaultsSetsGitHubPRCommentToTrue(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-value config",
		Should:   "set GitHubPRComment to exactly true",
		Actual:   *cfg.Notifications.GitHubPRComment,
		Expected: true,
	})
}

func TestApplyDefaultsSetsStorageBranchToQuarantineState(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a zero-value config",
		Should:   "set Storage.Branch to exactly \"quarantine/state\"",
		Actual:   cfg.Storage.Branch,
		Expected: "quarantine/state",
	})
}

// FrameworkDefaultJUnit is tested here as it directly supports ApplyDefaults.

func TestFrameworkDefaultJUnitKnownFrameworks(t *testing.T) {
	cases := []struct {
		framework string
		expected  string
	}{
		{"jest", "junit.xml"},
		{"rspec", "rspec.xml"},
		{"vitest", "junit-report.xml"},
	}

	for _, tc := range cases {
		riteway.Assert(t, riteway.Case[string]{
			Given:    "framework " + tc.framework,
			Should:   "return default junitxml glob " + tc.expected,
			Actual:   config.FrameworkDefaultJUnit(tc.framework),
			Expected: tc.expected,
		})
	}
}

func TestFrameworkDefaultJUnitUnknownFramework(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "an unknown framework 'pytest'",
		Should:   "return empty string",
		Actual:   config.FrameworkDefaultJUnit("pytest"),
		Expected: "",
	})
}
