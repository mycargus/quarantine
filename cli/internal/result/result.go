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

// TestEntry holds the result for a single test case.
type TestEntry struct {
	TestID         string  `json:"test_id"`
	FilePath       string  `json:"file_path"`
	Classname      string  `json:"classname"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	OriginalStatus *string `json:"original_status"`
	DurationMs     int     `json:"duration_ms"`
	FailureMessage *string `json:"failure_message"`
	IssueNumber    *int    `json:"issue_number"`
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

// Build constructs a Result from parsed test results and metadata.
// This is a pure function — no I/O.
func Build(tests []parser.TestResult, meta Metadata) Result {
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
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		CLIVersion: meta.CLIVersion,
		Framework:  meta.Framework,
		Config: ConfigInfo{
			RetryCount: meta.RetryCount,
		},
		Summary: summary,
		Tests:   entries,
	}
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
