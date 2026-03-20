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
