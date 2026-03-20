package config_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/config"
)

// parseYAML is a test helper that calls Parse on a YAML string.
func parseYAML(t *testing.T, yaml string) *config.Config {
	t.Helper()
	cfg, err := config.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	return cfg
}

// containsString reports whether s appears in slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// --- Parse ---

func TestParseValidMinimalConfig(t *testing.T) {
	yaml := `
version: 1
framework: jest
`
	cfg, err := config.Parse(strings.NewReader(yaml))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "decode version correctly",
		Actual:   cfg.Version,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "decode framework correctly",
		Actual:   cfg.Framework,
		Expected: "jest",
	})
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/quarantine.yml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a path that does not exist",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestParseInvalidYAML(t *testing.T) {
	yaml := `version: [unclosed`

	_, err := config.Parse(strings.NewReader(yaml))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed YAML",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Unknown top-level keys → warnings ---

func TestValidateUnknownTopLevelKeyProducesWarning(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
typo_field: oops
`)

	_, warns := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with an unknown top-level key 'typo_field'",
		Should:   "produce a warning mentioning the unknown field",
		Actual:   containsString(warns, "Unknown field 'typo_field' in quarantine.yml will be ignored."),
		Expected: true,
	})
}

func TestValidateMultipleUnknownTopLevelKeysProduceOneWarningEach(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
alpha: 1
beta: 2
`)

	_, warns := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with two unknown top-level keys",
		Should:   "produce a warning for 'alpha'",
		Actual:   containsString(warns, "Unknown field 'alpha' in quarantine.yml will be ignored."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with two unknown top-level keys",
		Should:   "produce a warning for 'beta'",
		Actual:   containsString(warns, "Unknown field 'beta' in quarantine.yml will be ignored."),
		Expected: true,
	})
}

func TestValidateKnownTopLevelKeysProduceNoUnknownWarning(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
retries: 3
junitxml: junit.xml
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state
exclude: []
rerun_command: ""
`)

	_, warns := cfg.Validate()

	for _, w := range warns {
		if strings.HasPrefix(w, "Unknown field '") {
			riteway.Assert(t, riteway.Case[string]{
				Given:    "a quarantine.yml with only known top-level keys",
				Should:   "not produce any unknown-field warnings",
				Actual:   w,
				Expected: "",
			})
		}
	}
}

// --- Unknown notifications keys → errors ---

func TestValidateUnknownNotificationChannelProducesError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
notifications:
  slack: true
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with an unknown notifications key 'slack'",
		Should:   "produce an error naming the unsupported channel",
		Actual: containsString(errs,
			"Unknown notification channel 'slack'. This version supports: github_pr_comment. "+
				"Slack and email notifications are planned for a future release."),
		Expected: true,
	})
}

func TestValidateMultipleUnknownNotificationChannelsProduceOneErrorEach(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
notifications:
  slack: true
  email: alerts@example.com
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "notifications with two unknown channels",
		Should:   "produce an error for 'slack'",
		Actual: containsString(errs,
			"Unknown notification channel 'slack'. This version supports: github_pr_comment. "+
				"Slack and email notifications are planned for a future release."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "notifications with two unknown channels",
		Should:   "produce an error for 'email'",
		Actual: containsString(errs,
			"Unknown notification channel 'email'. This version supports: github_pr_comment. "+
				"Slack and email notifications are planned for a future release."),
		Expected: true,
	})
}

func TestValidateKnownNotificationKeyProducesNoError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
notifications:
  github_pr_comment: true
`)

	errs, _ := cfg.Validate()

	for _, e := range errs {
		if strings.HasPrefix(e, "Unknown notification channel '") {
			riteway.Assert(t, riteway.Case[string]{
				Given:    "notifications with only the known key 'github_pr_comment'",
				Should:   "not produce any unknown-channel errors",
				Actual:   e,
				Expected: "",
			})
		}
	}
}

// --- Unknown storage keys → errors ---

func TestValidateUnknownStorageFieldProducesError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
storage:
  backend: actions-cache
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with an unknown storage key 'backend'",
		Should:   "produce an error naming the unsupported field",
		Actual:   containsString(errs, "Unknown storage field 'backend'. This version supports: branch."),
		Expected: true,
	})
}

func TestValidateMultipleUnknownStorageFieldsProduceOneErrorEach(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
storage:
  backend: actions-cache
  region: us-east-1
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "storage with two unknown keys",
		Should:   "produce an error for 'backend'",
		Actual:   containsString(errs, "Unknown storage field 'backend'. This version supports: branch."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "storage with two unknown keys",
		Should:   "produce an error for 'region'",
		Actual:   containsString(errs, "Unknown storage field 'region'. This version supports: branch."),
		Expected: true,
	})
}

func TestValidateKnownStorageKeyProducesNoError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
storage:
  branch: quarantine/state
`)

	errs, _ := cfg.Validate()

	for _, e := range errs {
		if strings.HasPrefix(e, "Unknown storage field '") {
			riteway.Assert(t, riteway.Case[string]{
				Given:    "storage with only the known key 'branch'",
				Should:   "not produce any unknown-storage errors",
				Actual:   e,
				Expected: "",
			})
		}
	}
}

// --- Version validation ---

func TestValidateMissingVersion(t *testing.T) {
	cfg := parseYAML(t, `
framework: jest
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with no version field",
		Should:   "produce a missing-version error",
		Actual:   containsString(errs, "Missing required field 'version' in quarantine.yml."),
		Expected: true,
	})
}

func TestValidateUnsupportedVersion(t *testing.T) {
	cfg := parseYAML(t, `
version: 2
framework: jest
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with version: 2",
		Should:   "produce an unsupported-version error",
		Actual:   containsString(errs, "Unsupported config version: 2. This version of the CLI supports version 1."),
		Expected: true,
	})
}

// --- Framework validation ---

func TestValidateMissingFramework(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with no framework field",
		Should:   "produce a missing-framework error",
		Actual:   containsString(errs, "Missing required field 'framework' in quarantine.yml."),
		Expected: true,
	})
}

func TestValidateInvalidFramework(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: pytest
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with framework: pytest",
		Should:   "produce an unknown-framework error",
		Actual:   containsString(errs, "Unknown framework 'pytest'. Supported frameworks: rspec, jest, vitest."),
		Expected: true,
	})
}

// --- Retries validation ---

func TestValidateRetriesNegative(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
retries: -1
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: -1",
		Should:   "produce an out-of-range retries error",
		Actual:   containsString(errs, "Invalid retries value: -1. Must be between 1 and 10."),
		Expected: true,
	})
}

func TestValidateRetriesAboveMax(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
retries: 11
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: 11",
		Should:   "produce an out-of-range retries error",
		Actual:   containsString(errs, "Invalid retries value: 11. Must be between 1 and 10."),
		Expected: true,
	})
}

func TestValidateRetriesAtBoundary(t *testing.T) {
	cfgMin := parseYAML(t, `
version: 1
framework: jest
retries: 1
`)
	cfgMax := parseYAML(t, `
version: 1
framework: jest
retries: 10
`)

	errsMin, _ := cfgMin.Validate()
	errsMax, _ := cfgMax.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: 1 (minimum valid)",
		Should:   "not produce a retries error",
		Actual:   containsString(errsMin, "Invalid retries value: 1. Must be between 1 and 10."),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: 10 (maximum valid)",
		Should:   "not produce a retries error",
		Actual:   containsString(errsMax, "Invalid retries value: 10. Must be between 1 and 10."),
		Expected: false,
	})
}

func TestValidateRetriesZeroIsNotAnError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
retries: 0
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with retries: 0 (treated as unset)",
		Should:   "not produce a retries error",
		Actual:   containsString(errs, "Invalid retries value"),
		Expected: false,
	})

	cfg.ApplyDefaults()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "retries: 0 after ApplyDefaults",
		Should:   "default to 3",
		Actual:   cfg.Retries,
		Expected: 3,
	})
}

// --- issue_tracker validation ---

func TestValidateUnsupportedIssueTracker(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
issue_tracker: jira
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with issue_tracker: jira",
		Should:   "produce an unsupported-issue-tracker error",
		Actual: containsString(errs,
			"Unsupported issue_tracker 'jira'. This version supports: github. "+
				"Jira support is planned for a future release."),
		Expected: true,
	})
}

func TestValidateSupportedIssueTracker(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
issue_tracker: github
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with issue_tracker: github",
		Should:   "not produce an issue_tracker error",
		Actual:   containsString(errs, "Unsupported issue_tracker"),
		Expected: false,
	})
}

// --- labels validation ---

func TestValidateCustomLabels(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
labels:
  - flaky
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with a custom label 'flaky'",
		Should:   "produce a custom-labels error",
		Actual:   containsString(errs, "Custom labels are not supported in this version. Only ['quarantine'] is accepted."),
		Expected: true,
	})
}

func TestValidateMultipleLabels(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
labels:
  - quarantine
  - flaky
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with multiple labels including the default one",
		Should:   "produce a custom-labels error",
		Actual:   containsString(errs, "Custom labels are not supported in this version. Only ['quarantine'] is accepted."),
		Expected: true,
	})
}

func TestValidateDefaultLabel(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
labels:
  - quarantine
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with the default label ['quarantine']",
		Should:   "not produce a labels error",
		Actual:   containsString(errs, "Custom labels"),
		Expected: false,
	})
}

// --- Labels: empty labels list produces no error ---

func TestValidateNoLabelsFieldProducesNoCustomLabelsError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
framework: jest
`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with no labels field",
		Should:   "not produce a custom-labels error",
		Actual:   containsString(errs, "Custom labels are not supported in this version. Only ['quarantine'] is accepted."),
		Expected: false,
	})
}

// --- ApplyDefaults does not overwrite already-set values ---

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

// --- Unknown keys do not shadow existing validation errors ---

func TestValidateUnknownTopLevelKeyAlongsideMissingRequired(t *testing.T) {
	cfg := parseYAML(t, `
version: 1
mystery_key: oops
`)
	// framework is missing

	errs, warns := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config missing 'framework' and containing an unknown top-level key",
		Should:   "still produce the missing-framework error",
		Actual:   containsString(errs, "Missing required field 'framework' in quarantine.yml."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a config missing 'framework' and containing an unknown top-level key",
		Should:   "also produce the unknown-field warning",
		Actual:   containsString(warns, "Unknown field 'mystery_key' in quarantine.yml will be ignored."),
		Expected: true,
	})
}
