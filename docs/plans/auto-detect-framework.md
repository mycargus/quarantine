# Plan: Auto-Detection for `quarantine init`

## Scope

**IN scope:**
- Framework detection from `package.json` (both `dependencies` and
  `devDependencies`) and `Gemfile`
- Multi-framework selection prompt
- Documentation updates

**REMOVED from v1 scope:**
- Package manager detection (no consumer — nothing uses the result)
- JUnit XML path detection (framework defaults already correct)

## New Package: `cli/internal/detect`

### Types

```go
type DetectedFramework struct {
    Name   string // "jest", "vitest", "rspec" — matches config.IsValidFramework
    Source string // e.g., "package.json devDependencies", "Gemfile"
}

type Result struct {
    Frameworks []DetectedFramework
}
```

### Public API

```go
// Scan examines the directory for test framework indicators.
// Never returns an error — detection failures are silently ignored.
// Detection is best-effort; the user can always type manually.
func Scan(dir string) Result
```

### Detection Logic

**`package.json`:**
- Parse with `encoding/json`, minimal struct (just `Dependencies` and
  `DevDependencies` maps).
- Check BOTH `dependencies` and `devDependencies` for `"jest"` and `"vitest"`.
- `Source` indicates which section: `"package.json devDependencies"` or
  `"package.json dependencies"`.
- If both jest and vitest found, vitest appears first (migration direction).
- Malformed JSON: silently return empty.

**`Gemfile`:**
- Read line by line.
- `strings.TrimSpace(line)` before checking for `#` prefix (skip comments).
- Match `gem\s+['"]rspec(-core)?['"]` via regex.
- `Gemfile` without `Gemfile.lock` still works — framework detection is
  independent of lockfile presence.

### Error Handling

`Scan()` returns `Result`, not `(Result, error)`. This is intentional:
detection is advisory, not critical. Matches the "never break the build"
principle.

## Changes to `init.go`

### Move `os.Getwd()` Earlier

Currently at Step 7 (line 89). Move to top of `runInit`, before Step 1.
Fail-fast improvement: errors before prompts, not after.

### Call `detect.Scan(cwd)`

Between cwd resolution and overwrite check.

### Modified Init Behavior (post-M9)

Detection is advisory only — no interactive framework prompt. Three cases:

**Case A — One framework detected:**
Pre-fills one suite entry in `.quarantine/config.yml` with the detected
framework's defaults for `junitxml` and `rerun_command`. Prints:
```
Detected test frameworks: jest
Pre-filled 1 suite entry in .quarantine/config.yml
```

**Case B — Multiple detected (pre-fills all, no prompt):**

> **Note (2026-04-11):** The numbered-list selection prompt for Case B is
> deferred with M9. Detection now pre-fills one suite entry per detected
> framework. No interactive selection prompt is shown. See
> `docs/plans/multi-suite-support.md` decision D4.

Pre-fills one suite entry per detected framework. Prints:
```
Detected test frameworks: vitest, jest
Pre-filled 2 suite entries in .quarantine/config.yml
```

**Case C — None detected:**
Writes a commented example suite entry. No prompt for framework. Prints:
```
No test frameworks detected.
```

## Documentation Updates

| File | Line | Change |
|------|------|--------|
| `docs/specs/config-schema.md` | 66 | Remove "There is no auto-detection; the user must choose explicitly." Replace with auto-detection description. |
| `docs/adr/010-config-format.md` | 59 | Update "No auto-detection" to describe default presentation. |
| `docs/milestones/index.md` | 133 | Update "no auto-detection" note. |
| `docs/scenarios/v1/01-initialization.md` | — | Add detection scenarios. |

## Testing

### `cli/internal/detect/detect_test.go`

All tests use `t.TempDir()` + riteway assertions.

| Case | Setup | Expected |
|------|-------|----------|
| Empty directory | No files | `[]` |
| Jest in devDeps | `package.json` w/ jest | `[jest]` |
| Vitest in devDeps | `package.json` w/ vitest | `[vitest]` |
| Jest in deps | `package.json` w/ jest in dependencies | `[jest]` |
| Both jest + vitest | Both in devDeps | `[vitest, jest]` (vitest first) |
| RSpec in Gemfile | `gem 'rspec'` | `[rspec]` |
| rspec-core in Gemfile | `gem "rspec-core"` | `[rspec]` |
| Commented gem | `# gem 'rspec'` | `[]` |
| RSpec + Jest | Both files | `[rspec, jest]` |
| Malformed JSON | Invalid `package.json` | `[]` |
| No devDeps key | `package.json` w/ `{}` | `[]` |

### `cli/cmd/quarantine/init_happy_test.go`

New interface tests:

- Detection pre-fills default: create `package.json` w/ jest, send `"\n\n\n"`,
  verify `framework: jest`.
- Multi-framework by number: both jest + vitest, send `"2\n\n\n"`, verify jest.
- Multi-framework by name: send `"jest\n\n\n"`, verify jest.
- Multi-framework accept default: send `"\n\n\n"`, verify vitest.
- Override detected: jest detected, send `"rspec\n\n\n"`, verify rspec.

Existing tests unaffected (empty temp dirs → Case C).

## ADRs

Use `/create-adr` to create an ADR for:

- **Framework auto-detection design**: documents the scope decisions (no
  package manager detection in v1, no JUnit XML detection in v1), the
  `Scan()` never-error API, vitest-over-jest priority, and the rationale
  that detection is advisory (never blocks init).

## User Scenarios

Use `/create-user-scenario` to author Given/When/Then scenarios before
implementation. Scenarios to create:

- Init with single detected framework (jest from package.json) — user accepts
  default with Enter.
- Init with single detected framework — user overrides with a different name.
- Init with multiple detected frameworks — user selects by number.
- Init with multiple detected frameworks — user selects by name.
- Init with no detectable framework — identical to current behavior.
- Init with malformed package.json — detection silently skipped, normal prompt.

## Workflow

Use `/mikey:tdd` for all code implementation:
- `detect` package (Scan function + all detection logic)
- `init.go` prompt changes (promptFramework helper)

After each package is implemented, run `/mikey:testify --with-design` to
verify tests align with the test philosophy and check for code design issues
(I/O mixed with logic, etc.):
- `/mikey:testify --with-design` on `cli/internal/detect/detect_test.go`
- `/mikey:testify --with-design` on `cli/cmd/quarantine/init_test.go` (and
  related test files)

## Implementation Sequence

1. `/create-adr` — framework auto-detection design decisions
2. `/create-user-scenario` — author detection scenarios
3. Create `detect` package + tests (`/mikey:tdd`)
4. `/mikey:testify --with-design` on detect package
5. Modify `init.go`: move `os.Getwd()`, add `detect.Scan()`, extract
   `promptFramework()`, handle three cases (`/mikey:tdd`)
6. `/mikey:testify --with-design` on init tests
7. Verify all existing tests pass
8. Add interface tests
9. Update documentation
