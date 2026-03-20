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

func TestNewEmptyStateAtUsesProvidedTimestamp(t *testing.T) {
	s := quarantine.NewEmptyStateAt("2026-01-01T00:00:00Z")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "NewEmptyStateAt called with a fixed timestamp",
		Should:   "use the provided timestamp for updated_at",
		Actual:   s.UpdatedAt,
		Expected: "2026-01-01T00:00:00Z",
	})
}

func TestMarshalAtUsesProvidedTimestamp(t *testing.T) {
	s := quarantine.NewEmptyStateAt("2026-01-01T00:00:00Z")
	data, err := s.MarshalAt("2026-06-01T12:00:00Z")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "MarshalAt called with a fixed timestamp",
		Should:   "marshal without error",
		Actual:   err,
		Expected: nil,
	})

	parsed, _ := quarantine.ParseState(strings.NewReader(string(data)))

	riteway.Assert(t, riteway.Case[string]{
		Given:    "MarshalAt called with timestamp 2026-06-01T12:00:00Z",
		Should:   "write that timestamp to updated_at",
		Actual:   parsed.UpdatedAt,
		Expected: "2026-06-01T12:00:00Z",
	})
}
