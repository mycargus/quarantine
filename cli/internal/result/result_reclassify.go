package result

// ReclassifyQuarantinedTests updates res in-place, changing the status of any
// test whose TestID is in quarantinedIDs from its execution-derived status
// ("failed", "flaky", or "passed") to "quarantined", preserving the
// pre-reclassification status in OriginalStatus. Tests with status "skipped"
// or "unresolved" are left unchanged. The Summary is adjusted to reflect each
// reclassification.
//
// quarantinedIDs must be the set of test IDs that were in the quarantine state
// BEFORE this run started — newly detected flaky tests added during this run
// must not be included, so their first-detection status ("flaky") is preserved.
func ReclassifyQuarantinedTests(quarantinedIDs map[string]bool, res *Result) {
	for i := range res.Tests {
		t := &res.Tests[i]
		if !quarantinedIDs[t.TestID] {
			continue
		}
		switch t.Status {
		case "failed", "flaky", "passed":
			orig := t.Status
			t.OriginalStatus = &orig
			switch orig {
			case "failed":
				res.Summary.Failed--
			case "flaky":
				res.Summary.FlakyDetected--
			case "passed":
				res.Summary.Passed--
			}
			t.Status = "quarantined"
			res.Summary.Quarantined++
		}
		// "skipped" and "unresolved" fall through unchanged.
	}
}
