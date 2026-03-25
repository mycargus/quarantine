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
		Should:   "update LastFlakyAt to a non-empty timestamp",
		Actual:   entry.LastFlakyAt != "",
		Expected: true,
	})
}

