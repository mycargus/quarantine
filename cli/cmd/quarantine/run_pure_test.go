package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/mycargus/quarantine/cli/internal/parser"
	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
	"github.com/mycargus/quarantine/cli/internal/runner"
	riteway "github.com/mycargus/riteway-golang"
)

func TestConfigResolutionTrace(t *testing.T) {
	cfg := &config.Config{
		Framework: "jest",
		Retries:   3,
		JUnitXML:  "junit.xml",
	}

	lines := configResolutionTrace(cfg, 0, 0, "", "")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "jest config with all values at defaults",
		Should:   "return 3 trace lines (no header line)",
		Actual:   len(lines),
		Expected: 3,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "jest config with all values at defaults",
		Should:   "include framework with quarantine.yml source",
		Actual:   lines[0],
		Expected: "[quarantine] config: framework=jest (from quarantine.yml)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries not set in config file or flag",
		Should:   "report retries as default source",
		Actual:   lines[1],
		Expected: "[quarantine] config: retries=3 (from default)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml not set in config file or flag",
		Should:   "report junitxml as default source",
		Actual:   lines[2],
		Expected: "[quarantine] config: junitxml=junit.xml (from default)",
	})
}

func TestConfigResolutionTraceSourceAttribution(t *testing.T) {
	cfg := &config.Config{
		Framework: "rspec",
		Retries:   5,
		JUnitXML:  "override.xml",
	}

	// Both values came from the CLI flag.
	linesFromFlag := configResolutionTrace(cfg, 5, 0, "override.xml", "")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries set via --retries flag",
		Should:   "report retries as cli flag source",
		Actual:   linesFromFlag[1],
		Expected: "[quarantine] config: retries=5 (from cli flag)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set via --junitxml flag",
		Should:   "report junitxml as cli flag source",
		Actual:   linesFromFlag[2],
		Expected: "[quarantine] config: junitxml=override.xml (from cli flag)",
	})

	// Both values came from the config file (not overridden by flag).
	linesFromConfig := configResolutionTrace(cfg, 0, 5, "", "override.xml")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries set in config file (not flag)",
		Should:   "report retries as quarantine.yml source",
		Actual:   linesFromConfig[1],
		Expected: "[quarantine] config: retries=5 (from quarantine.yml)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set in config file (not flag)",
		Should:   "report junitxml as quarantine.yml source",
		Actual:   linesFromConfig[2],
		Expected: "[quarantine] config: junitxml=override.xml (from quarantine.yml)",
	})
}

func TestMergeParseResults(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}
	r2 := []parser.TestResult{{TestID: "b", Status: "failed"}}

	merged, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{results: r2},
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "two successful parse attempts",
		Should:   "return all results merged",
		Actual:   len(merged),
		Expected: 2,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "two successful parse attempts",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})

	merged, warnings = mergeParseResults([]parseAttempt{
		{results: r1},
		{warning: "Failed to parse shard-2.xml: unexpected EOF. Skipping."},
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "1 successful and 1 failed parse attempt",
		Should:   "return results from the successful file only",
		Actual:   len(merged),
		Expected: 1,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "1 successful and 1 failed parse attempt",
		Should:   "return file-level warning plus summary warning",
		Actual:   len(warnings),
		Expected: 2,
	})

	merged, warnings = mergeParseResults([]parseAttempt{
		{warning: "Failed to parse a.xml: unexpected EOF. Skipping."},
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all parse attempts failed",
		Should:   "return nil results",
		Actual:   merged == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all parse attempts failed",
		Should:   "return warnings",
		Actual:   len(warnings) > 0,
		Expected: true,
	})

	merged, warnings = mergeParseResults([]parseAttempt{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty attempts slice",
		Should:   "return empty results",
		Actual:   len(merged),
		Expected: 0,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty attempts slice",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})
}

func TestRepoString(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner and repo both present",
		Should:   "return owner/repo",
		Actual:   repoString("acme", "api"),
		Expected: "acme/api",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner is empty",
		Should:   "return empty string",
		Actual:   repoString("", "api"),
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "repo is empty",
		Should:   "return empty string",
		Actual:   repoString("acme", ""),
		Expected: "",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "both owner and repo are empty",
		Should:   "return empty string",
		Actual:   repoString("", ""),
		Expected: "",
	})
}

func TestBuildPRScopeInputs(t *testing.T) {
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a result with no tests",
		Should:   "return empty inputs",
		Actual:   len(buildPRScopeInputs(result.Result{})),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a result with only passing tests",
		Should:   "return empty inputs (only flaky tests are classified)",
		Actual: len(buildPRScopeInputs(result.Result{
			Tests: []result.TestEntry{
				{TestID: "a", Status: "passed"},
				{TestID: "b", Status: "failed"},
				{TestID: "c", Status: "skipped"},
			},
		})),
		Expected: 0,
	})

	inputs := buildPRScopeInputs(result.Result{
		Tests: []result.TestEntry{
			{TestID: "a", FilePath: "src/a.test.js", Name: "test A", Status: "passed"},
			{TestID: "b", FilePath: "src/b.test.js", Name: "test B", Status: "flaky"},
			{TestID: "c", FilePath: "src/c.test.js", Name: "test C", Status: "flaky"},
		},
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a result with mixed statuses",
		Should:   "return only flaky test inputs",
		Actual:   len(inputs),
		Expected: 2,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "a flaky test entry",
		Should:   "preserve TestID, FilePath, and Name in the input",
		Actual:   inputs[0].TestID + "|" + inputs[0].FilePath + "|" + inputs[0].Name,
		Expected: "b|src/b.test.js|test B",
	})
}

func TestDefaultCheckPRScopeForTestsFallback(t *testing.T) {
	// Run in a non-git directory so git commands fail immediately without
	// attempting a network fetch to the real remote.
	cdTo(t, t.TempDir())

	flakyInputs := []prScopeInput{
		{TestID: "t1", FilePath: "src/a.test.js", Name: "test A"},
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GITHUB_BASE_REF is not set (empty baseRef)",
		Should:   "return empty map without running git (treat all as pre-existing)",
		Actual:   len(defaultCheckPRScopeForTests("", flakyInputs)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "no flaky test inputs",
		Should:   "return empty map without running git",
		Actual:   len(defaultCheckPRScopeForTests("main", nil)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "git commands fail (not a git repo)",
		Should:   "return empty map (fallback to pre-existing — do not break the build)",
		Actual:   len(defaultCheckPRScopeForTests("main", flakyInputs)),
		Expected: 0,
	})
}

func TestClassifyPRScope(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "file appears in added-files list",
		Should:   "return new_file_in_pr",
		Actual:   classifyPRScope([]string{"src/payment-refund.test.js"}, "src/payment-refund.test.js", "should process refund", nil),
		Expected: "new_file_in_pr",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "file not in added-files list and test name in added diff lines",
		Should:   "return new_test_in_pr",
		Actual: classifyPRScope(
			[]string{"src/other.test.js"},
			"src/payment.test.js",
			"should process refund",
			[]string{"+  it('should process refund', () => {", " existing line"},
		),
		Expected: "new_test_in_pr",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "file not in added-files list and test name not in added lines",
		Should:   "return empty string (pre-existing test)",
		Actual: classifyPRScope(
			[]string{"src/other.test.js"},
			"src/payment.test.js",
			"should process refund",
			[]string{" unchanged line", "-removed line"},
		),
		Expected: "",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "no base ref info available (empty newFiles, empty diffLines)",
		Should:   "return empty string (fallback to pre-existing)",
		Actual:   classifyPRScope(nil, "src/payment.test.js", "should process refund", nil),
		Expected: "",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test name appears only in a removed line (not added)",
		Should:   "return empty string (test pre-existed, was not added)",
		Actual: classifyPRScope(
			nil,
			"src/payment.test.js",
			"should process refund",
			[]string{"-  it('should process refund', () => {"},
		),
		Expected: "",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "empty test name with added diff lines present",
		Should:   "return empty string (empty name must not match every added line)",
		Actual: classifyPRScope(
			nil,
			"src/payment.test.js",
			"",
			[]string{"+  it('some test', () => {"},
		),
		Expected: "",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "file appears in added-files list at second position (not first)",
		Should:   "return new_file_in_pr",
		Actual: classifyPRScope(
			[]string{"src/other.test.js", "src/payment.test.js"},
			"src/payment.test.js",
			"should process refund",
			nil,
		),
		Expected: "new_file_in_pr",
	})
}

func TestAddNewFlakyTestsUpdatesExistingEntry(t *testing.T) {
	state := qstate.NewEmptyState()
	state.AddTest(qstate.Entry{
		TestID:     "src/payment.test.js::PaymentService::should process payment",
		FlakyCount: 3,
	})

	res := result.Result{
		Tests: []result.TestEntry{{
			TestID: "src/payment.test.js::PaymentService::should process payment",
			Status: "flaky",
		}},
	}

	changed := addNewFlakyTests(state, res, nil, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test already present in quarantine state",
		Should:   "return changed=true",
		Actual:   changed,
		Expected: true,
	})

	entry := state.Tests["src/payment.test.js::PaymentService::should process payment"]

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test already present in quarantine state",
		Should:   "increment FlakyCount",
		Actual:   entry.FlakyCount,
		Expected: 4,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a flaky test already present in quarantine state",
		Should:   "update LastFlakyAt to a non-empty timestamp",
		Actual:   entry.LastFlakyAt != "",
		Expected: true,
	})
}

func TestRerunFailureWarningJest(t *testing.T) {
	msg := rerunFailureWarning("should apply discount", "npx", runner.Jest)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Jest rerun command that fails to execute",
		Should:   "include the test name in the warning",
		Actual:   strings.Contains(msg, `"should apply discount"`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Jest rerun command that fails to execute",
		Should:   "include the rerun command in the warning",
		Actual:   strings.Contains(msg, "npx exited with error"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Jest rerun command that fails to execute",
		Should:   "include pnpm example with testNamePattern",
		Actual:   strings.Contains(msg, "pnpm exec jest --testNamePattern '{name}'"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Jest rerun command that fails to execute",
		Should:   "include bun example with testNamePattern",
		Actual:   strings.Contains(msg, "bunx jest --testNamePattern '{name}'"),
		Expected: true,
	})
}

func TestRerunFailureWarningRSpec(t *testing.T) {
	msg := rerunFailureWarning("should process payment", "bundle", runner.RSpec)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an RSpec rerun command that fails to execute",
		Should:   "include the test name in the warning",
		Actual:   strings.Contains(msg, `"should process payment"`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an RSpec rerun command that fails to execute",
		Should:   "include rspec -e example",
		Actual:   strings.Contains(msg, "rspec -e '{name}'"),
		Expected: true,
	})
}

func TestRerunFailureWarningVitest(t *testing.T) {
	msg := rerunFailureWarning("renders correctly", "npx", runner.Vitest)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Vitest rerun command that fails to execute",
		Should:   "include the test name in the warning",
		Actual:   strings.Contains(msg, `"renders correctly"`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Vitest rerun command that fails to execute",
		Should:   "include pnpm vitest example",
		Actual:   strings.Contains(msg, "pnpm exec vitest run"),
		Expected: true,
	})
}

// --- Command construction: flag defaults and error messages ---

func TestNewRunCmdFlagDefaults(t *testing.T) {
	cmd := newRunCmd()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --config flag to quarantine.yml",
		Actual:   cmd.Flags().Lookup("config").DefValue,
		Expected: "quarantine.yml",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --retries flag to 0",
		Actual:   cmd.Flags().Lookup("retries").DefValue,
		Expected: "0",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --output flag to .quarantine/results.json",
		Actual:   cmd.Flags().Lookup("output").DefValue,
		Expected: ".quarantine/results.json",
	})
}

func TestNewDoctorCmdFlagDefaults(t *testing.T) {
	cmd := newDoctorCmd()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newDoctorCmd with no arguments",
		Should:   "default --config flag to quarantine.yml",
		Actual:   cmd.Flags().Lookup("config").DefValue,
		Expected: "quarantine.yml",
	})
}

func TestNewRunCmdMissingSeparatorError(t *testing.T) {
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"run", "--unknown-flag"})

	err := rootCmd.Execute()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with an unknown flag and no -- separator",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with an unknown flag and no -- separator",
		Should:   "error message contains missing separator",
		Actual:   strings.Contains(err.Error(), "missing separator"),
		Expected: true,
	})
}

func TestNewVersionCmdOutput(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	// version is a package-level var; capture stdout via os.Pipe since
	// fmt.Printf bypasses cmd.OutOrStdout().
	// Instead, redirect stdout at the process level for this call.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine version command",
		Should:   "output quarantine v prefix",
		Actual:   strings.HasPrefix(out.String(), "quarantine v"),
		Expected: true,
	})
}

// --- Mutation guard: line 653 len(matches) == 0 ---

// TestParseJUnitXMLNoMatchesReturnsNil kills the mutation len(matches) > 0 on
// line 653. When the glob matches no files, parseJUnitXML must return nil
// results (not an empty non-nil slice). The caller uses testResults == nil to
// decide whether to emit the "no JUnit XML found" warning.
func TestParseJUnitXMLNoMatchesReturnsNil(t *testing.T) {
	dir := t.TempDir()
	// A glob that matches nothing.
	pattern := filepath.Join(dir, "*.xml")

	results, warnings := parseJUnitXML(pattern)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "glob pattern that matches no files",
		Should:   "return nil results (not empty slice)",
		Actual:   results == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "glob pattern that matches no files",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})
}

// TestParseJUnitXMLWithMatchReturnsNonNil is the counterpart: a valid XML file
// must produce non-nil results so the caller does NOT falsely emit the warning.
func TestParseJUnitXMLWithMatchReturnsNonNil(t *testing.T) {
	dir := t.TempDir()
	xmlContent := `<?xml version="1.0"?>
<testsuites tests="1">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.01"/>
  </testsuite>
</testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	results, _ := parseJUnitXML(xmlPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "glob pattern matching one valid XML file",
		Should:   "return non-nil results",
		Actual:   results != nil,
		Expected: true,
	})
}

// --- Mutation guard: line 694 succeeded < total ---

// TestMergeParseResultsSummaryWarningIsEmitted kills the mutation
// succeeded > total on line 694. When 1 of 2 files fails, succeeded=1,
// total=2. With the mutation: 1 > 2 = false, so no summary warning would be
// appended. This test checks that the summary warning IS present.
func TestMergeParseResultsSummaryWarningIsEmitted(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}

	_, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{warning: "Failed to parse shard-2.xml: unexpected EOF. Skipping."},
	})

	var hasSummaryWarning bool
	for _, w := range warnings {
		if strings.Contains(w, "Parsed 1/2") {
			hasSummaryWarning = true
		}
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "1 of 2 parse attempts failed (succeeded=1 < total=2)",
		Should:   "include a 'Parsed 1/2' summary warning",
		Actual:   hasSummaryWarning,
		Expected: true,
	})
}

// TestMergeParseResultsAllSucceededNoSummaryWarning is the counterpart:
// when all files parse successfully, no summary warning should appear.
func TestMergeParseResultsAllSucceededNoSummaryWarning(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}
	r2 := []parser.TestResult{{TestID: "b", Status: "passed"}}

	_, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{results: r2},
	})

	var hasSummaryWarning bool
	for _, w := range warnings {
		if strings.Contains(w, "Parsed") && strings.Contains(w, "JUnit XML") {
			hasSummaryWarning = true
		}
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all 2 parse attempts succeeded (succeeded=2, not < total=2)",
		Should:   "not include a summary warning",
		Actual:   hasSummaryWarning,
		Expected: false,
	})
}

// --- Mutation guard: line 837 res.Summary.Total == 0 ---

// TestAllTestsQuarantinedTotalZeroReturnsTrue kills the mutation
// res.Summary.Total != 0 on line 837. When Jest excludes all tests via
// exclusion flags, Total==0. allTestsQuarantined must return true so the
// "All tests are quarantined" warning is emitted.
func TestAllTestsQuarantinedTotalZeroReturnsTrue(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 0},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=0 (Jest excluded all tests)",
		Should:   "return true (all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: true,
	})
}

// TestAllTestsQuarantinedTotalNonZeroAllQuarantined tests the RSpec path:
// Total > 0 and Quarantined == Total.
func TestAllTestsQuarantinedTotalNonZeroAllQuarantined(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 2, Quarantined: 2},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=2 and Quarantined=2 (RSpec post-filtering)",
		Should:   "return true (all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: true,
	})
}

// TestAllTestsQuarantinedPartialReturnsFalse ensures a partial quarantine
// (some tests still ran) does not trigger the warning.
func TestAllTestsQuarantinedPartialReturnsFalse(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 3, Quarantined: 1, Passed: 2},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=3 but only 1 quarantined",
		Should:   "return false (not all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: false,
	})
}

// --- Mutation guard: line 710 runID == "" ---

// TestBuildMetadataUsesEnvRunID kills the mutation runID != "" on line 710.
// When GITHUB_RUN_ID is set, buildMetadata must use it as-is (not generate a
// local fallback). With the mutation the condition is flipped: a non-empty env
// value would be overwritten by the local-... fallback.
func TestBuildMetadataUsesEnvRunID(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "12345678")
	// Unset branch / SHA env vars so they fall back to git (or empty) — we
	// only care about RunID here.
	t.Setenv("GITHUB_REF_NAME", "")
	t.Setenv("GITHUB_SHA", "")

	cfg := &config.Config{Framework: "jest", Retries: 3}
	meta := buildMetadata(cfg)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GITHUB_RUN_ID=12345678 is set in the environment",
		Should:   "use the env value as RunID without generating a local fallback",
		Actual:   meta.RunID,
		Expected: "12345678",
	})
}

// TestBuildMetadataGeneratesFallbackWhenRunIDNotSet kills the same mutation
// from the other direction: when GITHUB_RUN_ID is empty, a local-... fallback
// must be generated. With the mutation the condition is `runID != ""`, so an
// empty env value would leave RunID as "" instead of generating the fallback.
func TestBuildMetadataGeneratesFallbackWhenRunIDNotSet(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "")

	cfg := &config.Config{Framework: "jest", Retries: 3}
	meta := buildMetadata(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GITHUB_RUN_ID is not set",
		Should:   "generate a local-... fallback run ID",
		Actual:   strings.HasPrefix(meta.RunID, "local-"),
		Expected: true,
	})
}

