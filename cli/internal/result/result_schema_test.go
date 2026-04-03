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
	// Guard: fail loudly if schema file not accessible.
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
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

func TestResultSchemaConformance_FlakyTest(t *testing.T) {
	// Scenario 77: BuildAtWithRetries must produce a result containing a flaky
	// test (failed initial run, passed on retry) that conforms to the schema.
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	failMsg := "expected true, got false"
	tests := []parser.TestResult{
		{
			TestID:     "src/stable.test.ts::Suite::passes",
			FilePath:   "src/stable.test.ts",
			Classname:  "Suite",
			Name:       "passes",
			Status:     "passed",
			DurationMs: 80,
		},
		{
			TestID:         "src/broken.test.ts::Suite::always fails",
			FilePath:       "src/broken.test.ts",
			Classname:      "Suite",
			Name:           "always fails",
			Status:         "failed",
			DurationMs:     150,
			FailureMessage: &failMsg,
		},
		{
			TestID:         "src/flaky.test.ts::Suite::sometimes fails",
			FilePath:       "src/flaky.test.ts",
			Classname:      "Suite",
			Name:           "sometimes fails",
			Status:         "failed",
			DurationMs:     120,
			FailureMessage: &failMsg,
		},
	}
	retries := map[string]result.RetryOutcome{
		"src/flaky.test.ts::Suite::sometimes fails": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "passed", DurationMs: 90},
			},
		},
	}
	meta := result.Metadata{
		RunID:      "run-flaky-77",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123def456abc123def456abc123def456abc1",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 3,
	}

	res := result.BuildAtWithRetries(tests, retries, meta, "2026-01-15T10:00:00Z")
	err := validateResult(t, sch, res)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Result with passing, failing, and flaky tests built via BuildAtWithRetries",
		Should:   "conform to test-result.schema.json (Scenario 77)",
		Actual:   err,
		Expected: nil,
	})
}

func TestResultSchemaConformance_ErrorElementMappedToFailed(t *testing.T) {
	// Scenario 72: a test case with an <error> JUnit XML element must produce
	// a status value that conforms to the schema enum. The parser maps <error>
	// to "failed", which is in the enum ["passed","failed","skipped","quarantined","flaky"].
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
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

func TestResultSchemaConformance_RejectsInvalidData(t *testing.T) {
	// MEDIUM-1: The schema enum for test status must reject unknown values like
	// "errored". This test tampers with a valid result's JSON to inject an
	// invalid status, confirming the schema guard is actually enforced.
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	tests := []parser.TestResult{
		{
			TestID:     "src/foo.test.ts::Suite::passes",
			FilePath:   "src/foo.test.ts",
			Classname:  "Suite",
			Name:       "passes",
			Status:     "passed",
			DurationMs: 100,
		},
	}
	meta := result.Metadata{
		RunID:      "run-invalid",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123def456abc123def456abc123def456abc1",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 3,
	}

	res := result.BuildAt(tests, meta, "2026-01-15T10:00:00Z")

	// Marshal to JSON, unmarshal to generic map, inject invalid status.
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var m any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	testEntries := m.(map[string]any)["tests"].([]any)
	testEntries[0].(map[string]any)["status"] = "errored"

	validationErr := sch.Validate(m)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a Result with a test entry whose status is 'errored' (not in the schema enum)",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestResultSchemaConformance_AllRetriesFail(t *testing.T) {
	// MEDIUM-2: BuildAtWithRetries where the test fails on initial run and on
	// all retries must produce a schema-conforming result with status "failed".
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	failMsg := "assertion failed"
	tests := []parser.TestResult{
		{
			TestID:         "src/broken.test.ts::Suite::always fails",
			FilePath:       "src/broken.test.ts",
			Classname:      "Suite",
			Name:           "always fails",
			Status:         "failed",
			DurationMs:     150,
			FailureMessage: &failMsg,
		},
	}
	retries := map[string]result.RetryOutcome{
		"src/broken.test.ts::Suite::always fails": {
			Attempts: []result.RetryEntry{
				{Attempt: 1, Status: "failed", DurationMs: 140},
				{Attempt: 2, Status: "failed", DurationMs: 145},
			},
		},
	}
	meta := result.Metadata{
		RunID:      "run-all-retries-fail",
		Repo:       "owner/repo",
		Branch:     "main",
		CommitSHA:  "abc123def456abc123def456abc123def456abc1",
		CLIVersion: "0.1.0",
		Framework:  "jest",
		RetryCount: 2,
	}

	res := result.BuildAtWithRetries(tests, retries, meta, "2026-01-15T10:00:00Z")
	err := validateResult(t, sch, res)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a Result from BuildAtWithRetries where the test fails on initial run and all retries",
		Should:   "conform to test-result.schema.json",
		Actual:   err,
		Expected: nil,
	})
}

func TestResultSchemaConformance_QuarantinedTest(t *testing.T) {
	// MEDIUM-3: A Result containing a test with status "quarantined" must
	// conform to the schema. ComputeSummary does not count quarantined tests
	// (returns 0), but the schema only requires minimum: 0, so this is valid.
	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema file not found: %v", err)
	}

	sch := loadSchema(t)

	tests := []parser.TestResult{
		{
			TestID:     "src/flaky.test.ts::Suite::known flaky",
			FilePath:   "src/flaky.test.ts",
			Classname:  "Suite",
			Name:       "known flaky",
			Status:     "quarantined",
			DurationMs: 75,
		},
	}
	meta := result.Metadata{
		RunID:      "run-quarantined",
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
		Given:    "a Result containing a test with status 'quarantined'",
		Should:   "conform to test-result.schema.json",
		Actual:   err,
		Expected: nil,
	})
}
