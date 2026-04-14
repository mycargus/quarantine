package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/mycargus/quarantine/cli/internal/github"
	"github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/spf13/cobra"
)

// statusEntry is a pure data type holding the fields needed for status display.
// No I/O — deterministic.
type statusEntry struct {
	TestID        string
	Name          string
	QuarantinedAt string
	LastFailureAt string
	IssueNumber   *int
}

// averageDurationMs computes the average of a slice of millisecond durations.
// Returns nil when the slice is empty. This is a pure function — no I/O.
func averageDurationMs(durations []int64) *int64 {
	if len(durations) == 0 {
		return nil
	}
	var sum int64
	for _, d := range durations {
		sum += d
	}
	avg := sum / int64(len(durations))
	return &avg
}

// formatDuration converts milliseconds to a string like "4.2s".
// This is a pure function — no I/O, deterministic.
func formatDuration(ms int64) string {
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.1fs", seconds)
}

// daysBetween returns the number of full days between t and now.
// This is a pure function — no I/O, deterministic.
func daysBetween(t, now time.Time) int {
	diff := now.Sub(t)
	return int(diff.Hours() / 24)
}

// computeStatusText formats the full status output string for a suite.
// This is a pure function — no I/O, deterministic.
// entries should already be sorted oldest-first by QuarantinedAt.
func computeStatusText(suiteName string, entries []statusEntry, avgDurationMs *int64, now time.Time) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Suite: %s\n", suiteName)
	fmt.Fprintf(&sb, "Quarantined tests: %d\n", len(entries))

	if avgDurationMs != nil {
		avgSec := float64(*avgDurationMs) / 1000.0
		totalSec := avgSec * float64(len(entries))
		fmt.Fprintf(&sb, "Avg quarantined test duration: %s (from last 10 runs)\n", formatDuration(*avgDurationMs))
		fmt.Fprintf(&sb, "Estimated CI time per run on quarantined tests: ~%.1fs\n", totalSec)
	}

	sb.WriteString("\nOldest quarantined (consider closing if fixed):\n")
	for _, e := range entries {
		qt, err := time.Parse(time.RFC3339, e.QuarantinedAt)
		if err != nil {
			qt = now
		}
		lf, err := time.Parse(time.RFC3339, e.LastFailureAt)
		if err != nil {
			lf = now
		}
		days := daysBetween(qt, now)
		lastFailed := daysBetween(lf, now)

		issueStr := ""
		if e.IssueNumber != nil {
			issueStr = fmt.Sprintf("#%d", *e.IssueNumber)
		}

		fmt.Fprintf(&sb, "  %-60s (%s, %d days, last failed %d days ago)\n",
			e.Name, issueStr, days, lastFailed)
	}

	return sb.String()
}

// parseResultsJSON extracts duration_ms values for quarantined tests from
// artifact results.json bytes. This is a pure function — no I/O, deterministic.
func parseResultsJSON(data []byte) []int64 {
	var results struct {
		Tests []struct {
			Status     string `json:"status"`
			DurationMs int64  `json:"duration_ms"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}
	var durations []int64
	for _, t := range results.Tests {
		if t.Status == "quarantined" {
			durations = append(durations, t.DurationMs)
		}
	}
	return durations
}

// extractResultsFromZip reads results.json from a ZIP archive bytes and returns
// the raw JSON bytes. This is a pure function — no I/O (operates on in-memory bytes).
func extractResultsFromZip(zipData []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range r.File {
		if f.Name == "results.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open results.json in zip: %w", err)
			}
			defer func() { _ = rc.Close() }()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				return nil, fmt.Errorf("read results.json: %w", err)
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("results.json not found in artifact zip")
}

// newStatusCmd returns the "status" cobra command.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [suite-name]",
		Short: "Show quarantine status for a suite",
		Long: `Show quarantine status for a suite. Reads quarantined tests from the
quarantine/state branch and artifact data for duration estimates.`,
		Args: cobra.ExactArgs(1),
		RunE: runStatus,
	}
}

// runStatus is the I/O shell for "status [suite-name]". Reads state from GitHub
// and artifact data, then prints formatted status output.
func runStatus(cmd *cobra.Command, args []string) error {
	suiteName := args[0]

	cfg, err := config.Load(".quarantine/config.yml")
	if err != nil {
		return err
	}

	owner, repo := resolveOwnerRepo(cfg)

	gh, err := github.NewClient(owner, repo)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Read state file from quarantine/state branch.
	stateBytes, _, err := gh.GetContents(ctx, fmt.Sprintf(".quarantine/%s/state.json", suiteName), "quarantine/state")
	if err != nil {
		return fmt.Errorf("read state for suite %q: %w", suiteName, err)
	}

	var state quarantine.State
	if len(stateBytes) > 0 {
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			return fmt.Errorf("parse state for suite %q: %w", suiteName, err)
		}
	} else {
		state.Tests = make(map[string]quarantine.Entry)
	}

	// Build status entries sorted oldest-first by quarantined_at.
	entries := make([]statusEntry, 0, len(state.Tests))
	for _, e := range state.Tests {
		se := statusEntry{
			TestID:        e.TestID,
			Name:          e.Name,
			QuarantinedAt: e.QuarantinedAt,
			LastFailureAt: e.LastFailureAt,
			IssueNumber:   e.IssueNumber,
		}
		entries = append(entries, se)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].QuarantinedAt < entries[j].QuarantinedAt
	})

	// Fetch artifact duration data.
	artifactName := fmt.Sprintf("quarantine-results-%s", suiteName)
	artifacts, err := gh.ListArtifacts(ctx, artifactName, 10)
	if err != nil {
		// Non-fatal: proceed without duration data.
		artifacts = nil
	}

	var allDurations []int64
	for _, a := range artifacts {
		zipData, err := gh.DownloadArtifactZip(ctx, a.ArchiveDownloadURL)
		if err != nil {
			continue
		}
		resultsJSON, err := extractResultsFromZip(zipData)
		if err != nil {
			continue
		}
		durations := parseResultsJSON(resultsJSON)
		allDurations = append(allDurations, durations...)
	}

	// Compute per-test average across all artifacts.
	// Each artifact contributes duration per quarantined test; we want an
	// average per-test duration across all runs.
	var avgMs *int64
	if len(allDurations) > 0 {
		avg := averageDurationMs(allDurations)
		avgMs = avg
	}

	now := time.Now().UTC()
	output := computeStatusText(suiteName, entries, avgMs, now)
	cmd.Print(output)
	return nil
}
