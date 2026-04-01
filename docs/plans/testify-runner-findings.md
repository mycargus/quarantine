# Plan: Runner Package Test Gaps (Testify Findings)

All five findings are test-only changes. No production code changes. Tests use
riteway assertions. All changes in `cli/internal/runner/runner_test.go`.

## Imports

Add `"fmt"` to the import block in the commit that introduces `errWriter`.
Add `"time"` in the commit that introduces `TestRunContextCancellation`.
Do NOT add both at once — unused imports cause compilation errors.

## errWriter Type

Define after imports, before `TestRunStartFails`. First test-local type in this
file.

```go
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
    return 0, fmt.Errorf("errWriter: forced write error")
}
```

## Finding 1 (MEDIUM): Backslash Escaping in `TestEscapeJestPattern`

**Problem:** `jestRegexSpecialChars` starts with `"\\"` to prevent
double-escaping, but no test asserts backslash behavior.

**What to add:** Append two assertions to `TestEscapeJestPattern`:

1. Standalone backslash: `"foo\\bar"` → raw `` `foo\\bar` ``
2. Backslash + dot (order matters): `"\\.bar"` → raw `` `\\\.bar` ``

Use raw strings for `Should` and `Expected` fields.

**Mutations killed:** Removing `"\\"` from the slice, reordering the slice.

## Finding 2 (LOW): Unclosed Quotes in `TestSplitShellArgs`

**Problem:** `SplitShellArgs` silently treats unclosed-quote content as a
single token. No test asserts this.

**What to add:** Append two assertions to `TestSplitShellArgs`:

1. Unclosed double: `` `cmd "hello world` `` → `["cmd", "hello world"]`
2. Unclosed single: `"cmd 'hello world"` → `["cmd", "hello world"]`

Both paths tested because `inSingle`/`inDouble` are independent flags.

## Finding 3 (LOW): Non-ExitError Wait Branch (runner.go line 61)

**Problem:** Line 61 handles `cmd.Wait()` returning an error that is NOT
`*exec.ExitError`. Returns `(-1, error)`. Untested.

**What to add:** New `TestRunWaitNonExitError` function.

- Run `echo hello` with `errWriter{}` as stdout, `io.Discard` as stderr.
- Assert exit code -1 and error non-nil.
- Comment notes the test depends on `echo hello` producing at least one byte.

**Mechanism:** When `cmd.Stdout` is `errWriter`, the exec package's I/O copy
goroutine gets the write error. `cmd.Wait()` returns it as a non-ExitError,
hitting line 61.

## Finding 4 (LOW): Context Cancellation (runner.go line 36)

**Problem:** Line 36 uses `exec.CommandContext(ctx, ...)`. No test verifies
that context cancellation kills the child process.

**IMPORTANT:** Do NOT use a pre-cancelled context. A pre-cancelled context
causes `Start()` to fail immediately, hitting the Start failure path (line
40-42) — the same path as `TestRunStartFails`. This does NOT test
`CommandContext`'s kill behavior.

**What to add:** New `TestRunContextCancellation` function.

- `context.WithTimeout(context.Background(), 100*time.Millisecond)`
- Run `sleep 30`
- Assert elapsed < 2 seconds (proves process was killed)
- Assert exit code != 0

The timeout fires while the process is running. `CommandContext` sends SIGKILL.

## Finding 5 (LOW): Framework Parameter Clarity

**Problem:** Custom template tests pass specific framework values when
framework is ignored (`customTemplate != ""` returns before `switch fw`).

**What to change:** Replace framework argument with `""` at lines 87, 103,
117, 131, 145. Pure clarity improvement.

## Workflow

Use `/mikey:tdd` for each finding — write the failing test first, then verify
it passes (no production code changes needed, but the TDD discipline catches
assertion mistakes early).

After all findings are committed, run `/mikey:testify --with-design` on
`cli/internal/runner/runner_test.go` to verify the new tests align with the
test philosophy and no further design issues exist.

## Commit Sequence

1. Finding 1: Add backslash assertions to `TestEscapeJestPattern`
2. Finding 2: Add unclosed-quote assertions to `TestSplitShellArgs`
3. Finding 3: Add `"fmt"` import + `errWriter` type + `TestRunWaitNonExitError`
4. Finding 4: Add `"time"` import + `TestRunContextCancellation`
5. Finding 5: Replace framework values with `""` in custom template tests

Run `make cli-test` after each commit.
