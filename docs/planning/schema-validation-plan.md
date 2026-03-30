# Plan: Golden Fixture Schema Validation + Wire Fixtures Into Tests

## Context

The test strategy says contract tests validate golden fixtures against JSON schemas on every commit, breaking "on both sides" if schemas drift. Three schemas exist in `schemas/`, 12 golden fixtures exist in `testdata/`, but:

1. **No schema validation exists.** The `schemas-validate` Makefile target is a TODO stub. The contract test CI job passes silently via `--passWithNoTests`.
2. **No code reads the golden fixtures.** The `testdata/expected/` and `testdata/quarantine-state/` fixtures are orphaned — no Go test, no JS test, no dashboard test references them.
3. **The dashboard duplicates fixtures.** Both `ingest.server.test.ts` and `ingest-artifact.server.test.ts` define inline `validFixture` objects identical to `testdata/expected/jest-flaky.json`.
4. **No Go-side schema validation.** Go structs could drift from schemas without detection. The `omitempty` vs `required` tension on `quarantine.Entry.IssueNumber` proves this is a real risk, not theoretical.

This plan closes all four gaps.

## Contract inventory

Contracts this plan validates (schema ↔ data shape):

| Contract | Producer | Consumer | Schema | Validated by |
|---|---|---|---|---|
| test-result ↔ CLI fixtures | hand-written | Go tests (aspirational) | `test-result.schema.json` | Part A (JS) |
| test-result ↔ dashboard fixtures | hand-written | dashboard tests | `test-result.schema.json` | Part A (JS) |
| test-result ↔ Go `Result` struct | `result.Build*()` | dashboard via artifacts | `test-result.schema.json` | Part B (Go) |
| quarantine-state ↔ CLI fixtures | hand-written | Go tests (aspirational) | `quarantine-state.schema.json` | Part A (JS) |
| quarantine-state ↔ Go `State` struct | `State.MarshalAt()` | CLI on next run | `quarantine-state.schema.json` | Part B (Go) |
| quarantine-config ↔ config fixtures | hand-written | CLI config parser | `quarantine-config.schema.json` | Part A (JS) |

Contracts out of scope for this plan:

| Contract | Notes |
|---|---|
| CLI ↔ GitHub API (Contents, Issues, Search, Comments, Refs) | Covered by Prism contract tests (separate effort) |
| Dashboard ↔ GitHub Artifacts API | Covered by Prism contract tests (separate effort) |
| JUnit XML ↔ CLI parser | Covered by existing parser unit tests with XML fixtures |
| quarantine.yml ↔ Go config validation | CLI validates via Go code, not the JSON schema. Config is YAML, not JSON — schema validation would require YAML→JSON conversion. Separate concern. |
| Artifact naming convention | Implicit convention in README. Not a schema contract. |
| PR comment format (`<!-- quarantine-bot -->` marker) | Implicit contract. Not a schema concern. |
| Issue label structure (`quarantine` + `quarantine:{hash}`) | Implicit contract. Not a schema concern. |
| CLI exit codes (0/1/2) | Well-tested already in run_*_test.go. |

## Known bug found during planning

**Parser "error" status not in schema.** `parser.go:174` maps `<error>` XML elements to `"error"` status, but `test-result.schema.json` only allows `passed|failed|skipped|quarantined|flaky`. If a test framework produces `<error>` instead of `<failure>`, the CLI creates a result that fails dashboard schema validation. File as a separate bug.

## Part A: JS Schema Validation Tests — `test/schema/`

Centralized schema validation in its own directory and vitest project, separate from Prism-based contract tests in `test/contract/`. These are different concerns:

- **Schema tests** (this part): validate static JSON files against JSON schemas. Pure data-at-rest checking. No server, no network.
- **Contract tests** (`test/contract/`): validate HTTP request/response shapes against OpenAPI specs via Prism. Data-in-flight checking with a mock server lifecycle.

Keeping them separate avoids confusing agents and developers who see "contract test" and default to the Prism pattern.

### Changes

**`test/package.json`** — Add `ajv` + `ajv-formats` as direct devDependencies. Add scripts:
```json
"test:schema": "vitest run --project schema",
"lint:schema": "biome check ./schema",
"lint:ci:schema": "biome ci --error-on-warnings ./schema"
```

Needed because:
- Schemas use `$schema: draft/2020-12` → import from `ajv/dist/2020`
- Schemas use `format: "date-time"` and `format: "uri"` → `ajv-formats`
- pnpm strict resolution won't resolve transitive deps from Prism

**`test/vitest.config.js`** — Add a third project entry:
```js
{
  test: {
    name: "schema",
    include: ["schema/**/*.{test,spec}.?(c|m)[jt]s?(x)"],
    testTimeout: 10_000,
  },
}
```

**`test/schema/schema-validation.test.js`** — New test file. Validates golden fixtures from both `testdata/` (CLI) and `dashboard/test-fixtures/` (dashboard) against shared schemas. Four `describe` blocks for positive cases:

| Block | Schema | Fixtures | Count |
|-------|--------|----------|-------|
| test-result (CLI) | `schemas/test-result.schema.json` | `testdata/expected/*.json` | 9 |
| test-result (dashboard) | `schemas/test-result.schema.json` | `dashboard/test-fixtures/*.json` | 1+ |
| quarantine-state | `schemas/quarantine-state.schema.json` | `testdata/quarantine-state/*.json` | 3 |
| quarantine-config | `schemas/quarantine-config.schema.json` | `testdata/quarantine-config/*.json` | 2 |

Plus a `describe` block for **negative cases** — one per schema, verifying ajv rejects known-invalid data:
- test-result: missing `run_id` → invalid
- quarantine-state: entry missing `issue_number` → invalid (schema requires it; Go's `omitempty` would omit it for nil values — this confirms the schema catches that)
- quarantine-config: `framework: "mocha"` → invalid (not in enum)

Each fixture gets one `test()` with a RITEway assertion. Formatted ajv errors in the `given` string on failure.

**`testdata/quarantine-config/minimal.json`** — New fixture: `{ "version": 1, "framework": "jest" }`

**`testdata/quarantine-config/full.json`** — New fixture: all optional fields populated with valid values per the schema.

### Key files
- `test/schema/schema-validation.test.js` (new)
- `test/vitest.config.js` (modify — add schema project)
- `test/package.json` (modify — add deps + scripts)
- `testdata/quarantine-config/minimal.json` (new)
- `testdata/quarantine-config/full.json` (new)

## Part B: Go Schema Validation of Marshaled Output

Validate that Go's actual marshaled JSON conforms to the schemas. This catches struct↔schema drift that the JS fixture tests cannot detect (since fixtures are hand-written, not generated from Go output).

### Why this is needed

The JS schema test (Part A) proves: `fixtures → schema`. But nothing proves: `Go output → schema`. These are different claims. The `omitempty` vs `required` tension on `quarantine.Entry.IssueNumber` is proof — hand-written fixtures include the field, but Go's marshaled output omits it when nil.

### Changes

**`cli/go.mod`** — Add `github.com/santhosh-tekuri/jsonschema/v6` (test-only dependency). Supports draft/2020-12 as the default draft. Actively maintained. The original Makefile stub referenced this library.

**`cli/internal/result/result_test.go`** — Add two test functions:

1. `TestBuildAt_OutputConformsToSchema` — Build a Result from sample parser output + metadata, marshal to JSON, validate against `schemas/test-result.schema.json`.

2. `TestBuildAtWithRetries_OutputConformsToSchema` — Same but with retries (exercises `retry_entry`, `flaky` status, `original_status` fields).

These tests catch: mismatched json tags, missing required fields, wrong types, extra fields rejected by `additionalProperties: false`.

**`cli/internal/quarantine/state_test.go`** — Add test function:

3. `TestMarshalAt_OutputConformsToSchema` — Build a State with entries (including entries with and without `IssueNumber`/`IssueURL`), marshal via `State.MarshalAt()`, validate against `schemas/quarantine-state.schema.json`.

This test will **immediately surface** the `omitempty` vs `required` conflict as a failure. We then decide: fix the schema (make `issue_number`/`issue_url` optional) or fix the Go struct (remove `omitempty`).

### Key files
- `cli/go.mod` / `cli/go.sum` (modify)
- `cli/internal/result/result_test.go` (modify)
- `cli/internal/quarantine/state_test.go` (modify)

## Part C: Dashboard Fixture Extraction — `dashboard/test-fixtures/`

Extract inline fixtures to local files. Dashboard tests import from `dashboard/test-fixtures/` (short, local paths). The schema test in Part A validates these files.

### Changes

**`dashboard/test-fixtures/jest-flaky.json`** — New file. Contents extracted from the inline `validFixture` in `ingest.server.test.ts`. Conforms to `test-result.schema.json`.

**`dashboard/app/lib/ingest.server.test.ts`** — Replace the inline `validFixture` (lines 13-60) with:
```typescript
import _fixture from "../../test-fixtures/jest-flaky.json" with { type: "json" }
const validFixture = _fixture as TestResult
```

The existing spread patterns (`{ ...validFixture, run_id: undefined }`) continue to work unchanged.

**`dashboard/app/lib/ingest-artifact.server.test.ts`** — Same replacement (lines 6-41).

### Consideration: TypeScript + JSON imports

Dashboard tests run via `node --test` with `tsx` loader. `tsx` supports JSON imports natively. The `TestResult` type assertion on the import needs a cast since JSON imports are typed as the parsed shape, not the application type.

### Key files
- `dashboard/test-fixtures/jest-flaky.json` (new)
- `dashboard/app/lib/ingest.server.test.ts` (modify)
- `dashboard/app/lib/ingest-artifact.server.test.ts` (modify)

## Part D: Makefile + CI

### Makefile

**New targets:**
```makefile
schema-test:
	cd test && pnpm run test:schema

schema-lint:
	cd test && pnpm run lint:schema

schema-lint-ci:
	cd test && pnpm run lint:ci:schema
```

**Replace** the `schemas-validate` stub with:
```makefile
schemas-validate: schema-test
```

**Update** `test-all`: replace `schemas-validate` with `schema-test` (or keep `schemas-validate` since it now delegates — either works, but using the direct target is clearer).

**Update** `test-lint`: add `schema-lint` if not already covered.

**Update** `.PHONY` line.

### CI — `.github/workflows/ci.yml`

Add a `schema` job. The schema tests have no special dependencies — just Node + pnpm. Lightweight enough to be its own job.

### Key files
- `Makefile` (modify)
- `.github/workflows/ci.yml` (modify)

## Part E: Documentation Updates

Update docs and skills so future work follows the new pattern. Without these updates, agents and developers will create new fixtures or schemas without validating them.

### `docs/specs/test-strategy.md` — Contract Tests section (lines 80-82)

Expand to document two distinct test types:
- **Schema validation tests** in `test/schema/`: validate golden fixtures against JSON schemas AND Go marshaled output against schemas. CLI fixtures in `testdata/`, dashboard fixtures in `dashboard/test-fixtures/`. Run via `make schema-test`.
- **Prism contract tests** in `test/contract/`: validate HTTP request/response shapes against vendored OpenAPI specs. Run via `make contract-test`.
- Add schema tests to the test layers table.

### `test/contract/README.md`

No changes needed — stays Prism-focused since schema tests have their own directory now.

### `test/schema/README.md` (new)

- What schema validation tests cover
- Where fixtures live (two locations: `testdata/` for CLI, `dashboard/test-fixtures/` for dashboard)
- How to add a new fixture (create the file, test auto-discovers it)
- How to add negative test cases

### `dashboard/CLAUDE.md`

Add `test-fixtures/` to the Structure table. Note that test fixtures are external JSON files validated by the schema test suite.

### New skill: `/create-schema-test`

A new skill (`.claude/skills/create-schema-test/SKILL.md`) that guides creation of schema validation tests. Triggered when adding new JSON schemas or fixture files. Covers:
- When to use this skill vs `/create-contract-test`
- Where to put fixtures (CLI vs dashboard)
- How negative cases work
- How auto-discovery picks up new fixture files

### `.claude/skills/create-contract-test/SKILL.md`

No changes needed — stays Prism-focused. The new `/create-schema-test` skill handles the schema validation concern.

### `docs/milestones/m6.md`

- Line 29: Update "Developed against golden test fixtures from `testdata/expected/`" → reference `dashboard/test-fixtures/`
- Line 51: "Contract tests: fixture JSON validates against shared schema" — mark as implemented, reference `test/schema/`

### Key files
- `docs/specs/test-strategy.md` (modify)
- `test/schema/README.md` (new)
- `dashboard/CLAUDE.md` (modify)
- `.claude/skills/create-schema-test/SKILL.md` (new)
- `docs/milestones/m6.md` (modify)

## What this plan does NOT include

### Golden file comparison (Go output vs fixtures)

Out of scope. Would require resolving:
- `*-flaky.json` fixtures include `issue_number` set by the issue creation step, not `Build*()`
- Go's `json.Marshal` field ordering differs from fixture ordering for entries with retries
- Retry attempt data in `*-single-failure.json` doesn't come from XML fixtures alone

### quarantine.yml ↔ schema validation

Out of scope. The CLI validates config via Go code, not the JSON schema. Config is YAML, not JSON — adding schema validation would require YAML→JSON conversion. The Go validation code and the JSON schema could drift independently, but that's a separate concern.

### Dashboard fixture consolidation beyond `jest-flaky.json`

The dashboard currently needs only one fixture. If future dashboard tests need more fixture variety, they can be added to `dashboard/test-fixtures/` and the schema test auto-discovers them.

## Verification

1. `cd test && pnpm install` — installs ajv
2. `make schema-test` — 15+ positive tests + 3 negative tests pass
3. `make schemas-validate` — delegates to `schema-test`, same results
4. `make schema-lint` — Biome passes on new test file
5. `make cli-test` — Go schema validation tests pass (or fail, surfacing the `omitempty` issue)
6. `make dash-test` — dashboard tests pass with imported fixtures
7. `make test-all` — all green, no duplicate runs
8. Break a fixture (remove `version`) → schema test fails with clear ajv error
9. Negative test: entry missing `issue_number` → schema rejects it
10. Go test: marshal State with nil IssueNumber → schema validation surfaces `omitempty` vs `required` conflict
