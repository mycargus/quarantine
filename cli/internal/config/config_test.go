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
