package quarantine_test

import (
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// --- Mutation-coverage tests for the FirstFlakyAt preservation logic ---
//
// The merge loop contains this block (lines 134-146):
//
//   current := merged.Tests[id]
//   earliest := current.FirstFlakyAt
//   if existing.FirstFlakyAt != "" &&
//       (earliest == "" || existing.FirstFlakyAt < earliest) {
//       earliest = existing.FirstFlakyAt
//   }
//   if entry.FirstFlakyAt != "" &&
//       (earliest == "" || entry.FirstFlakyAt < earliest) {
//       earliest = entry.FirstFlakyAt
//   }
//
// Mutation 1 (line 135): `&&` → `||`
//
// The intent is to skip updating `earliest` when `existing.FirstFlakyAt`
// is empty, so that a previously established value is not overwritten.
// At first glance, flipping `&&` to `||` when existing.FirstFlakyAt == ""
// could corrupt `earliest`. However, `current` and `entry` represent the
// SAME local entry whenever local wins the FlakyCount check — so
// `current.FirstFlakyAt == entry.FirstFlakyAt`. The subsequent `entry`
// block (line 139-141) always re-applies `entry.FirstFlakyAt` when it is
// non-empty and `earliest` has been zeroed out, restoring the correct
// value. When `entry.FirstFlakyAt` is empty, `earliest` started as "" and
// there is nothing to corrupt. As a result, this mutation is **equivalent**
// — no observable behavioral difference exists regardless of input. A
// best-effort test is included below to document the intent and to serve as
// a regression guard if the surrounding code ever changes.
//
// Mutations 2 and 3 (lines 136, 140): `<` → `<=`
//
// These replace a strict-less-than comparison with less-than-or-equal. When
// `existing.FirstFlakyAt == earliest`, the assignment `earliest =
// existing.FirstFlakyAt` is a no-op (the value is already the same). The
// final result is therefore identical under both `<` and `<=`, making these
// **equivalent mutants**. Tests are included below to document the expected
// behavior and to catch any future refactoring that changes the semantics.

// TestMergeConflictExistingEmptyFirstFlakyAtLocalWins targets mutation 1
// (line 135: `&&` → `||`). Local wins the FlakyCount check and carries a
// non-empty FirstFlakyAt; remote has an empty FirstFlakyAt. The merged entry
// must retain local's timestamp.
//
// NOTE: This mutation is equivalent — the `entry` block (lines 139-141)
// restores the correct value in all reachable scenarios — but the test is
// kept to document intent and guard against future code changes.
func TestMergeConflictExistingEmptyFirstFlakyAtLocalWins(t *testing.T) {
	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
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

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "", // remote has no first_flaky_at
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    3,
		QuarantinedAt: "2026-03-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given: "a conflict where local wins the flaky count (10 > 3) and has" +
			" a non-empty first_flaky_at while remote has an empty one",
		Should:   "preserve local's first_flaky_at in the merged entry",
		Actual:   entry.FirstFlakyAt,
		Expected: "2026-03-05T00:00:00Z",
	})
}

// TestMergeConflictIdenticalFirstFlakyAtBothSides targets mutations 2 and 3
// (lines 136 and 140: `<` → `<=`). Both local and remote have the same
// FirstFlakyAt timestamp. The merged entry must carry that timestamp
// unchanged.
//
// NOTE: These mutations are equivalent — when the candidate value equals
// `earliest`, the assignment is a no-op and the final result is identical
// under either operator. The test is kept to document the expected behavior.
func TestMergeConflictIdenticalFirstFlakyAtBothSides(t *testing.T) {
	const sharedTimestamp = "2026-03-01T00:00:00Z"

	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  sharedTimestamp,
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    10,
		QuarantinedAt: sharedTimestamp,
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  sharedTimestamp,
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    5,
		QuarantinedAt: sharedTimestamp,
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given: "a conflict where both local and remote have the same" +
			" first_flaky_at timestamp",
		Should:   "preserve that shared first_flaky_at in the merged entry",
		Actual:   entry.FirstFlakyAt,
		Expected: sharedTimestamp,
	})
}

// TestMergeConflictExistingFirstFlakyAtEqualsEarliest targets mutation 2
// (line 136: `existing.FirstFlakyAt < earliest` → `<=`). Remote wins the
// FlakyCount check, so current == existing and earliest is initialised from
// the remote entry. The existing block then compares that same value against
// itself; with `<=`, the assignment is executed but the value is unchanged
// (no-op). The test verifies the correct timestamp is present in the result.
//
// NOTE: This mutation is equivalent — assigning a value to itself cannot
// change the outcome.
func TestMergeConflictExistingFirstFlakyAtEqualsEarliest(t *testing.T) {
	const remoteTimestamp = "2026-03-03T00:00:00Z"

	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-10T00:00:00Z", // later than remote
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    2,
		QuarantinedAt: "2026-03-10T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  remoteTimestamp,
		LastFlakyAt:   "2026-03-12T00:00:00Z",
		FlakyCount:    8, // remote wins FlakyCount; current == existing
		QuarantinedAt: remoteTimestamp,
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given: "a conflict where remote wins the flaky count and its" +
			" first_flaky_at equals the initial value of earliest",
		Should:   "keep the remote's first_flaky_at unchanged in the merged entry",
		Actual:   entry.FirstFlakyAt,
		Expected: remoteTimestamp,
	})
}

// TestMergeConflictEntryFirstFlakyAtEqualsEarliest targets mutation 3
// (line 140: `entry.FirstFlakyAt < earliest` → `<=`). Local wins the
// FlakyCount check, so current == entry and earliest is initialised from the
// local entry. The entry block then compares that same value against itself;
// with `<=`, the assignment is executed but the value is unchanged (no-op).
//
// NOTE: This mutation is equivalent — assigning a value to itself cannot
// change the outcome.
func TestMergeConflictEntryFirstFlakyAtEqualsEarliest(t *testing.T) {
	const localTimestamp = "2026-03-03T00:00:00Z"

	local := quarantine.NewEmptyState()
	local.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  localTimestamp,
		LastFlakyAt:   "2026-03-14T00:00:00Z",
		FlakyCount:    10, // local wins FlakyCount; current == entry
		QuarantinedAt: localTimestamp,
		QuarantinedBy: "cli-auto",
	})

	remote := quarantine.NewEmptyState()
	remote.AddTest(quarantine.Entry{
		TestID:        "a::b::c",
		FilePath:      "a",
		Classname:     "b",
		Name:          "c",
		Suite:         "b",
		FirstFlakyAt:  "2026-03-10T00:00:00Z", // later than local
		LastFlakyAt:   "2026-03-10T00:00:00Z",
		FlakyCount:    4,
		QuarantinedAt: "2026-03-10T00:00:00Z",
		QuarantinedBy: "cli-auto",
	})

	merged := quarantine.MergeAt(local, remote, "2026-03-20T00:00:00Z")
	entry := merged.Tests["a::b::c"]

	riteway.Assert(t, riteway.Case[string]{
		Given: "a conflict where local wins the flaky count and its" +
			" first_flaky_at equals the initial value of earliest",
		Should:   "keep the local's first_flaky_at unchanged in the merged entry",
		Actual:   entry.FirstFlakyAt,
		Expected: localTimestamp,
	})
}

// TestMergeAtResultHasVersion1 verifies the merged state has Version=1.
// Kills mutation on line 112: `Version: 1` → `Version: 0`.
func TestMergeAtResultHasVersion1(t *testing.T) {
	local := quarantine.NewEmptyState()
	remote := quarantine.NewEmptyState()

	merged := quarantine.MergeAt(local, remote, "2026-03-28T00:00:00Z")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two empty states merged",
		Should:   "produce a merged state with Version=1",
		Actual:   merged.Version,
		Expected: 1,
	})
}
