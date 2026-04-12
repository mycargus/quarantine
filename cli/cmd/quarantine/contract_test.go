package main

// Implicit contract tests for PR comment format (#12), Issue creation (#13),
// and Issue dedup labels (#14).
//
// These tests are not behind a build tag — they run with `go test ./...`.
// Each test name includes "Contract" to make contract tests grep-able and to
// signal that the tested value is a compatibility surface (not an
// implementation detail). See docs/specs/contracts.md for the contract
// definitions these tests guard.

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// ── #12 PR comment format ────────────────────────────────────────────────────

// TestContractSuitePRCommentMarkerIsFirstLine verifies that the suite-specific
// quarantine marker is the very first line of every rendered PR comment.
// Contract: the marker MUST be the first line for update-vs-create detection.
func TestContractSuitePRCommentMarkerIsFirstLine(t *testing.T) {
	marker := suitePRCommentMarker("backend")
	data := PRCommentData{Total: 1, Passed: 1, Version: "0.1.0"}
	comment := renderPRComment(data, marker)
	firstLine := strings.SplitN(comment, "\n", 2)[0]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "renderPRComment with any data and suite marker",
		Should:   "have the suite marker as the exact first line",
		Actual:   firstLine,
		Expected: marker,
	})
}

// TestContractSuitePRCommentMarkerDetectsExistingComment verifies that a comment
// body starting with the suite marker is recognised as an existing bot comment.
// This is the update-vs-create heuristic used in postOrUpdatePRComment.
func TestContractSuitePRCommentMarkerDetectsExistingComment(t *testing.T) {
	marker := suitePRCommentMarker("backend")
	body := renderPRComment(PRCommentData{Total: 1, Passed: 1, Version: "0.1.0"}, marker)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a PR comment rendered by renderPRComment with a suite marker",
		Should:   "start with the suite marker (detected as existing bot comment)",
		Actual:   strings.HasPrefix(body, marker),
		Expected: true,
	})
}

// TestContractSuitePRCommentMarkerFormat locks down the exact format of the
// suite-specific PR comment marker. Changing suitePRCommentMarker breaks
// update detection for all existing bot comments in the wild.
func TestContractSuitePRCommentMarkerFormat(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "suitePRCommentMarker(\"backend\")",
		Should:   "equal '<!-- quarantine:backend -->'",
		Actual:   suitePRCommentMarker("backend"),
		Expected: "<!-- quarantine:backend -->",
	})
}

// ── #13 Issue creation ───────────────────────────────────────────────────────

// TestContractIssueTitleFormat verifies that issue titles are prefixed with
// IssueTitlePrefix. The search-based dedup relies on label structure (not
// title), but the prefix is a human-readable convention worth protecting.
func TestContractIssueTitleFormat(t *testing.T) {
	testName := "should handle eviction"
	title := IssueTitlePrefix + testName

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an issue title built from IssueTitlePrefix + testName",
		Should:   "start with IssueTitlePrefix",
		Actual:   strings.HasPrefix(title, IssueTitlePrefix),
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "IssueTitlePrefix constant",
		Should:   "equal '[Quarantine] '",
		Actual:   IssueTitlePrefix,
		Expected: "[Quarantine] ",
	})
}

// ── #14 Issue dedup labels ───────────────────────────────────────────────────

// TestContractIssueLabelArrayStructure verifies that issue label arrays have
// exactly two elements: the base label and the hash label. Changing the count
// or structure breaks the search query used for deduplication.
func TestContractIssueLabelArrayStructure(t *testing.T) {
	hash := testHash("src/cache.test.js::CacheService::should handle eviction")
	labels := []string{IssueLabelBase, IssueLabelPrefix + hash}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a flaky test's issue label array",
		Should:   "have exactly 2 labels",
		Actual:   len(labels),
		Expected: 2,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "first label in issue label array",
		Should:   "equal IssueLabelBase ('quarantine')",
		Actual:   labels[0],
		Expected: IssueLabelBase,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "second label in issue label array",
		Should:   "start with IssueLabelPrefix ('quarantine:')",
		Actual:   strings.HasPrefix(labels[1], IssueLabelPrefix),
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "IssueLabelBase constant",
		Should:   "equal 'quarantine'",
		Actual:   IssueLabelBase,
		Expected: "quarantine",
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "IssueLabelPrefix constant",
		Should:   "equal 'quarantine:'",
		Actual:   IssueLabelPrefix,
		Expected: "quarantine:",
	})
}

// TestContractDedupHashIsEightHexChars verifies that testHash always returns
// exactly DedupHashLength (8) hexadecimal characters.
// The search query uses label:quarantine:{hash} — a different length breaks
// matching against existing issues.
func TestContractDedupHashIsEightHexChars(t *testing.T) {
	hash := testHash("src/payment.test.js::PaymentService::should handle charge timeout")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "testHash of any test ID",
		Should:   "return exactly DedupHashLength characters",
		Actual:   len(hash),
		Expected: DedupHashLength,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "DedupHashLength constant",
		Should:   "equal 8",
		Actual:   DedupHashLength,
		Expected: 8,
	})
	// Verify all chars are hex digits.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "testHash output",
		Should:   "consist of lowercase hex digits only",
		Actual: func() bool {
			for _, ch := range hash {
				if !strings.ContainsRune("0123456789abcdef", ch) {
					return false
				}
			}
			return true
		}(),
		Expected: true,
	})
}

// TestContractDedupHashIsDeterministic verifies that testHash returns the same
// value for the same input. The dedup search query is built from this hash —
// non-determinism would create duplicate issues on every run.
func TestContractDedupHashIsDeterministic(t *testing.T) {
	testID := "src/payment.test.js::PaymentService::should handle charge timeout"

	riteway.Assert(t, riteway.Case[string]{
		Given:    "testHash called twice with the same test ID",
		Should:   "return the same hash both times",
		Actual:   testHash(testID),
		Expected: testHash(testID),
	})
}
