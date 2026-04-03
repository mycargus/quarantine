package quarantine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// quarantineSchemaPath resolves the path to quarantine-state.schema.json
// relative to this file's location, so the test works regardless of the
// working directory.
func quarantineSchemaPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cli/internal/quarantine/ → cli/ → repo root → schemas/
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schemas", "quarantine-state.schema.json")
}

func loadQuarantineSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(quarantineSchemaPath(t))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

func validateState(t *testing.T, sch *jsonschema.Schema, s *quarantine.State, timestamp string) error {
	t.Helper()
	raw, err := s.MarshalAt(timestamp)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal for validation: %v", err)
	}
	return sch.Validate(v)
}

func TestStateSchemaConformance_EntryWithoutIssueFields(t *testing.T) {
	// Scenario 66: an entry written before issue creation completes must
	// conform to quarantine-state.schema.json even without issue_number or
	// issue_url. The schema uses dependentRequired (not required) for those
	// fields, so omitting both is valid.
	if _, err := os.Stat(quarantineSchemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
	}

	sch := loadQuarantineSchema(t)

	s := quarantine.NewEmptyStateAt("2026-01-01T00:00:00Z")
	s.AddTest(quarantine.Entry{
		TestID:        "src/auth.test.ts::AuthService::logs in",
		FilePath:      "src/auth.test.ts",
		Classname:     "AuthService",
		Name:          "logs in",
		Suite:         "AuthService",
		FirstFlakyAt:  "2026-01-01T00:00:00Z",
		LastFlakyAt:   "2026-01-01T00:00:00Z",
		FlakyCount:    1,
		QuarantinedAt: "2026-01-01T00:00:00Z",
		QuarantinedBy: "cli-auto",
		// IssueNumber and IssueURL intentionally absent — issue creation not yet complete
	})

	err := validateState(t, sch, s, "2026-01-01T00:00:00Z")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a State entry with no issue_number or issue_url (pre-issue-creation)",
		Should:   "conform to quarantine-state.schema.json (Scenario 66)",
		Actual:   err,
		Expected: nil,
	})
}

func TestStateSchemaRejectsInvalidData_DependentRequiredViolation(t *testing.T) {
	// Scenario 66 negative case: if issue_number is present, the schema's
	// dependentRequired constraint mandates that issue_url must also be
	// present. An entry with only issue_number must fail validation.
	sch := loadQuarantineSchema(t)

	v := map[string]any{
		"version":    1,
		"updated_at": "2026-01-01T00:00:00Z",
		"tests": map[string]any{
			"src/foo.test.js::Suite::test": map[string]any{
				"test_id":        "src/foo.test.js::Suite::test",
				"file_path":      "src/foo.test.js",
				"classname":      "Suite",
				"name":           "test",
				"suite":          "Suite",
				"first_flaky_at": "2026-01-01T00:00:00Z",
				"last_flaky_at":  "2026-01-01T00:00:00Z",
				"flaky_count":    1,
				"quarantined_at": "2026-01-01T00:00:00Z",
				"quarantined_by": "cli-auto",
				"issue_number":   42,
				// issue_url intentionally absent — violates dependentRequired
			},
		},
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine state JSON entry with issue_number set but issue_url absent",
		Should:   "fail schema validation (dependentRequired)",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestStateSchemaRejectsInvalidData_DependentRequiredViolation_ReverseDirection(t *testing.T) {
	// The schema's dependentRequired is bidirectional: issue_url requires
	// issue_number, and issue_number requires issue_url. This test covers the
	// reverse direction: issue_url present without issue_number must fail.
	sch := loadQuarantineSchema(t)

	v := map[string]any{
		"version":    1,
		"updated_at": "2026-01-01T00:00:00Z",
		"tests": map[string]any{
			"src/foo.test.js::Suite::test": map[string]any{
				"test_id":        "src/foo.test.js::Suite::test",
				"file_path":      "src/foo.test.js",
				"classname":      "Suite",
				"name":           "test",
				"suite":          "Suite",
				"first_flaky_at": "2026-01-01T00:00:00Z",
				"last_flaky_at":  "2026-01-01T00:00:00Z",
				"flaky_count":    1,
				"quarantined_at": "2026-01-01T00:00:00Z",
				"quarantined_by": "cli-auto",
				// issue_number intentionally absent — violates dependentRequired
				"issue_url": "https://github.com/owner/repo/issues/42",
			},
		},
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine state JSON entry with issue_url set but issue_number absent",
		Should:   "fail schema validation (dependentRequired reverse direction)",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestStateSchemaConformance_EntryWithBothIssueFields(t *testing.T) {
	// A fully-populated entry (with both issue_number and issue_url) must pass
	// schema validation. This positive conformance test confirms that the
	// dependentRequired constraint does not block valid entries.
	sch := loadQuarantineSchema(t)

	v := map[string]any{
		"version":    1,
		"updated_at": "2026-01-01T00:00:00Z",
		"tests": map[string]any{
			"src/foo.test.js::Suite::test": map[string]any{
				"test_id":        "src/foo.test.js::Suite::test",
				"file_path":      "src/foo.test.js",
				"classname":      "Suite",
				"name":           "test",
				"suite":          "Suite",
				"first_flaky_at": "2026-01-01T00:00:00Z",
				"last_flaky_at":  "2026-01-01T00:00:00Z",
				"flaky_count":    1,
				"quarantined_at": "2026-01-01T00:00:00Z",
				"quarantined_by": "cli-auto",
				"issue_number":   42,
				"issue_url":      "https://github.com/owner/repo/issues/42",
			},
		},
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a quarantine state JSON entry with both issue_number and issue_url present",
		Should:   "pass schema validation",
		Actual:   validationErr,
		Expected: nil,
	})
}

func TestStateSchemaRejectsInvalidData_MissingTestID(t *testing.T) {
	// Scenario 78: the schema must enforce the required test_id field in each
	// quarantined test entry. This is a regression test — if a future schema
	// change makes test_id optional, this test will catch it.
	sch := loadQuarantineSchema(t)

	v := map[string]any{
		"version":    1,
		"updated_at": "2026-01-01T00:00:00Z",
		"tests": map[string]any{
			"src/foo.test.js::Suite::test": map[string]any{
				// "test_id": INTENTIONALLY ABSENT,
				"file_path":      "src/foo.test.js",
				"classname":      "Suite",
				"name":           "test",
				"suite":          "Suite",
				"first_flaky_at": "2026-01-01T00:00:00Z",
				"last_flaky_at":  "2026-01-01T00:00:00Z",
				"flaky_count":    1,
				"quarantined_at": "2026-01-01T00:00:00Z",
				"quarantined_by": "cli-auto",
			},
		},
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine state JSON with an entry missing the required test_id field",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}
