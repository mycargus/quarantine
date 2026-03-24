// Package result builds the structured JSON output for .quarantine/results.json.
package result

import (
	"time"

	"github.com/mycargus/quarantine/internal/parser"
)

// Result is the top-level structure for results.json.
type Result struct {
	Version    int        `json:"version"`
	RunID      string     `json:"run_id"`
	Repo       string     `json:"repo"`
	Branch     string     `json:"branch"`
	CommitSHA  string     `json:"commit_sha"`
	PRNumber   *int       `json:"pr_number"`
	Timestamp  string     `json:"timestamp"`
	CLIVersion string     `json:"cli_version"`
	Framework  string     `json:"framework"`
	Config     ConfigInfo `json:"config"`
	Summary    Summary    `json:"summary"`
	Tests      []TestEntry `json:"tests"`
}

// ConfigInfo records which config values were active.
type ConfigInfo struct {
	RetryCount int `json:"retry_count"`
}

// Summary holds aggregate counts.
type Summary struct {
	Total         int `json:"total"`
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Skipped       int `json:"skipped"`
	Quarantined   int `json:"quarantined"`
	FlakyDetected int `json:"flaky_detected"`
}

// RetryEntry records the outcome of a single retry attempt.
type RetryEntry struct {
	Attempt    int    `json:"attempt"`
	Status     string `json:"status"`
	DurationMs int    `json:"duration_ms"`
}

// TestEntry holds the result for a single test case.
type TestEntry struct {
	TestID         string       `json:"test_id"`
	FilePath       string       `json:"file_path"`
	Classname      string       `json:"classname"`
	Name           string       `json:"name"`
	Status         string       `json:"status"`
	OriginalStatus *string      `json:"original_status"`
	DurationMs     int          `json:"duration_ms"`
	FailureMessage *string      `json:"failure_message"`
	IssueNumber    *int         `json:"issue_number"`
	Retries        []RetryEntry `json:"retries,omitempty"`
}

// Metadata holds run-level metadata used to build the result.
type Metadata struct {
	RunID      string
	Repo       string
	Branch     string
	CommitSHA  string
	PRNumber   *int
	CLIVersion string
	Framework  string
	RetryCount int
}

// BuildAt constructs a Result from parsed test results, metadata, and a timestamp.
// This is a pure function — no I/O.
func BuildAt(tests []parser.TestResult, meta Metadata, timestamp string) Result {
	entries := make([]TestEntry, len(tests))
	summary := ComputeSummary(tests)

	for i, t := range tests {
		entries[i] = TestEntry{
			TestID:         t.TestID,
			FilePath:       t.FilePath,
			Classname:      t.Classname,
			Name:           t.Name,
			Status:         t.Status,
			OriginalStatus: nil,
			DurationMs:     t.DurationMs,
			FailureMessage: t.FailureMessage,
			IssueNumber:    nil,
		}
	}

	return Result{
		Version:    1,
		RunID:      meta.RunID,
		Repo:       meta.Repo,
		Branch:     meta.Branch,
		CommitSHA:  meta.CommitSHA,
		PRNumber:   meta.PRNumber,
		Timestamp:  timestamp,
		CLIVersion: meta.CLIVersion,
		Framework:  meta.Framework,
		Config: ConfigInfo{
			RetryCount: meta.RetryCount,
		},
		Summary: summary,
		Tests:   entries,
	}
}

// Build constructs a Result from parsed test results and metadata.
func Build(tests []parser.TestResult, meta Metadata) Result {
	return BuildAt(tests, meta, time.Now().UTC().Format(time.RFC3339))
}

// RetryOutcome holds the retry attempts for a single test (keyed by TestID).
type RetryOutcome struct {
	Attempts []RetryEntry
}

// BuildAtWithRetries constructs a Result incorporating retry outcomes.
// Tests that failed initially but passed on any retry are classified as "flaky".
// Tests that failed all retries remain "failed".
// This is a pure function — no I/O.
func BuildAtWithRetries(tests []parser.TestResult, retries map[string]RetryOutcome, meta Metadata, timestamp string) Result {
	entries := make([]TestEntry, len(tests))
	var summary Summary
	summary.Total = len(tests)

	for i, t := range tests {
		entry := TestEntry{
			TestID:         t.TestID,
			FilePath:       t.FilePath,
			Classname:      t.Classname,
			Name:           t.Name,
			Status:         t.Status,
			OriginalStatus: nil,
			DurationMs:     t.DurationMs,
			FailureMessage: t.FailureMessage,
			IssueNumber:    nil,
		}

		if outcome, ok := retries[t.TestID]; ok && len(outcome.Attempts) > 0 {
			orig := t.Status
			entry.OriginalStatus = &orig
			entry.Retries = outcome.Attempts

			// If any retry passed, classify as flaky.
			passedOnRetry := false
			for _, a := range outcome.Attempts {
				if a.Status == "passed" {
					passedOnRetry = true
					break
				}
			}
			if passedOnRetry {
				entry.Status = "flaky"
			}
		}

		switch entry.Status {
		case "passed":
			summary.Passed++
		case "failed", "error":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		case "flaky":
			summary.FlakyDetected++
		}

		entries[i] = entry
	}

	return Result{
		Version:    1,
		RunID:      meta.RunID,
		Repo:       meta.Repo,
		Branch:     meta.Branch,
		CommitSHA:  meta.CommitSHA,
		PRNumber:   meta.PRNumber,
		Timestamp:  timestamp,
		CLIVersion: meta.CLIVersion,
		Framework:  meta.Framework,
		Config: ConfigInfo{
			RetryCount: meta.RetryCount,
		},
		Summary: summary,
		Tests:   entries,
	}
}

// BuildWithRetries constructs a Result incorporating retry outcomes.
func BuildWithRetries(tests []parser.TestResult, retries map[string]RetryOutcome, meta Metadata) Result {
	return BuildAtWithRetries(tests, retries, meta, time.Now().UTC().Format(time.RFC3339))
}

// ComputeSummary tallies test results by status.
// This is a pure function — no I/O.
func ComputeSummary(tests []parser.TestResult) Summary {
	var s Summary
	s.Total = len(tests)
	for _, t := range tests {
		switch t.Status {
		case "passed":
			s.Passed++
		case "failed", "error":
			s.Failed++
		case "skipped":
			s.Skipped++
		}
	}
	return s
}
