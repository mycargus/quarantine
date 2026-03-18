package quarantine_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)

// ptr is a helper to take the address of an int literal in tests.
func ptr(n int) *int { return &n }

func TestNewEmptyState(t *testing.T) {
	s := quarantine.NewEmptyState()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a freshly created empty state",
		Should:   "have version 1",
		Actual:   s.Version,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a freshly created empty state",
		Should:   "have a non-empty updated_at timestamp",
		Actual:   s.UpdatedAt != "",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a freshly created empty state",
		Should:   "have zero quarantined tests",
		Actual:   len(s.Tests),
		Expected: 0,
	})
}

func TestParseState(t *testing.T) {
	const raw = `{
		"version": 1,
		"updated_at": "2026-03-14T12:00:00Z",
		"tests": {
			"spec/auth_spec.rb::AuthService::logs in": {
				"test_id":        "spec/auth_spec.rb::AuthService::logs in",
				"file_path":      "spec/auth_spec.rb",
				"classname":      "AuthService",
				"name":           "logs in",
				"suite":          "AuthService",
				"first_flaky_at": "2026-03-10T08:30:00Z",
				"last_flaky_at":  "2026-03-14T11:45:00Z",
				"flaky_count":    7,
				"issue_number":   42,
				"issue_url":      "https://github.com/org/repo/issues/42",
				"quarantined_at": "2026-03-10T08:30:00Z",
				"quarantined_by": "cli-auto"
			}
		}
	}`

	s, err := quarantine.ParseState(strings.NewReader(raw))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a valid quarantine.json payload",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "the parsed state",
		Should:   "contain one quarantined test",
		Actual:   len(s.Tests),
		Expected: 1,
	})

	entry := s.Tests["spec/auth_spec.rb::AuthService::logs in"]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct test_id",
		Actual:   entry.TestID,
		Expected: "spec/auth_spec.rb::AuthService::logs in",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct suite",
		Actual:   entry.Suite,
		Expected: "AuthService",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct first_flaky_at",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-10T08:30:00Z",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct last_flaky_at",
		Actual:   entry.LastFlakyAt,
		Expected: "2026-03-14T11:45:00Z",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "the parsed entry",
		Should:   "have the correct flaky_count",
		Actual:   entry.FlakyCount,
		Expected: 7,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the parsed entry",
		Should:   "have a non-nil issue_number",
		Actual:   entry.IssueNumber != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "the parsed entry",
		Should:   "have the correct issue_number",
		Actual:   *entry.IssueNumber,
		Expected: 42,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct issue_url",
		Actual:   entry.IssueURL,
		Expected: "https://github.com/org/repo/issues/42",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the parsed entry",
		Should:   "have the correct quarantined_by",
		Actual:   entry.QuarantinedBy,
		Expected: "cli-auto",
	})
}

func TestParseStateWithoutOptionalIssueFields(t *testing.T) {
	// Represents an entry written before GitHub Issue creation completes
	// (degraded mode or two-phase write).
	const raw = `{
		"version": 1,
		"updated_at": "2026-03-14T12:00:00Z",
		"tests": {
			"spec/auth_spec.rb::AuthService::logs in": {
				"test_id":        "spec/auth_spec.rb::AuthService::logs in",
				"file_path":      "spec/auth_spec.rb",
				"classname":      "AuthService",
				"name":           "logs in",
				"suite":          "AuthService",
				"first_flaky_at": "2026-03-14T12:00:00Z",
				"last_flaky_at":  "2026-03-14T12:00:00Z",
				"flaky_count":    1,
				"quarantined_at": "2026-03-14T12:00:00Z",
				"quarantined_by": "cli-auto"
			}
		}
	}`

	s, err := quarantine.ParseState(strings.NewReader(raw))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a quarantine.json entry without issue_number or issue_url",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	entry := s.Tests["spec/auth_spec.rb::AuthService::logs in"]

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an entry with no issue_number in JSON",
		Should:   "have a nil IssueNumber pointer",
		Actual:   entry.IssueNumber == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "an entry with no issue_url in JSON",
		Should:   "have an empty IssueURL string",
		Actual:   entry.IssueURL,
		Expected: "",
	})
}

func TestMarshalRoundTrip(t *testing.T) {
	s := quarantine.NewEmptyState()
	num := 99
	s.AddTest(quarantine.Entry{
		TestID:        "src/foo.test.js::Foo::bar",
		FilePath:      "src/foo.test.js",
		Classname:     "Foo",
		Name:          "bar",
		Suite:         "Foo",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    3,
		IssueNumber:   &num,
		IssueURL:      "https://github.com/org/repo/issues/99",
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	data, err := s.Marshal()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a state with one fully-populated entry",
		Should:   "marshal without error",
		Actual:   err,
		Expected: nil,
	})

	parsed, err := quarantine.ParseState(strings.NewReader(string(data)))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "marshaled JSON from a valid state",
		Should:   "parse back without error",
		Actual:   err,
		Expected: nil,
	})

	entry := parsed.Tests["src/foo.test.js::Foo::bar"]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the round-tripped entry",
		Should:   "preserve suite",
		Actual:   entry.Suite,
		Expected: "Foo",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the round-tripped entry",
		Should:   "preserve first_flaky_at",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-01T00:00:00Z",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the round-tripped entry",
		Should:   "preserve last_flaky_at",
		Actual:   entry.LastFlakyAt,
		Expected: "2026-03-14T00:00:00Z",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the round-tripped entry",
		Should:   "preserve quarantined_by",
		Actual:   entry.QuarantinedBy,
		Expected: "cli-auto",
	})
}

func TestMergeUnionSemantics(t *testing.T) {
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:       "a::b::c",
		FilePath:     "a",
		Classname:    "b",
		Name:         "c",
		Suite:        "b",
		FirstFlakyAt: "2026-03-01T00:00:00Z",
		LastFlakyAt:  "2026-03-10T00:00:00Z",
		FlakyCount:   3,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:       "d::e::f",
		FilePath:     "d",
		Classname:    "e",
		Name:         "f",
		Suite:        "e",
		FirstFlakyAt: "2026-03-05T00:00:00Z",
		LastFlakyAt:  "2026-03-12T00:00:00Z",
		FlakyCount:   2,
		QuarantinedAt: "2026-03-05T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "local and remote each with one distinct test",
		Should:   "merge to two quarantined tests",
		Actual:   len(merged.Tests),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the local-only test",
		Should:   "be present in the merged state",
		Actual:   merged.HasTest("a::b::c"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the remote-only test",
		Should:   "be present in the merged state",
		Actual:   merged.HasTest("d::e::f"),
		Expected: true,
	})
}

func TestMergeConflictKeepsHigherFlakyCount(t *testing.T) {
	// Same test exists in both states with different flaky counts.
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:       "a::b::c",
		FilePath:     "a",
		Classname:    "b",
		Name:         "c",
		Suite:        "b",
		FirstFlakyAt: "2026-03-05T00:00:00Z",
		LastFlakyAt:  "2026-03-14T00:00:00Z",
		FlakyCount:   10,
		QuarantinedAt: "2026-03-05T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:       "a::b::c",
		FilePath:     "a",
		Classname:    "b",
		Name:         "c",
		Suite:        "b",
		FirstFlakyAt: "2026-03-01T00:00:00Z",
		LastFlakyAt:  "2026-03-10T00:00:00Z",
		FlakyCount:   5,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)

	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a conflict where local has flaky_count 10 and remote has 5",
		Should:   "keep the higher flaky_count (10)",
		Actual:   entry.FlakyCount,
		Expected: 10,
	})
}

func TestMergeConflictPreservesEarliestFirstFlakyAt(t *testing.T) {
	// Local has the higher flaky count but a later first_flaky_at.
	// Remote has the earlier first_flaky_at.
	// Merge should keep local's flaky_count but remote's first_flaky_at.
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:       "a::b::c",
		FilePath:     "a",
		Classname:    "b",
		Name:         "c",
		Suite:        "b",
		FirstFlakyAt: "2026-03-05T00:00:00Z",
		LastFlakyAt:  "2026-03-14T00:00:00Z",
		FlakyCount:   10,
		QuarantinedAt: "2026-03-05T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:       "a::b::c",
		FilePath:     "a",
		Classname:    "b",
		Name:         "c",
		Suite:        "b",
		FirstFlakyAt: "2026-03-01T00:00:00Z",
		LastFlakyAt:  "2026-03-10T00:00:00Z",
		FlakyCount:   5,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)

	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a conflict where remote has an earlier first_flaky_at",
		Should:   "preserve the earliest first_flaky_at (remote's 2026-03-01)",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-01T00:00:00Z",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a conflict where local has the higher flaky_count",
		Should:   "still keep the higher flaky_count (10)",
		Actual:   entry.FlakyCount,
		Expected: 10,
	})
}
