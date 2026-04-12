package main

import (
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/parser"
	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)


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

	changed := addNewFlakyTests(state, res, nil)

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
		Should:   "update LastFailureAt to a non-empty timestamp",
		Actual:   entry.LastFailureAt != "",
		Expected: true,
	})
}

func TestRerunFailureWarning(t *testing.T) {
	msg := rerunFailureWarning("should apply discount", "npx")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a rerun command that fails to execute",
		Should:   "include the test name in the warning",
		Actual:   strings.Contains(msg, `"should apply discount"`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a rerun command that fails to execute",
		Should:   "include the rerun command in the warning",
		Actual:   strings.Contains(msg, "npx exited with error"),
		Expected: true,
	})
}
