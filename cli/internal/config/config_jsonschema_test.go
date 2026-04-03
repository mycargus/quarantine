package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	riteway "github.com/mycargus/riteway-golang"
)

func configSchemaPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cli/internal/config/ → cli/ → repo root → schemas/
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schemas", "quarantine-config.schema.json")
}

func loadConfigSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(configSchemaPath(t))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

func TestConfigSchemaRejectsInvalidData_InvalidFramework(t *testing.T) {
	// Scenario 78: the schema must reject a framework value not in the enum.
	// "mocha" is not in ["jest", "rspec", "vitest"]. This is a regression test
	// — if a future schema change adds "mocha" to the enum, this test will
	// catch the unintentional expansion.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version":   1,
		"framework": "mocha",
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine config JSON with framework: \"mocha\" (not in the enum)",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestConfigSchemaRejectsInvalidData_MissingFramework(t *testing.T) {
	// The schema requires the framework field. A config with only version set
	// and no framework must fail validation.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version": 1,
		// framework intentionally absent
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine config JSON with the required framework field absent",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestConfigSchemaConformance_ValidMinimalConfig(t *testing.T) {
	// A minimal valid config (version:1, framework:jest) must pass schema
	// validation. This positive conformance test ensures that the schema
	// accepts the simplest correct input and acts as a regression guard if
	// required fields accidentally become stricter.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version":   1,
		"framework": "jest",
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a minimal valid quarantine config JSON (version:1, framework:jest)",
		Should:   "pass schema validation",
		Actual:   validationErr,
		Expected: nil,
	})
}

func TestConfigSchemaRejectsInvalidData_InvalidVersion(t *testing.T) {
	// The schema uses const:1 for version. Any other integer (e.g. 2) must be
	// rejected, ensuring the CLI can detect incompatible config files from
	// future versions.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version":   2,
		"framework": "jest",
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine config JSON with version:2 (const requires 1)",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestConfigSchemaRejectsInvalidData_RetriesOutOfRange(t *testing.T) {
	// The schema enforces minimum:1 on retries. A value of 0 must be rejected.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version":   1,
		"framework": "jest",
		"retries":   0,
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine config JSON with retries:0 (minimum is 1)",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}

func TestConfigSchemaRejectsInvalidData_AdditionalProperties(t *testing.T) {
	// The schema sets additionalProperties:false. An unknown top-level field
	// must be rejected.
	sch := loadConfigSchema(t)

	v := map[string]any{
		"version":       1,
		"framework":     "jest",
		"unknown_field": "value",
	}

	validationErr := sch.Validate(v)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a quarantine config JSON with an unknown top-level field (additionalProperties:false)",
		Should:   "fail schema validation",
		Actual:   validationErr != nil,
		Expected: true,
	})
}
