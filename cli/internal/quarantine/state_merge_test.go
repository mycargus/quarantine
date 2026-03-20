package quarantine_test

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/quarantine"
)

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

func TestMergeLocalOnlyOneEntry(t *testing.T) {
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    1,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})
	remote := quarantine.NewEmptyState()

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "local with one entry and an empty remote",
		Should:   "produce a merged state with one test",
		Actual:   len(merged.Tests),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "local with one entry and an empty remote",
		Should:   "contain the local entry in the merged state",
		Actual:   merged.HasTest("a::b::c"),
		Expected: true,
	})
}

func TestMergeRemoteOnlyOneEntry(t *testing.T) {
	local := quarantine.NewEmptyState()
	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "d::e::f",
		FilePath:      "d",
		Classname:     "e",
		Name:          "f",
		Suite:         "e",
		FirstFlakyAt:  "2026-03-05T00:00:00Z",
		LastFlakyAt:   "2026-03-12T00:00:00Z",
		FlakyCount:    1,
		QuarantinedAt: "2026-03-05T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "an empty local and remote with one entry",
		Should:   "produce a merged state with one test",
		Actual:   len(merged.Tests),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an empty local and remote with one entry",
		Should:   "contain the remote entry in the merged state",
		Actual:   merged.HasTest("d::e::f"),
		Expected: true,
	})
}

func TestMergeAtUsesProvidedTimestamp(t *testing.T) {
	local := quarantine.NewEmptyState()
	remote := quarantine.NewEmptyState()

	merged := quarantine.MergeAt(local, remote, "2026-05-01T00:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "MergeAt called with a fixed timestamp",
		Should:   "use the provided timestamp for updated_at",
		Actual:   merged.UpdatedAt,
		Expected: "2026-05-01T00:00:00Z",
	})
}

func TestMergeConflictBothFlakyCountZeroKeepsRemote(t *testing.T) {
	// When both local and remote have FlakyCount == 0, the local > check
	// (0 > 0 == false) does not override remote, so remote entry wins.
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-01T00:00:00Z",
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    0,
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
		FlakyCount:    0,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.Merge(local, remote)
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a conflict where both local and remote have flaky_count 0",
		Should:   "keep remote's last_flaky_at (local does not override when 0 > 0 is false)",
		Actual:   entry.LastFlakyAt,
		Expected: "2026-03-14T00:00:00Z",
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
