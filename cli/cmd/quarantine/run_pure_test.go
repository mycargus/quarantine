package main

import (
	"testing"

	"github.com/mycargus/quarantine/internal/config"
	"github.com/mycargus/quarantine/internal/parser"
	qstate "github.com/mycargus/quarantine/internal/quarantine"
	"github.com/mycargus/quarantine/internal/result"
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
		Should:   "return 4 trace lines",
		Actual:   len(lines),
		Expected: 4,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "jest config with all values at defaults",
		Should:   "include framework with config source",
		Actual:   lines[1],
		Expected: "[verbose]   framework = jest (source: config)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries not set in config file or flag",
		Should:   "report retries as default source",
		Actual:   lines[2],
		Expected: "[verbose]   retries   = 3 (source: default)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml not set in config file or flag",
		Should:   "report junitxml as default source",
		Actual:   lines[3],
		Expected: "[verbose]   junitxml  = junit.xml (source: default)",
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
		Should:   "report retries as flag source",
		Actual:   linesFromFlag[2],
		Expected: "[verbose]   retries   = 5 (source: flag)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set via --junitxml flag",
		Should:   "report junitxml as flag source",
		Actual:   linesFromFlag[3],
		Expected: "[verbose]   junitxml  = override.xml (source: flag)",
	})

	// Both values came from the config file (not overridden by flag).
	linesFromConfig := configResolutionTrace(cfg, 0, 5, "", "override.xml")
	riteway.Assert(t, riteway.Case[string]{
		Given:    "retries set in config file (not flag)",
		Should:   "report retries as config source",
		Actual:   linesFromConfig[2],
		Expected: "[verbose]   retries   = 5 (source: config)",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "junitxml set in config file (not flag)",
		Should:   "report junitxml as config source",
		Actual:   linesFromConfig[3],
		Expected: "[verbose]   junitxml  = override.xml (source: config)",
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

