package result_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mycargus/quarantine/cli/internal/parser"
	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)

// schemaPath resolves the path to test-result.schema.json relative to this
// file's location, so the test works regardless of the working directory.
func schemaPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cli/internal/result/ → cli/ → repo root → schemas/
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schemas", "test-result.schema.json")
}

func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath(t))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

func validateResult(t *testing.T, sch *jsonschema.Schema, r result.Result) error {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal for validation: %v", err)
	}
	return sch.Validate(v)
}

func TestResultSchemaConformance(t *testing.T) {
	// Guard: skip if schema file not accessible (shouldn't happen in normal CI).
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Skipf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	failMsg := "assertion failed"
	tests := []parser.TestResult{
		{
			TestID:     "src/foo.test.ts::Suite::passes",
			FilePath:   "src/foo.test.ts",
			Classname:  "Suite",
			Name:       "passes",
			Status:     "passed",
			DurationMs: 100,
		},
		{
			TestID:         "src/bar.test.ts::Suite::fails",
			FilePath:       "src/bar.test.ts",
			Classname:      "Suite",
			Name:           "fails",
			Status:         "failed",
			DurationMs:     200,
			FailureMessage: &failMsg,
		},
	}
	meta := result.Metadata{
		RunID:      "run-123",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123def456abc123def456abc123def456abc1",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 3,
	}

	res := result.BuildAt(tests, meta, "2026-01-15T10:00:00Z")
	err := validateResult(t, sch, res)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Result with valid fields for a mixed passed/failed run",
		Should:   "conform to test-result.schema.json",
		Actual:   err,
		Expected: nil,
	})
}

func TestResultSchemaConformance_ErrorElementMappedToFailed(t *testing.T) {
	// Scenario 72: a test case with an <error> JUnit XML element must produce
	// a status value that conforms to the schema enum. The parser maps <error>
	// to "failed", which is in the enum ["passed","failed","skipped","quarantined","flaky"].
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Skipf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	tests := []parser.TestResult{
		{
			TestID:     "src/crash.test.ts::Suite::crashes",
			FilePath:   "src/crash.test.ts",
			Classname:  "Suite",
			Name:       "crashes",
			Status:     "failed", // mapped from <error> by the parser
			DurationMs: 50,
		},
	}
	meta := result.Metadata{
		RunID:      "run-error",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123def456abc123def456abc123def456abc1",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 3,
	}

	res := result.BuildAt(tests, meta, "2026-01-15T10:00:00Z")
	err := validateResult(t, sch, res)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Result where a test had an <error> JUnit element (mapped to 'failed')",
		Should:   "conform to test-result.schema.json (Scenario 72)",
		Actual:   err,
		Expected: nil,
	})
}
