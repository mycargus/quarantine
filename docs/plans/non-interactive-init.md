# Plan: Non-Interactive `quarantine init` (`--yes` + Flag Overrides)

## Motivation

ADR-019 deferred `--yes` to v2. However, CI-as-code automation (Terraform,
Ansible, Dockerfiles, GitHub Actions setup steps) is a blocking use case.
This plan pulls `--yes` into v1 and updates ADR-019 accordingly.

## Flags

Register in `newInitCmd()` in `main.go`:

| Flag | Type | Short | Default | Description |
|------|------|-------|---------|-------------|
| `--yes` | bool | `-y` | false | Skip all prompts. Requires `--framework`. |
| `--framework` | string | — | `""` | Test framework: rspec, jest, or vitest. |
| `--retries` | int | — | 0 (sentinel) | Retry count (1-10, default 3). |
| `--junitxml` | string | — | `""` | JUnit XML glob path. |

Deliberately omitted: `--config` (init always writes `quarantine.yml` in cwd).

## Behavior

### `--yes` Mode (Non-Interactive)

- `--framework` **required**. Missing → exit 2.
- Invalid `--framework` → exit 2.
- `--retries`: if 0 (not provided), default to 3. If provided, must be 1-10.
- `--junitxml`: if empty, use `config.FrameworkDefaultJUnit(framework)`.
- Existing `quarantine.yml`: overwrite silently, log
  `"quarantine.yml already exists, overwriting."` to stdout.
- No stdin read. No `bufio.Reader` created.
- Error messages go to stderr (via `cmd.PrintErrf`).

### Flags Without `--yes` (Partial Override)

Each provided flag **overrides** that value and **skips its prompt**. Prompts
still appear for unspecified values.

- `quarantine init --framework jest` → skips framework prompt, prompts
  retries + junitxml.
- `quarantine init --framework jest --retries 5` → prompts only junitxml.
- `quarantine init --framework invalid` → exit 2 immediately (no fallthrough
  to prompt).

### Error Messages

```
Error: --framework is required when using --yes. Supported: rspec, jest, vitest.
Error: invalid framework 'pytest'. Supported: rspec, jest, vitest.
Error: --retries value 11 is out of range. Must be between 1 and 10.
```

## Prerequisite: Fix Retries Range Validation

Current interactive `parseRetriesInput` (init.go lines 177-186) accepts ANY
integer with no range check. This creates inconsistency: init can write values
that `doctor` and `run` later reject.

**Fix (separate commit):** After `parseRetriesInput` returns, check
`retries < 1 || retries > 10`. If out of range, print warning and re-prompt.
Apply to BOTH interactive and non-interactive paths.

## TTY Detection

Check whether Cobra's input has been overridden before inspecting:

```go
reader := cmd.InOrStdin()
if reader == os.Stdin {
    info, _ := os.Stdin.Stat()
    if info != nil && (info.Mode()&os.ModeCharDevice) == 0 {
        cmd.PrintErrln("Warning: stdin is not a terminal. Use --yes for non-interactive mode.")
    }
}
```

- Uses `os.Stdin.Stat()` + `ModeCharDevice` — no new dependency.
- When `cmd.SetIn()` was called (all tests), `reader != os.Stdin` → skip check.
- Warning only, never blocks execution.

## `executeInitCmd` Signature Change

```go
// Before:
func executeInitCmd(t *testing.T, stdin string, configDir string, env map[string]string) (stdout string, err error)

// After:
func executeInitCmd(t *testing.T, stdin string, args []string, configDir string, env map[string]string) (stdout string, err error)
```

Inside: `rootCmd.SetArgs(append([]string{"init"}, args...))`.
All existing callers pass `nil` for `args`.

## ADRs

Use `/create-adr` to create ADRs for:

- **Non-interactive init (`--yes`)**: documents the scope change from v2 to v1,
  the rationale (CI-as-code automation is a blocking use case), flag semantics
  (flags skip prompts, not pre-fill), `--retries 0` sentinel convention, and
  TTY detection as warning-only. This supersedes the "deferred to v2" note in
  ADR-019.

## User Scenarios

Use `/create-user-scenario` to author Given/When/Then scenarios before
implementation. Scenarios to create:

- `quarantine init --yes --framework jest` creates config non-interactively.
- `quarantine init --yes` without `--framework` exits with error.
- `quarantine init --yes --framework jest` overwrites existing config with log
  message.
- `quarantine init --framework jest` (no --yes) skips framework prompt, still
  prompts retries and junitxml.
- `quarantine init --yes --framework jest --retries 11` exits with range error.
- `quarantine init` on piped stdin without `--yes` warns about non-TTY.
- Interactive retries prompt rejects out-of-range values (1-10 validation).

## Workflow

Use `/mikey:tdd` for all code implementation:
- Retries range validation fix
- `--yes` non-interactive mode
- Flag overrides without `--yes`
- TTY detection

After each major step, run `/mikey:testify --with-design` to verify tests
align with the test philosophy and check for code design issues:
- `/mikey:testify --with-design` on `cli/cmd/quarantine/init_test.go` (and
  related test files) after retries fix
- `/mikey:testify --with-design` on init test files after `--yes` implementation

## Implementation Steps

### Commit 0: `/create-adr` — non-interactive init design decisions

### Commit 0.5: `/create-user-scenario` — author non-interactive scenarios

### Commit 1: Fix retries range validation in interactive init (`/mikey:tdd`)

Add 1-10 range check to interactive retries prompt loop. Add test for
out-of-range interactive input.

### Commit 1.5: `/mikey:testify --with-design` on init tests

### Commit 2: Update `executeInitCmd` to accept args parameter

Signature change + update all ~24 callers with `nil`. Pure refactor.

### Commit 3: Register flags on init command

Add `--yes`, `--framework`, `--retries`, `--junitxml` to `newInitCmd()`.
Not yet wired into `runInit`.

### Commit 4: Implement `--yes` non-interactive mode (`/mikey:tdd`)

Branch in `runInit`. All `--yes` test cases pass:
- `--yes --framework jest` succeeds
- `--yes --framework rspec --retries 5 --junitxml custom.xml` succeeds
- `--yes` without `--framework` exits 2
- `--yes --framework invalid` exits 2
- `--yes --framework jest --retries 11` exits 2
- Overwrite with `--yes` logs message

### Commit 5: Implement flag overrides without `--yes` (`/mikey:tdd`)

Each flag skips its prompt. Tests:
- `--framework jest` skips framework prompt
- `--framework jest --retries 5` skips both
- `--framework invalid` (no --yes) exits 2

### Commit 5.5: `/mikey:testify --with-design` on init tests

### Commit 6: Add TTY detection warning (`/mikey:tdd`)

Warn when stdin is piped without `--yes`.

### Commit 7: Update milestone docs

Remove "No `--yes` flag" exclusion from milestone index.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Flags skip prompts (not pre-fill) | Standard CLI convention: flags are authoritative |
| `--retries 0` = sentinel | Matches `run.go` pattern; 0 is never valid (range 1-10) |
| TTY = warning only | Don't block edge cases; `--yes` is the explicit opt-in |
| No `--config` on init | Different semantics (write vs read); defer to avoid scope creep |
| Errors to stderr in --yes mode | Non-interactive output should separate data from diagnostics |
