package config_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/config"
)

// parseSuiteYAML is a helper that parses a YAML test_suites block and returns
// the first suite. It exercises the real YAML unmarshaling path so CommandNode
// is populated correctly.
func parseSuiteYAML(t *testing.T, yamlContent string) config.TestSuite {
	t.Helper()
	cfg, err := config.Parse(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("parseSuiteYAML: %v", err)
	}
	if len(cfg.TestSuites) == 0 {
		t.Fatal("parseSuiteYAML: no test_suites found")
	}
	return cfg.TestSuites[0]
}

// --- IsSuiteConfig unit tests ---

func TestIsSuiteConfigTrueWhenTestSuitesPresent(t *testing.T) {
	cfg, err := config.Parse(strings.NewReader(`
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "YAML with a populated test_suites key",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "YAML with test_suites key present",
		Should:   "IsSuiteConfig() return true",
		Actual:   cfg.IsSuiteConfig(),
		Expected: true,
	})
}

func TestIsSuiteConfigTrueWhenTestSuitesIsEmpty(t *testing.T) {
	cfg, err := config.Parse(strings.NewReader(`
version: 1
test_suites: []
`))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "YAML with test_suites: [] (empty array)",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "YAML with test_suites: [] (empty array but key is present)",
		Should:   "IsSuiteConfig() return true (key is present even if empty)",
		Actual:   cfg.IsSuiteConfig(),
		Expected: true,
	})
}

func TestIsSuiteConfigFalseWhenNoTestSuitesKey(t *testing.T) {
	cfg, err := config.Parse(strings.NewReader(`
version: 1
framework: jest
`))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "legacy YAML without test_suites key",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "legacy YAML without test_suites key",
		Should:   "IsSuiteConfig() return false",
		Actual:   cfg.IsSuiteConfig(),
		Expected: false,
	})
}

// --- TestSuite.Commands() unit tests ---

func TestCommandsValidSequence(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`)

	cmds := suite.Commands()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a suite with command as a YAML sequence",
		Should:   "return a slice with 3 elements",
		Actual:   len(cmds),
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a suite with command [\"bundle\", \"exec\", \"rspec\"]",
		Should:   "return 'bundle' as first element",
		Actual:   cmds[0],
		Expected: "bundle",
	})
}

func TestCommandsStringReturnsNil(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: "bundle exec rspec"
    junitxml: "rspec.xml"
`)

	cmds := suite.Commands()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with command as a YAML string (invalid)",
		Should:   "return nil (not a sequence)",
		Actual:   cmds == nil,
		Expected: true,
	})
}

func TestCommandsZeroValueReturnsNil(t *testing.T) {
	// A zero-value TestSuite has no CommandNode set.
	suite := config.TestSuite{}

	cmds := suite.Commands()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-value TestSuite with no command set",
		Should:   "return nil",
		Actual:   cmds == nil,
		Expected: true,
	})
}

// --- ValidateSuiteName ---

func TestValidateSuiteNameValidLowercase(t *testing.T) {
	err := config.ValidateSuiteName("backend")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid lowercase name 'backend'",
		Should:   "return nil",
		Actual:   err,
		Expected: nil,
	})
}

func TestValidateSuiteNameValidWithHyphen(t *testing.T) {
	err := config.ValidateSuiteName("e2e-tests")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid name with a hyphen 'e2e-tests'",
		Should:   "return nil",
		Actual:   err,
		Expected: nil,
	})
}

func TestValidateSuiteNameValidMaxLength(t *testing.T) {
	// Exactly 30 characters.
	name := "abcdefghijklmnopqrstuvwxyz1234"
	err := config.ValidateSuiteName(name)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a name that is exactly 30 characters",
		Should:   "return nil (at limit, not over)",
		Actual:   err,
		Expected: nil,
	})
}

func TestValidateSuiteNameEmpty(t *testing.T) {
	err := config.ValidateSuiteName("")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty name",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteNameTooLong(t *testing.T) {
	// 31 characters — one over the limit.
	name := "abcdefghijklmnopqrstuvwxyz12345"
	err := config.ValidateSuiteName(name)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a name that is 31 characters (one over the limit)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteNameUppercase(t *testing.T) {
	err := config.ValidateSuiteName("Backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a name with an uppercase letter 'Backend'",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteNameSpecialChars(t *testing.T) {
	err := config.ValidateSuiteName("my backend!")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a name with spaces and special chars 'my backend!'",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestValidateSuiteNameLeadingHyphen(t *testing.T) {
	err := config.ValidateSuiteName("-backend")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a name starting with a hyphen '-backend'",
		Should:   "return an error (hyphen only valid after first char)",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- ValidateSuiteCommand ---

func TestValidateSuiteCommandValidArray(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`)

	err := config.ValidateSuiteCommand(&suite)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a suite with command as a YAML array",
		Should:   "return nil",
		Actual:   err,
		Expected: nil,
	})
}

func TestValidateSuiteCommandStringReturnsError(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: "bundle exec rspec"
    junitxml: "rspec.xml"
`)

	err := config.ValidateSuiteCommand(&suite)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with command as a YAML string",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with command as a YAML string",
		Should:   "error message mentions 'YAML array'",
		Actual:   strings.Contains(err.Error(), "YAML array"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with 'bundle exec rspec' as string command",
		Should:   "error message includes suggested array syntax",
		Actual:   strings.Contains(err.Error(), `"bundle"`) && strings.Contains(err.Error(), `"rspec"`),
		Expected: true,
	})
}

// --- ValidateSuiteJUnitXML ---

func TestValidateSuiteJUnitXMLValidPath(t *testing.T) {
	err := config.ValidateSuiteJUnitXML("junit.xml")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a non-empty junitxml path 'junit.xml'",
		Should:   "return nil",
		Actual:   err,
		Expected: nil,
	})
}

func TestValidateSuiteJUnitXMLEmpty(t *testing.T) {
	err := config.ValidateSuiteJUnitXML("")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty junitxml path",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty junitxml path",
		Should:   "error message says junitxml is required",
		Actual:   strings.Contains(err.Error(), "required"),
		Expected: true,
	})
}

// --- ValidateSuites ---

func TestValidateSuitesValidSingle(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
`)

	errs := config.ValidateSuites([]config.TestSuite{suite})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a single valid suite",
		Should:   "return zero errors",
		Actual:   len(errs),
		Expected: 0,
	})
}

func TestValidateSuitesAllErrorsCollected(t *testing.T) {
	suite := parseSuiteYAML(t, `
version: 1
test_suites:
  - name: "My Backend!"
    command: "bundle exec rspec"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
`)

	errs := config.ValidateSuites([]config.TestSuite{suite})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a suite with invalid name, string command, and missing junitxml",
		Should:   "collect all 3 errors (not short-circuit after first)",
		Actual:   len(errs),
		Expected: 3,
	})
}

func TestValidateSuitesMultipleSuitesCollectsAllErrors(t *testing.T) {
	// Parse may succeed or fail depending on how yaml.v3 handles mixed command types.
	// In either case, use whatever suites were decoded.
	cfg, _ := config.Parse(strings.NewReader(`
version: 1
test_suites:
  - name: "Bad Name!"
    command: ["npx", "jest"]
    junitxml: "junit.xml"
  - name: frontend
    command: "npm test"
    junitxml: "junit.xml"
`))

	var suites []config.TestSuite
	if cfg != nil {
		suites = cfg.TestSuites
	}

	errs := config.ValidateSuites(suites)

	// At minimum: suite[0] has an invalid name, suite[1] has a string command.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two suites each with one error",
		Should:   "report at least 2 errors (one per suite)",
		Actual:   len(errs) >= 2,
		Expected: true,
	})
}
