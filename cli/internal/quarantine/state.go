// Package quarantine handles reading, writing, and merging quarantine.json
// state files stored on the quarantine/state branch.
package quarantine

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// State represents the quarantine.json file stored on the quarantine/state
// branch.
type State struct {
	Version   int              `json:"version"`
	UpdatedAt string           `json:"updated_at"`
	Tests     map[string]Entry `json:"tests"`
}

// Entry represents a single quarantined test entry in quarantine.json.
// The map key is the test_id.
//
// IssueNumber and IssueURL are optional at the Go level even though the JSON
// schema marks them as required. A newly detected flaky test is written to
// quarantine.json before GitHub Issue creation, so an entry may exist briefly
// without an issue. If issue creation fails (degraded mode), the entry persists
// without an issue number — a subsequent successful run will backfill it.
// This follows the "never break the build" principle (CLAUDE.md, error-handling.md).
type Entry struct {
	TestID          string `json:"test_id"`
	FilePath        string `json:"file_path"`
	Classname       string `json:"classname"`
	Name            string `json:"name"`
	Suite           string `json:"suite"`
	FirstFlakyAt    string `json:"first_flaky_at"`
	LastFlakyAt     string `json:"last_flaky_at"`
	FlakyCount      int    `json:"flaky_count"`
	IssueNumber     *int   `json:"issue_number,omitempty"`
	IssueURL        string `json:"issue_url,omitempty"`
	QuarantinedAt   string `json:"quarantined_at"`
	QuarantinedBy   string `json:"quarantined_by"`
}

// NewEmptyStateAt returns an empty quarantine state with the given timestamp.
// This is a pure function — no I/O.
func NewEmptyStateAt(timestamp string) *State {
	return &State{
		Version:   1,
		UpdatedAt: timestamp,
		Tests:     make(map[string]Entry),
	}
}

// NewEmptyState returns an empty quarantine state suitable for initial
// creation.
func NewEmptyState() *State {
	return NewEmptyStateAt(time.Now().UTC().Format(time.RFC3339))
}

// ParseState reads and parses quarantine.json from a reader.
func ParseState(r io.Reader) (*State, error) {
	var state State
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to parse quarantine.json: %w", err)
	}
	return &state, nil
}

// MarshalAt serializes the state to JSON using the given timestamp.
// This is a pure function — no I/O.
func (s *State) MarshalAt(timestamp string) ([]byte, error) {
	snapshot := *s
	snapshot.UpdatedAt = timestamp
	return json.MarshalIndent(&snapshot, "", "  ")
}

// Marshal serializes the state to JSON.
func (s *State) Marshal() ([]byte, error) {
	return s.MarshalAt(time.Now().UTC().Format(time.RFC3339))
}

// AddTest adds or updates a quarantined test entry.
func (s *State) AddTest(entry Entry) {
	s.Tests[entry.TestID] = entry
}

// RemoveTest removes a test from the quarantine state.
func (s *State) RemoveTest(testID string) {
	delete(s.Tests, testID)
}

// HasTest checks whether a test is currently quarantined.
func (s *State) HasTest(testID string) bool {
	_, ok := s.Tests[testID]
	return ok
}

// QuarantinedTestIDs returns a slice of all quarantined test IDs.
func (s *State) QuarantinedTestIDs() []string {
	ids := make([]string, 0, len(s.Tests))
	for id := range s.Tests {
		ids = append(ids, id)
	}
	return ids
}

// MergeAt combines two states using the given timestamp. Pure function — no I/O.
// Per ADR-012, quarantine wins on conflict: if a test is quarantined in
// either state, it remains quarantined in the merged result.
func MergeAt(local, remote *State, timestamp string) *State {
	merged := &State{
		Version:   1,
		UpdatedAt: timestamp,
		Tests:     make(map[string]Entry),
	}

	// Start with remote entries.
	for id, entry := range remote.Tests {
		merged.Tests[id] = entry
	}

	// Overlay local entries. Local wins for individual fields, but both
	// sides contribute their quarantined tests (union).
	for id, entry := range local.Tests {
		if existing, ok := merged.Tests[id]; ok {
			// Keep the higher flaky count; that entry also carries the
			// most recent last_flaky_at.
			if entry.FlakyCount > existing.FlakyCount {
				merged.Tests[id] = entry
			}
			// Preserve the earliest first_flaky_at across both sides.
			current := merged.Tests[id]
			earliest := current.FirstFlakyAt
			if existing.FirstFlakyAt != "" &&
				(earliest == "" || existing.FirstFlakyAt < earliest) {
				earliest = existing.FirstFlakyAt
			}
			if entry.FirstFlakyAt != "" &&
				(earliest == "" || entry.FirstFlakyAt < earliest) {
				earliest = entry.FirstFlakyAt
			}
			if earliest != current.FirstFlakyAt {
				current.FirstFlakyAt = earliest
				merged.Tests[id] = current
			}
		} else {
			merged.Tests[id] = entry
		}
	}

	return merged
}

// Merge combines two states, producing a union of quarantined tests.
// Per ADR-012, quarantine wins on conflict: if a test is quarantined in
// either state, it remains quarantined in the merged result.
func Merge(local, remote *State) *State {
	return MergeAt(local, remote, time.Now().UTC().Format(time.RFC3339))
}
