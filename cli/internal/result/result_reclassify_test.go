package result_test

import (
	"testing"

	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)

// Scenario 148: Previously quarantined test fails all retries — failure suppressed.
func TestReclassifyQuarantinedTests_FailedBecomesQuarantined(t *testing.T) {
	failMsg := "assertion error"
	testID := "src/payment.test.js::PaymentService::should handle charge timeout"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: testID, Status: "failed", FailureMessage: &failMsg},
		},
		Summary: result.Summary{Total: 1, Failed: 1},
	}
	quarantinedIDs := map[string]bool{testID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a previously quarantined test that fails all retries",
		Should:   "reclassify the test status to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a previously quarantined test that fails all retries",
		Should:   "set original_status to failed",
		Actual:   res.Tests[0].OriginalStatus != nil && *res.Tests[0].OriginalStatus == "failed",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test that fails all retries",
		Should:   "decrement summary.failed to 0",
		Actual:   res.Summary.Failed,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test that fails all retries",
		Should:   "increment summary.quarantined to 1",
		Actual:   res.Summary.Quarantined,
		Expected: 1,
	})
}

// Scenario 149: Previously quarantined test passes — still reclassified as quarantined.
func TestReclassifyQuarantinedTests_PassedBecomesQuarantined(t *testing.T) {
	testID := "src/auth.test.js::AuthService::should validate expired token"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: testID, Status: "passed"},
		},
		Summary: result.Summary{Total: 1, Passed: 1},
	}
	quarantinedIDs := map[string]bool{testID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a previously quarantined test that passes",
		Should:   "reclassify the test status to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a previously quarantined test that passes",
		Should:   "set original_status to passed",
		Actual:   res.Tests[0].OriginalStatus != nil && *res.Tests[0].OriginalStatus == "passed",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test that passes",
		Should:   "decrement summary.passed to 0",
		Actual:   res.Summary.Passed,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test that passes",
		Should:   "increment summary.quarantined to 1",
		Actual:   res.Summary.Quarantined,
		Expected: 1,
	})
}

// Scenario 150: Previously quarantined test is flaky (fails then passes on retry).
func TestReclassifyQuarantinedTests_FlakyBecomesQuarantined(t *testing.T) {
	testID := "src/cache.test.js::CacheService::should handle eviction under load"
	origStatus := "failed"
	res := result.Result{
		Tests: []result.TestEntry{
			// BuildAtWithRetries sets Status="flaky", OriginalStatus="failed" for a flaky test.
			{TestID: testID, Status: "flaky", OriginalStatus: &origStatus},
		},
		Summary: result.Summary{Total: 1, FlakyDetected: 1},
	}
	quarantinedIDs := map[string]bool{testID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a previously quarantined test classified as flaky by retries",
		Should:   "reclassify the test status to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})

	// original_status is overwritten with the post-retry status ("flaky"), not the
	// pre-retry status ("failed") — the caller wants to know the final execution
	// classification before reclassification occurred.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a previously quarantined test classified as flaky by retries",
		Should:   "overwrite original_status with the post-retry status (flaky)",
		Actual:   res.Tests[0].OriginalStatus != nil && *res.Tests[0].OriginalStatus == "flaky",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test classified as flaky by retries",
		Should:   "decrement summary.flaky_detected to 0",
		Actual:   res.Summary.FlakyDetected,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a previously quarantined test classified as flaky by retries",
		Should:   "increment summary.quarantined to 1",
		Actual:   res.Summary.Quarantined,
		Expected: 1,
	})
}

// Scenario 151: Quarantined failure + genuine failure — genuine failure drives exit 1.
func TestReclassifyQuarantinedTests_MixedQuarantinedAndGenuineFailure(t *testing.T) {
	failMsg := "failure"
	quarantinedID := "src/payment.test.js::PaymentService::should process refund"
	genuineID := "src/checkout.test.js::CheckoutService::should apply discount"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: quarantinedID, Status: "failed", FailureMessage: &failMsg},
			{TestID: genuineID, Status: "failed", FailureMessage: &failMsg},
		},
		Summary: result.Summary{Total: 2, Failed: 2},
	}
	quarantinedIDs := map[string]bool{quarantinedID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined failure alongside a genuine failure",
		Should:   "reclassify only the quarantined test to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined failure alongside a genuine failure",
		Should:   "leave the genuine failure as failed",
		Actual:   res.Tests[1].Status,
		Expected: "failed",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined failure alongside a genuine failure",
		Should:   "set summary.failed to 1 (only the genuine failure remains)",
		Actual:   res.Summary.Failed,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined failure alongside a genuine failure",
		Should:   "set summary.quarantined to 1",
		Actual:   res.Summary.Quarantined,
		Expected: 1,
	})
}

// Scenario 152: Quarantined test is skipped — skipped status is preserved, not reclassified.
func TestReclassifyQuarantinedTests_SkippedPreserved(t *testing.T) {
	testID := "src/migration.test.js::MigrationService::should migrate v2 schema"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: testID, Status: "skipped"},
		},
		Summary: result.Summary{Total: 1, Skipped: 1},
	}
	quarantinedIDs := map[string]bool{testID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined test that the test runner skipped",
		Should:   "preserve the skipped status unchanged",
		Actual:   res.Tests[0].Status,
		Expected: "skipped",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantined test that the test runner skipped",
		Should:   "leave original_status as nil",
		Actual:   res.Tests[0].OriginalStatus == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined test that the test runner skipped",
		Should:   "not increment summary.quarantined",
		Actual:   res.Summary.Quarantined,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined test that the test runner skipped",
		Should:   "preserve summary.skipped",
		Actual:   res.Summary.Skipped,
		Expected: 1,
	})
}

// Non-quarantined tests must not be affected.
func TestReclassifyQuarantinedTests_NonQuarantinedTestUnchanged(t *testing.T) {
	testID := "src/auth.test.js::AuthService::login"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: testID, Status: "passed"},
		},
		Summary: result.Summary{Total: 1, Passed: 1},
	}
	// testID is NOT in quarantinedIDs
	quarantinedIDs := map[string]bool{}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a test not in the quarantined IDs set",
		Should:   "leave the status unchanged",
		Actual:   res.Tests[0].Status,
		Expected: "passed",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a test not in the quarantined IDs set",
		Should:   "not affect summary.quarantined",
		Actual:   res.Summary.Quarantined,
		Expected: 0,
	})
}

// Unresolved tests in quarantine state must not be reclassified (infrastructure failure).
func TestReclassifyQuarantinedTests_UnresolvedPreserved(t *testing.T) {
	testID := "src/foo.test.js::Suite::unreliable"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: testID, Status: "unresolved"},
		},
		Summary: result.Summary{Total: 1, Unresolved: 1},
	}
	quarantinedIDs := map[string]bool{testID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined test whose rerun command failed (unresolved)",
		Should:   "preserve the unresolved status unchanged",
		Actual:   res.Tests[0].Status,
		Expected: "unresolved",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a quarantined test whose rerun command failed (unresolved)",
		Should:   "not increment summary.quarantined",
		Actual:   res.Summary.Quarantined,
		Expected: 0,
	})
}

// Scenario 153: all tests are quarantined and all fail — summary reflects full suppression.
func TestReclassifyQuarantinedTests_AllTestsQuarantinedAllFail(t *testing.T) {
	failMsg := "test failed"
	aID := "src/a.test.js::A::test1"
	bID := "src/b.test.js::B::test2"
	cID := "src/c.test.js::C::test3"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: aID, Status: "failed", FailureMessage: &failMsg},
			{TestID: bID, Status: "failed", FailureMessage: &failMsg},
			{TestID: cID, Status: "failed", FailureMessage: &failMsg},
		},
		Summary: result.Summary{Total: 3, Failed: 3},
	}
	quarantinedIDs := map[string]bool{aID: true, bID: true, cID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "all three tests are quarantined and all fail",
		Should:   "set summary.quarantined to 3",
		Actual:   res.Summary.Quarantined,
		Expected: 3,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "all three tests are quarantined and all fail",
		Should:   "set summary.failed to 0 (all failures suppressed)",
		Actual:   res.Summary.Failed,
		Expected: 0,
	})

	allQuarantined := res.Tests[0].Status == "quarantined" &&
		res.Tests[1].Status == "quarantined" &&
		res.Tests[2].Status == "quarantined"
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all three tests are quarantined and all fail",
		Should:   "reclassify every test to quarantined",
		Actual:   allQuarantined,
		Expected: true,
	})
}

// Scenario 155: pre-run snapshot prevents newly detected flaky from being reclassified.
func TestReclassifyQuarantinedTests_NewlyFlakyNotReclassified(t *testing.T) {
	preExistingID := "src/payment.test.js::PaymentService::should process refund"
	newlyFlakyID := "src/auth.test.js::AuthService::should validate session"
	// newlyFlakyID has OriginalStatus="failed" set by BuildAtWithRetries (it failed
	// then passed on retry), but it is NOT in the pre-run snapshot.
	origFailed := "failed"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: preExistingID, Status: "failed"},
			{TestID: newlyFlakyID, Status: "flaky", OriginalStatus: &origFailed},
		},
		Summary: result.Summary{Total: 2, Failed: 1, FlakyDetected: 1},
	}
	// Pre-run snapshot: only the pre-existing quarantined test.
	quarantinedIDs := map[string]bool{preExistingID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "pre-existing quarantined test and a newly detected flaky test",
		Should:   "reclassify the pre-existing quarantined test to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "pre-existing quarantined test and a newly detected flaky test",
		Should:   "leave the newly detected flaky test as flaky",
		Actual:   res.Tests[1].Status,
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "pre-existing quarantined test and a newly detected flaky test",
		Should:   "set summary.quarantined to 1 (only the pre-existing test)",
		Actual:   res.Summary.Quarantined,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "pre-existing quarantined test and a newly detected flaky test",
		Should:   "preserve summary.flaky_detected at 1 (new discovery visible)",
		Actual:   res.Summary.FlakyDetected,
		Expected: 1,
	})
}

// Scenario 156: multiple quarantined tests with mixed outcomes — all reclassified.
func TestReclassifyQuarantinedTests_MixedOutcomesAllReclassified(t *testing.T) {
	aID := "src/a.test.js::A::test1" // passes
	bID := "src/b.test.js::B::test2" // flaky
	cID := "src/c.test.js::C::test3" // fails all retries
	origFailed := "failed"
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: aID, Status: "passed"},
			{TestID: bID, Status: "flaky", OriginalStatus: &origFailed},
			{TestID: cID, Status: "failed"},
		},
		Summary: result.Summary{Total: 3, Passed: 1, FlakyDetected: 1, Failed: 1},
	}
	quarantinedIDs := map[string]bool{aID: true, bID: true, cID: true}

	result.ReclassifyQuarantinedTests(quarantinedIDs, &res)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "reclassify test1 (passed) to quarantined",
		Actual:   res.Tests[0].Status,
		Expected: "quarantined",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "set test1 original_status to passed",
		Actual:   res.Tests[0].OriginalStatus != nil && *res.Tests[0].OriginalStatus == "passed",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "reclassify test2 (flaky) to quarantined",
		Actual:   res.Tests[1].Status,
		Expected: "quarantined",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "overwrite test2 original_status with flaky (post-retry status)",
		Actual:   res.Tests[1].OriginalStatus != nil && *res.Tests[1].OriginalStatus == "flaky",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "reclassify test3 (failed) to quarantined",
		Actual:   res.Tests[2].Status,
		Expected: "quarantined",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "set test3 original_status to failed",
		Actual:   res.Tests[2].OriginalStatus != nil && *res.Tests[2].OriginalStatus == "failed",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[result.Summary]{
		Given:    "three quarantined tests with passed/flaky/failed outcomes",
		Should:   "set summary with quarantined=3, all other counts=0 (total unchanged)",
		Actual:   res.Summary,
		Expected: result.Summary{Total: 3, Quarantined: 3},
	})
}
