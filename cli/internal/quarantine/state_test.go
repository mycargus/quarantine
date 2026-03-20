package quarantine_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)


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

func TestMergeConflictEqualFlakyCountKeepsRemote(t *testing.T) {
	// When flaky_count is equal, remote entry wins (local only overwrites when strictly greater).
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    5,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    5,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)

	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a conflict where local and remote both have flaky_count 5",
		Should:   "keep remote's entry (local does not override on equal count)",
		Actual:   entry.FlakyCount,
		Expected: 5,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a conflict where local and remote both have flaky_count 5",
		Should:   "keep remote's last_flaky_at",
		Actual:   entry.LastFlakyAt,
		Expected: "2026-03-14T00:00:00Z",
	})
}

func TestParseStateMalformedJSON(t *testing.T) {
	_, err := quarantine.ParseState(strings.NewReader(`{bad json`))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a malformed JSON string",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestQuarantinedTestIDsEmptyState(t *testing.T) {
	s := quarantine.NewEmptyState()

	ids := s.QuarantinedTestIDs()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a state with no quarantined tests",
		Should:   "return a non-nil empty slice",
		Actual:   ids != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a state with no quarantined tests",
		Should:   "return zero IDs",
		Actual:   len(ids),
		Expected: 0,
	})
}

func TestMergeConflictRemoteWinsOnCountButLocalHasEarlierFirstFlakyAt(t *testing.T) {
	// Remote wins the flaky-count check, but local has an earlier first_flaky_at.
	// Merge must still pull in local's earlier timestamp.
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    3,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-05T00:00:00Z",
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    10,
		QuarantinedAt: "2026-03-05T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a conflict where remote has flaky_count 10 and local has 3",
		Should:   "keep the higher flaky_count (10)",
		Actual:   entry.FlakyCount,
		Expected: 10,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a conflict where local has an earlier first_flaky_at (2026-03-01) despite losing the count",
		Should:   "preserve the earliest first_flaky_at (local's 2026-03-01)",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-01T00:00:00Z",
	})
}

func TestMergeConflictWinnerHasEmptyFirstFlakyAtLoserHasTimestamp(t *testing.T) {
	// Local wins the flaky-count check but has no first_flaky_at.
	// Remote has a first_flaky_at that should be preserved in the merged result.
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "",
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    10,
		QuarantinedAt: "2026-03-14T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    5,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a conflict where the winner (local) has an empty first_flaky_at and the loser (remote) has a timestamp",
		Should:   "use remote's first_flaky_at rather than leaving it empty",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-01T00:00:00Z",
	})
}

func TestMergeBothEmpty(t *testing.T) {
	local := quarantine.NewEmptyState()
	remote := quarantine.NewEmptyState()

	merged := quarantine.Merge(local, remote)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two empty states",
		Should:   "produce a merged state with zero tests",
		Actual:   len(merged.Tests),
		Expected: 0,
	})
}

func TestRemoveTest(t *testing.T) {
	s := quarantine.NewEmptyState()
	s.AddTest(quarantine.Entry{TestID: "a::b::c"})
	s.RemoveTest("a::b::c")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a state after AddTest then RemoveTest",
		Should:   "not contain the removed test",
		Actual:   s.HasTest("a::b::c"),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a state after AddTest then RemoveTest",
		Should:   "have zero tests",
		Actual:   len(s.Tests),
		Expected: 0,
	})
}

func TestHasTest(t *testing.T) {
	s := quarantine.NewEmptyState()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty state",
		Should:   "return false for HasTest on unknown ID",
		Actual:   s.HasTest("a::b::c"),
		Expected: false,
	})

	s.AddTest(quarantine.Entry{TestID: "a::b::c"})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a state after AddTest",
		Should:   "return true for HasTest on the added ID",
		Actual:   s.HasTest("a::b::c"),
		Expected: true,
	})
}

func TestQuarantinedTestIDsPopulated(t *testing.T) {
	s := quarantine.NewEmptyState()
	s.AddTest(quarantine.Entry{TestID: "a::b::c"})
	s.AddTest(quarantine.Entry{TestID: "d::e::f"})

	ids := s.QuarantinedTestIDs()

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a state with two quarantined tests",
		Should:   "return two IDs",
		Actual:   len(ids),
		Expected: 2,
	})

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a state with tests 'a::b::c' and 'd::e::f'",
		Should:   "include 'a::b::c' in the returned IDs",
		Actual:   idSet["a::b::c"],
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a state with tests 'a::b::c' and 'd::e::f'",
		Should:   "include 'd::e::f' in the returned IDs",
		Actual:   idSet["d::e::f"],
		Expected: true,
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
