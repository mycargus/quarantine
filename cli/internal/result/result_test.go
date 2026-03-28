package result_test

import (
	"testing"

	"github.com/mycargus/quarantine/internal/parser"
	"github.com/mycargus/quarantine/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)

func TestComputeSummary(t *testing.T) {
	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "empty test slice",
		Should:   "return zero summary",
		Actual:   result.ComputeSummary([]parser.TestResult{}),
		Expected: result.Summary{},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "all passed tests",
		Should:   "count all as passed",
		Actual:   result.ComputeSummary([]parser.TestResult{{Status: "passed"}, {Status: "passed"}}),
		Expected: result.Summary{Total: 2, Passed: 2},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "mixed passed/failed/skipped",
		Should:   "tally each status separately",
		Actual: result.ComputeSummary([]parser.TestResult{
			{Status: "passed"},
			{Status: "failed"},
			{Status: "skipped"},
		}),
		Expected: result.Summary{Total: 3, Passed: 1, Failed: 1, Skipped: 1},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "error status test",
		Should:   "count error as failed",
		Actual:   result.ComputeSummary([]parser.TestResult{{Status: "error"}}),
		Expected: result.Summary{Total: 1, Failed: 1},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "unknown status",
		Should:   "not count in any named category",
		Actual:   result.ComputeSummary([]parser.TestResult{{Status: "unknown"}}),
		Expected: result.Summary{Total: 1},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "all failed tests",
		Should:   "count all as failed",
		Actual:   result.ComputeSummary([]parser.TestResult{{Status: "failed"}, {Status: "failed"}, {Status: "failed"}}),
		Expected: result.Summary{Total: 3, Failed: 3},
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "all skipped tests",
		Should:   "count all as skipped",
		Actual:   result.ComputeSummary([]parser.TestResult{{Status: "skipped"}, {Status: "skipped"}}),
		Expected: result.Summary{Total: 2, Skipped: 2},
	})
}

func TestBuildEmptyTests(t *testing.T) {
	meta := result.Metadata{
		RunID:     "run-1",
		Repo:      "owner/repo",
		Framework: "jest",
	}

	res := result.Build([]parser.TestResult{}, meta)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Build called with empty test slice",
		Should:   "return Summary.Total == 0",
		Actual:   res.Summary.Total,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Build called with empty test slice",
		Should:   "return empty Tests slice",
		Actual:   len(res.Tests),
		Expected: 0,
	})
}

func TestBuildAtUsesProvidedTimestamp(t *testing.T) {
	meta := result.Metadata{
		RunID: "run-1",
		Repo:  "owner/repo",
	}

	res := result.BuildAt([]parser.TestResult{}, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "BuildAt called with timestamp 2026-01-15T10:00:00Z",
		Should:   "use the provided timestamp",
		Actual:   res.Timestamp,
		Expected: "2026-01-15T10:00:00Z",
	})
}

func TestBuildWithPRNumberNil(t *testing.T) {
	meta := result.Metadata{
		RunID:    "run-nil",
		Repo:     "owner/repo",
		PRNumber: nil,
	}

	res := result.BuildAt([]parser.TestResult{}, meta, "2026-01-01T00:00:00Z")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "BuildAt called with PRNumber nil",
		Should:   "set PRNumber to nil on the result",
		Actual:   res.PRNumber == nil,
		Expected: true,
	})
}

func TestBuildWithPRNumber(t *testing.T) {
	n := 42
	meta := result.Metadata{
		RunID:    "run-2",
		Repo:     "owner/repo",
		PRNumber: &n,
	}

	res := result.Build([]parser.TestResult{}, meta)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Build called with PRNumber set to 42",
		Should:   "set a non-nil PRNumber on the result",
		Actual:   res.PRNumber != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Build called with PRNumber set to 42",
		Should:   "set PRNumber to 42",
		Actual:   *res.PRNumber,
		Expected: 42,
	})
}

func TestBuildAtWithRetries_FlakyDetection(t *testing.T) {
	// Test 1: fails initially, passes on first retry → "flaky"
	failMsg := "assertion failed"
	tests := []parser.TestResult{
		{
			TestID:         "src/foo.test.ts::Suite::flaky_test",
			FilePath:       "src/foo.test.ts",
			Classname:      "Suite",
			Name:           "flaky_test",
			Status:         "failed",
			DurationMs:     100,
			FailureMessage: &failMsg,
		},
	}
	retries := map[string]result.RetryOutcome{
		"src/foo.test.ts::Suite::flaky_test": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "passed", DurationMs: 90},
			},
		},
	}
	meta := result.Metadata{RunID: "run-1", Repo: "owner/repo"}

	res := result.BuildAtWithRetries(tests, retries, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test that fails initially and passes on retry",
		Should:   "classify the test as flaky",
		Actual:   res.Tests[0].Status,
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a test that fails initially and passes on retry",
		Should:   "set OriginalStatus to the initial status",
		Actual:   res.Tests[0].OriginalStatus != nil && *res.Tests[0].OriginalStatus == "failed",
		Expected: true,
	})

	// Test 2: fails initially, fails all retries → stays "failed"
	tests2 := []parser.TestResult{
		{
			TestID:         "src/bar.test.ts::Suite::always_fails",
			FilePath:       "src/bar.test.ts",
			Classname:      "Suite",
			Name:           "always_fails",
			Status:         "failed",
			DurationMs:     200,
			FailureMessage: &failMsg,
		},
	}
	retries2 := map[string]result.RetryOutcome{
		"src/bar.test.ts::Suite::always_fails": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "failed", DurationMs: 190},
				{Attempt: 2, Status: "failed", DurationMs: 195},
			},
		},
	}

	res2 := result.BuildAtWithRetries(tests2, retries2, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test that fails initially and fails all retries",
		Should:   "keep the test status as failed",
		Actual:   res2.Tests[0].Status,
		Expected: "failed",
	})

	// Test 3: passes initially → no retry processing, OriginalStatus nil
	tests3 := []parser.TestResult{
		{
			TestID:    "src/baz.test.ts::Suite::always_passes",
			FilePath:  "src/baz.test.ts",
			Classname: "Suite",
			Name:      "always_passes",
			Status:    "passed",
			DurationMs: 50,
		},
	}

	res3 := result.BuildAtWithRetries(tests3, map[string]result.RetryOutcome{}, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test that passes initially with no retry entry",
		Should:   "keep the test status as passed",
		Actual:   res3.Tests[0].Status,
		Expected: "passed",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a test that passes initially with no retry entry",
		Should:   "leave OriginalStatus as nil",
		Actual:   res3.Tests[0].OriginalStatus == nil,
		Expected: true,
	})
}

func TestBuildAtWithRetries_SummaryCounting(t *testing.T) {
	failMsg := "oops"
	tests := []parser.TestResult{
		{TestID: "t1", Status: "passed"},
		{TestID: "t2", Status: "failed", FailureMessage: &failMsg},
		{TestID: "t3", Status: "failed", FailureMessage: &failMsg},
		{TestID: "t4", Status: "skipped"},
		{TestID: "t5", Status: "quarantined"},
	}
	// t2 gets retried and passes → flaky; t3 gets retried and fails → still failed
	retries := map[string]result.RetryOutcome{
		"t2": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "passed", DurationMs: 80},
			},
		},
		"t3": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "failed", DurationMs: 100},
			},
		},
	}
	meta := result.Metadata{RunID: "run-sum", Repo: "owner/repo"}

	res := result.BuildAtWithRetries(tests, retries, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "mixed test statuses with one retry-pass and one retry-fail",
		Should:   "count summary fields correctly",
		Actual:   res.Summary,
		Expected: result.Summary{
			Total:         5,
			Passed:        1,
			Failed:        1,
			Skipped:       1,
			Quarantined:   1,
			FlakyDetected: 1,
		},
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "five test results",
		Should:   "set Total to the number of tests",
		Actual:   res.Summary.Total,
		Expected: 5,
	})
}

func TestBuildAtWithRetries_BreakOnFirstPassedRetry(t *testing.T) {
	// Multiple attempts where first is "passed" — should classify as flaky exactly once.
	// This kills the break mutation: without break, the loop continues and passedOnRetry
	// stays true (no double-counting), but the break is required to stop processing.
	// We verify via summary that FlakyDetected is exactly 1.
	failMsg := "fail"
	tests := []parser.TestResult{
		{TestID: "t-break", Status: "failed", FailureMessage: &failMsg},
	}
	retries := map[string]result.RetryOutcome{
		"t-break": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "passed", DurationMs: 70},
				{Attempt: 2, Status: "failed", DurationMs: 80},
				{Attempt: 3, Status: "passed", DurationMs: 65},
			},
		},
	}
	meta := result.Metadata{RunID: "run-break", Repo: "owner/repo"}

	res := result.BuildAtWithRetries(tests, retries, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test with multiple retries where first retry passes",
		Should:   "classify the test as flaky",
		Actual:   res.Tests[0].Status,
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a test with multiple retries where first retry passes",
		Should:   "count exactly one flaky detection",
		Actual:   res.Summary.FlakyDetected,
		Expected: 1,
	})
}

func TestBuildAtWithRetries_VersionField(t *testing.T) {
	meta := result.Metadata{RunID: "run-v", Repo: "owner/repo"}

	res := result.BuildAtWithRetries([]parser.TestResult{}, map[string]result.RetryOutcome{}, meta, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "BuildAtWithRetries called",
		Should:   "set Version to 1",
		Actual:   res.Version,
		Expected: 1,
	})
}

func TestBuildAtWithRetries_NoRetryEntry(t *testing.T) {
	// When retries map has no entry for a test, the test is not retried and
	// OriginalStatus remains nil.
	tests := []parser.TestResult{
		{TestID: "t-no-retry", Status: "failed"},
	}

	res := result.BuildAtWithRetries(tests, map[string]result.RetryOutcome{}, result.Metadata{}, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a failed test with no retry entry in the retries map",
		Should:   "leave OriginalStatus as nil",
		Actual:   res.Tests[0].OriginalStatus == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a failed test with no retry entry in the retries map",
		Should:   "preserve the original failed status",
		Actual:   res.Tests[0].Status,
		Expected: "failed",
	})
}

func TestBuildAtWithRetries_EmptyAttemptsSlice(t *testing.T) {
	// When retries map has an entry but Attempts is empty, the ok && len > 0 guard
	// must prevent retry processing. This kills the len(outcome.Attempts) > 0 mutation.
	tests := []parser.TestResult{
		{TestID: "t-empty-attempts", Status: "failed"},
	}
	retries := map[string]result.RetryOutcome{
		"t-empty-attempts": {Attempts: []result.RetryEntry{}},
	}

	res := result.BuildAtWithRetries(tests, retries, result.Metadata{}, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a retry entry with an empty attempts slice",
		Should:   "leave OriginalStatus as nil (no retry processing)",
		Actual:   res.Tests[0].OriginalStatus == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a retry entry with an empty attempts slice",
		Should:   "preserve the original failed status",
		Actual:   res.Tests[0].Status,
		Expected: "failed",
	})
}

func TestBuildAtWithRetries_OriginalStatusSetOnRetry(t *testing.T) {
	// OriginalStatus must be set to the initial test status when a retry entry exists.
	tests := []parser.TestResult{
		{TestID: "t-orig", Status: "failed"},
	}
	retries := map[string]result.RetryOutcome{
		"t-orig": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "failed", DurationMs: 100},
			},
		},
	}

	res := result.BuildAtWithRetries(tests, retries, result.Metadata{}, "2026-01-15T10:00:00Z")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a test that is retried",
		Should:   "set OriginalStatus to non-nil",
		Actual:   res.Tests[0].OriginalStatus != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test that is retried",
		Should:   "set OriginalStatus to the initial test status",
		Actual:   *res.Tests[0].OriginalStatus,
		Expected: "failed",
	})
}

func TestBuild(t *testing.T) {
	msg := "assertion failed"
	tests := []parser.TestResult{
		{
			TestID:         "src/foo.test.ts::Suite::passes",
			FilePath:       "src/foo.test.ts",
			Classname:      "Suite",
			Name:           "passes",
			Status:         "passed",
			DurationMs:     100,
			FailureMessage: nil,
		},
		{
			TestID:         "src/bar.test.ts::Suite::fails",
			FilePath:       "src/bar.test.ts",
			Classname:      "Suite",
			Name:           "fails",
			Status:         "failed",
			DurationMs:     200,
			FailureMessage: &msg,
		},
	}

	meta := result.Metadata{
		RunID:      "run-123",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 2,
	}

	res := result.Build(tests, meta)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two test results and metadata",
		Should:   "set version to 1",
		Actual:   res.Version,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "metadata with run_id run-123",
		Should:   "set run_id from metadata",
		Actual:   res.RunID,
		Expected: "run-123",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "metadata with repo owner/repo",
		Should:   "set repo from metadata",
		Actual:   res.Repo,
		Expected: "owner/repo",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "metadata with framework jest",
		Should:   "set framework from metadata",
		Actual:   res.Framework,
		Expected: "jest",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "metadata with retry_count 2",
		Should:   "set config.retry_count from metadata",
		Actual:   res.Config.RetryCount,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "one passed and one failed test",
		Should:   "compute summary correctly",
		Actual:   res.Summary,
		Expected: result.Summary{Total: 2, Passed: 1, Failed: 1},
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two test results",
		Should:   "include all tests in tests array",
		Actual:   len(res.Tests),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "first test result",
		Should:   "map test_id correctly",
		Actual:   res.Tests[0].TestID,
		Expected: "src/foo.test.ts::Suite::passes",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "failed test result with failure message",
		Should:   "preserve failure message",
		Actual:   *res.Tests[1].FailureMessage,
		Expected: "assertion failed",
	})
}
