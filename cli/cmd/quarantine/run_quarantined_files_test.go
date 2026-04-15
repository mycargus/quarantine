package main

import (
	"testing"

	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// Interface tests are in run_quarantined_files_integration_test.go.

// --- Unit tests: quarantinedFilePaths pure function (Scenario 135) ---

func TestQuarantinedFilePaths(t *testing.T) {
	t.Run("deduplicates and sorts file paths", func(t *testing.T) {
		state := qstate.NewEmptyState()
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates email",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates password",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "spec/services/payment_spec.rb::Payment::charges card",
			FilePath: "spec/services/payment_spec.rb",
		})

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return 2 deduplicated file paths",
			Actual:   len(paths),
			Expected: 2,
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return first path as spec/models/user_spec.rb (sorted)",
			Actual:   paths[0],
			Expected: "spec/models/user_spec.rb",
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return second path as spec/services/payment_spec.rb (sorted)",
			Actual:   paths[1],
			Expected: "spec/services/payment_spec.rb",
		})
	})

	t.Run("returns empty slice for empty state", func(t *testing.T) {
		state := qstate.NewEmptyState()

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "an empty quarantine state",
			Should:   "return an empty slice",
			Actual:   len(paths),
			Expected: 0,
		})
	})

	t.Run("filters out empty file_path values", func(t *testing.T) {
		state := qstate.NewEmptyState()
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates email",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "::some::test",
			FilePath: "", // zero-value — should not appear in output
		})

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "state with one valid file_path and one empty file_path",
			Should:   "return only 1 path (empty filtered out)",
			Actual:   len(paths),
			Expected: 1,
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "state with one valid file_path and one empty file_path",
			Should:   "return the non-empty file path",
			Actual:   paths[0],
			Expected: "spec/models/user_spec.rb",
		})
	})
}
