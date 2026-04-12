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

// --- hasPackageKey unit tests (exercised via detectFrameworks) ---

func TestHasPackageKeyMalformedSection(t *testing.T) {
	// devDependencies value is not a valid JSON object — hasPackageKey should
	// handle the Unmarshal error and return false without panicking.
	pkgJSON := `{"devDependencies":"not-an-object"}`

	frameworks := detectFrameworks(pkgJSON, "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "devDependencies is a string (not an object) in package.json",
		Should:   "detect no frameworks (malformed section is ignored, not a panic)",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

func TestHasPackageKeyAbsentSections(t *testing.T) {
	// package.json has no dependencies or devDependencies keys at all.
	pkgJSON := `{"scripts":{"test":"jest"}}`

	frameworks := detectFrameworks(pkgJSON, "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with no dependencies or devDependencies keys",
		Should:   "detect no frameworks",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

// --- detectFrameworks unit tests ---

func TestDetectFrameworksJestInDevDependencies(t *testing.T) {
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`

	frameworks := detectFrameworks(pkgJSON, "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with jest in devDependencies",
		Should:   "detect one framework",
		Actual:   len(frameworks),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with jest in devDependencies",
		Should:   "detect jest",
		Actual:   frameworks[0],
		Expected: "jest",
	})
}

func TestDetectFrameworksJestInDependencies(t *testing.T) {
	pkgJSON := `{"dependencies":{"jest":"^29.0.0"}}`

	frameworks := detectFrameworks(pkgJSON, "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with jest in dependencies",
		Should:   "detect one framework",
		Actual:   len(frameworks),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with jest in dependencies",
		Should:   "detect jest",
		Actual:   frameworks[0],
		Expected: "jest",
	})
}

func TestDetectFrameworksVitestInDevDependencies(t *testing.T) {
	pkgJSON := `{"devDependencies":{"vitest":"^1.0.0"}}`

	frameworks := detectFrameworks(pkgJSON, "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with vitest in devDependencies",
		Should:   "detect one framework",
		Actual:   len(frameworks),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with vitest in devDependencies",
		Should:   "detect vitest",
		Actual:   frameworks[0],
		Expected: "vitest",
	})
}

func TestDetectFrameworksRSpecSingleQuotes(t *testing.T) {
	gemfile := "source 'https://rubygems.org'\ngem 'rspec', '~> 3.12'\n"

	frameworks := detectFrameworks("", gemfile)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with gem 'rspec' (single quotes)",
		Should:   "detect one framework",
		Actual:   len(frameworks),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "Gemfile with gem 'rspec' (single quotes)",
		Should:   "detect rspec",
		Actual:   frameworks[0],
		Expected: "rspec",
	})
}

func TestDetectFrameworksRSpecDoubleQuotes(t *testing.T) {
	gemfile := `gem "rspec", "~> 3.12"`

	frameworks := detectFrameworks("", gemfile)

	riteway.Assert(t, riteway.Case[string]{
		Given:    `Gemfile with gem "rspec" (double quotes)`,
		Should:   "detect rspec",
		Actual:   frameworks[0],
		Expected: "rspec",
	})
}

func TestDetectFrameworksRSpecRailsIsNotRSpec(t *testing.T) {
	// gem 'rspec-rails' does not match gem 'rspec' — the closing quote
	// follows 'rspec', so 'rspec-rails' has a different substring.
	gemfile := "gem 'rspec-rails'"

	frameworks := detectFrameworks("", gemfile)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with gem 'rspec-rails' but not gem 'rspec'",
		Should:   "detect no frameworks (rspec-rails is not rspec)",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

func TestDetectFrameworksRSpecWithoutQuotesNotDetected(t *testing.T) {
	// 'gem rspec' without quotes is not a valid gem declaration
	// and should not be detected.
	gemfile := "gem rspec"

	frameworks := detectFrameworks("", gemfile)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with 'gem rspec' (no quotes)",
		Should:   "detect no frameworks",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

func TestDetectFrameworksNone(t *testing.T) {
	pkgJSON := `{"devDependencies":{"typescript":"^5.0.0"}}`
	gemfile := "gem 'rails'"

	frameworks := detectFrameworks(pkgJSON, gemfile)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json without jest/vitest and Gemfile without rspec",
		Should:   "detect no frameworks",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

func TestDetectFrameworksEmpty(t *testing.T) {
	frameworks := detectFrameworks("", "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty package.json and Gemfile content",
		Should:   "detect no frameworks",
		Actual:   len(frameworks),
		Expected: 0,
	})
}

func TestDetectFrameworksJestAndRSpec(t *testing.T) {
	pkgJSON := `{"devDependencies":{"jest":"^29.0.0"}}`
	gemfile := "gem 'rspec'"

	frameworks := detectFrameworks(pkgJSON, gemfile)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with jest and Gemfile with rspec",
		Should:   "detect two frameworks",
		Actual:   len(frameworks),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with jest and Gemfile with rspec",
		Should:   "first framework is jest",
		Actual:   frameworks[0],
		Expected: "jest",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with jest and Gemfile with rspec",
		Should:   "second framework is rspec",
		Actual:   frameworks[1],
		Expected: "rspec",
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
