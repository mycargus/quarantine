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
		t.Skipf("schema file not found: %v", err)
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
