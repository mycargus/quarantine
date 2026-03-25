// Package quarantine handles reading, writing, and merging quarantine.json state.
package quarantine

import (
	"github.com/mycargus/quarantine/internal/parser"
)

// FilterQuarantinedFailures reclassifies test results whose status is "failed"
// or "error" AND whose TestID appears in the quarantine state to status
// "quarantined". Passing, skipped, or flaky quarantined tests are not changed.
//
// This is used for RSpec post-execution filtering: quarantined test failures are
// suppressed from the failed count so they don't break the build.
//
// This is a pure function — no I/O.
func FilterQuarantinedFailures(tests []parser.TestResult, state *State) []parser.TestResult {
	result := make([]parser.TestResult, len(tests))
	for i, t := range tests {
		if (t.Status == "failed" || t.Status == "error") && state.HasTest(t.TestID) {
			t.Status = "quarantined"
		}
		result[i] = t
	}
	return result
}
