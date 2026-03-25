package quarantine_test

import (
	"testing"

	"github.com/mycargus/quarantine/internal/parser"
	"github.com/mycargus/quarantine/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// --- FilterQuarantinedFailures ---

func TestFilterQuarantinedFailuresPassedTestUnchanged(t *testing.T) {
	state := quarantine.NewEmptyState()
	tests := []parser.TestResult{
		{TestID: "file.rb::Foo::passes", Name: "passes", Status: "passed"},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a passing test with an empty quarantine state",
		Should:   "leave the test status unchanged as 'passed'",
		Actual:   result[0].Status,
		Expected: "passed",
	})
}

func TestFilterQuarantinedFailuresQuarantinedFailureReclassified(t *testing.T) {
	state := quarantine.NewEmptyState()
	issueNum := 42
	state.AddTest(quarantine.Entry{
		TestID:      "spec/models/user_spec.rb::User::is valid",
		Name:        "is valid",
		IssueNumber: &issueNum,
	})

	failMsg := "expected true, got false"
	tests := []parser.TestResult{
		{
			TestID:         "spec/models/user_spec.rb::User::is valid",
			Name:           "is valid",
			Status:         "failed",
			FailureMessage: &failMsg,
		},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a failed test that is quarantined",
		Should:   "reclassify status to 'quarantined'",
		Actual:   result[0].Status,
		Expected: "quarantined",
	})
}

func TestFilterQuarantinedFailuresOriginalStatusPreservedInFailureMessage(t *testing.T) {
	state := quarantine.NewEmptyState()
	state.AddTest(quarantine.Entry{
		TestID: "spec/services/payment_spec.rb::PaymentService::handles timeout",
		Name:   "handles timeout",
	})

	failMsg := "timeout after 5000ms"
	tests := []parser.TestResult{
		{
			TestID:         "spec/services/payment_spec.rb::PaymentService::handles timeout",
			Name:           "handles timeout",
			Status:         "failed",
			FailureMessage: &failMsg,
		},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined test that failed",
		Should:   "preserve the failure message",
		Actual:   *result[0].FailureMessage,
		Expected: "timeout after 5000ms",
	})
}

func TestFilterQuarantinedFailuresNonQuarantinedFailureUnchanged(t *testing.T) {
	state := quarantine.NewEmptyState()
	// State has a different test quarantined.
	state.AddTest(quarantine.Entry{
		TestID: "spec/other_spec.rb::Other::something",
		Name:   "something",
	})

	failMsg := "AssertionError"
	tests := []parser.TestResult{
		{
			TestID:         "spec/services/payment_spec.rb::PaymentService::real failure",
			Name:           "real failure",
			Status:         "failed",
			FailureMessage: &failMsg,
		},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a failed test that is NOT quarantined",
		Should:   "leave status as 'failed'",
		Actual:   result[0].Status,
		Expected: "failed",
	})
}

func TestFilterQuarantinedFailuresMixedResults(t *testing.T) {
	state := quarantine.NewEmptyState()
	state.AddTest(quarantine.Entry{
		TestID: "spec/models/user_spec.rb::User::is valid",
		Name:   "is valid",
	})

	failMsg := "error"
	tests := []parser.TestResult{
		{TestID: "spec/models/user_spec.rb::User::is valid", Status: "failed", FailureMessage: &failMsg},
		{TestID: "spec/services/payment_spec.rb::PaymentService::real failure", Status: "failed", FailureMessage: &failMsg},
		{TestID: "spec/other_spec.rb::Other::passes", Status: "passed"},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a mix of quarantined failure, real failure, and pass",
		Should:   "reclassify the quarantined failure to 'quarantined'",
		Actual:   result[0].Status,
		Expected: "quarantined",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a mix of quarantined failure, real failure, and pass",
		Should:   "leave the real failure as 'failed'",
		Actual:   result[1].Status,
		Expected: "failed",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a mix of quarantined failure, real failure, and pass",
		Should:   "leave the passing test as 'passed'",
		Actual:   result[2].Status,
		Expected: "passed",
	})
}

func TestFilterQuarantinedFailuresEmptyInputReturnsEmpty(t *testing.T) {
	state := quarantine.NewEmptyState()
	result := quarantine.FilterQuarantinedFailures([]parser.TestResult{}, state)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty test results",
		Should:   "return empty slice",
		Actual:   len(result),
		Expected: 0,
	})
}

func TestFilterQuarantinedFailuresQuarantinedPassTreatedNormally(t *testing.T) {
	// A quarantined test that PASSES should not be reclassified.
	state := quarantine.NewEmptyState()
	state.AddTest(quarantine.Entry{
		TestID: "spec/models/user_spec.rb::User::is valid",
		Name:   "is valid",
	})

	tests := []parser.TestResult{
		{TestID: "spec/models/user_spec.rb::User::is valid", Status: "passed"},
	}

	result := quarantine.FilterQuarantinedFailures(tests, state)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a quarantined test that passes",
		Should:   "leave status as 'passed' (only failures are suppressed)",
		Actual:   result[0].Status,
		Expected: "passed",
	})
}
