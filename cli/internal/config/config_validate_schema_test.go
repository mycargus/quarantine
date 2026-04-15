package config_test

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/config"
)

// --- Version validation ---

func TestValidateMissingVersion(t *testing.T) {
	cfg := parseYAML(t, `

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

`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with version: 2",
		Should:   "produce an unsupported-version error",
		Actual:   containsString(errs, "Unsupported config version: 2. This version of the CLI supports version 1."),
		Expected: true,
	})
}

// --- Retries validation ---

func TestValidateRetriesNegative(t *testing.T) {
	cfg := parseYAML(t, `
version: 1

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

retries: 1
`)
	cfgMax := parseYAML(t, `
version: 1

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

// --- issue_tracker validation ---

func TestValidateUnsupportedIssueTracker(t *testing.T) {
	cfg := parseYAML(t, `
version: 1

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

func TestValidateNoLabelsFieldProducesNoCustomLabelsError(t *testing.T) {
	cfg := parseYAML(t, `
version: 1

`)

	errs, _ := cfg.Validate()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine.yml with no labels field",
		Should:   "not produce a custom-labels error",
		Actual:   containsString(errs, "Custom labels are not supported in this version. Only ['quarantine'] is accepted."),
		Expected: false,
	})
}

// --- Per-suite retries validation (ValidateSuiteRetries is a pure function) ---

func TestValidateSuiteRetriesAboveMax(t *testing.T) {
	err := config.ValidateSuiteRetries(20)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of 20 (above max)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteRetriesNegativeRejected(t *testing.T) {
	err := config.ValidateSuiteRetries(-1)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of -1 (negative)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteRetriesJustAboveMax(t *testing.T) {
	err := config.ValidateSuiteRetries(11)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of 11 (just above max)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteRetriesBoundaries(t *testing.T) {
	errMin := config.ValidateSuiteRetries(1)
	errMax := config.ValidateSuiteRetries(10)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of 1 (lower boundary)",
		Should:   "not return an error",
		Actual:   errMin == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of 10 (upper boundary)",
		Should:   "not return an error",
		Actual:   errMax == nil,
		Expected: true,
	})
}

func TestValidateSuiteRetriesZeroAccepted(t *testing.T) {
	err := config.ValidateSuiteRetries(0)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries value of 0 (unset/default)",
		Should:   "not return an error",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestValidateSuitesIncludesRetriesError(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: ["npm", "test"]
    junitxml: "junit.xml"
    rerun_command: ["npm", "test", "--", "{name}"]
    retries: 20
`)

	errs := config.ValidateSuites([]config.TestSuite{suite})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with retries: 20 validated through ValidateSuites",
		Should:   "include the suite-prefixed retries error",
		Actual:   containsString(errs, `suite "backend": invalid retries value: 20, must be between 1 and 10`),
		Expected: true,
	})
}
