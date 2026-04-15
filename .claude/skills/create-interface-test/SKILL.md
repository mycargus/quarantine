---
name: create-interface-test
description: Create an interface test that exercises a component through its public entry point (CLI binary or HTTP routes) with external APIs mocked. Interface tests are the sweet spot of the test pyramid — use when a scenario has user-facing behavior.
argument-hint: "<description of the behavior to interface-test>"
model: sonnet
effort: medium
context: fork
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, Bash, Agent
---

Create an interface test for the described behavior.

## Core principle

**Interface tests exercise a single component through its public entry point** — the CLI binary or HTTP routes — **with external APIs replaced by mock servers.** Internal dependencies (SQLite, file system, config parsing) stay real. This is the MTP Interface layer: the sweet spot that catches the most bugs per test.

## What interface tests are NOT

| If you need this... | Use this layer instead |
|---|---|
| Test pure logic (parsing, merging, validation) with no I/O | Unit test (no mocks needed) |
| Verify request/response shapes against OpenAPI specs | Contract test (`/create-contract-test`) |
| Observe real API behavior (stateful round-trips, redirects, auth) | E2E test (`/create-e2e-test`) |

If the behavior can be adequately tested at the interface level, it MUST be an interface test — not E2E and not unit. Interface tests are the default for any scenario with user-facing behavior.

## Step 1 — Identify the component

Determine which component the behavior belongs to:

| Signal | Component | Go to section |
|--------|-----------|---------------|
| Scenario mentions CLI commands (`quarantine run`, `quarantine init`) | CLI (Go) | CLI Interface Tests |
| Scenario mentions HTTP routes, dashboard views, or UI behavior | Dashboard (TS) | Dashboard Interface Tests |

## Step 2 — Check existing interface tests

Search for existing tests that already cover this behavior:

- CLI: `cli/cmd/quarantine/*_test.go`
- Dashboard: `dashboard/test/*.interface.test.ts`

Don't duplicate coverage. Extend an existing test file if the behavior fits.

---

## CLI Interface Tests (Go)

### Location and framework

- File: `cli/cmd/quarantine/<feature>_test.go` (colocated with the command source)
- Framework: Go `testing` + `riteway-golang`
- Assertion: `riteway.Assert(t, riteway.Case[T]{...})`

### Pattern: Cobra root command with mock HTTP

The CLI entry point is `newRootCmd().Execute()`. Interface tests create a fresh root command, set args and env vars, and capture output:

```go
func executeRunCmd(t *testing.T, args []string, env map[string]string) (output string, exitErr error) {
    t.Helper()
    for k, v := range env {
        t.Setenv(k, v)
    }
    rootCmd := newRootCmd()
    buf := &bytes.Buffer{}
    rootCmd.SetOut(buf)
    rootCmd.SetErr(buf)
    rootCmd.SetArgs(append([]string{"run"}, args...))
    exitErr = rootCmd.Execute()
    return buf.String(), exitErr
}
```

External APIs are replaced by `httptest.NewServer`:

```go
func fakeGitHubAPI(t *testing.T, branchExists bool) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Route by URL path, return canned JSON responses
    }))
}
```

### Existing helpers

Read these from `cli/cmd/quarantine/run_test.go` before writing new tests:

| Helper | Purpose |
|--------|---------|
| `executeRunCmd(t, args, env)` | Run the `run` subcommand, capture output |
| `executeInitCmd(t, args, env)` | Run the `init` subcommand, capture output |
| `fakeGitHubAPI(t, branchExists)` | Mock GitHub branch-check endpoint |
| `writeSuiteConfigFull(t, dir, ...)` | Create a `.quarantine/config.yml` with all fields |
| `writeSuiteConfigWithRetries(t, dir, ...)` | Config with custom retry count |
| `writeSuiteConfigWithRerunScript(t, dir, ...)` | Config with custom rerun command |

### Key patterns

1. **Entry point**: `newRootCmd().Execute()` — exercises the real cobra command tree in-process
2. **Mock external APIs**: `httptest.NewServer` returning canned JSON. Point CLI at mock via `QUARANTINE_GITHUB_API_URL` env var.
3. **Real internal dependencies**: Config parsing, JUnit XML parsing, quarantine state merging, git operations all use real code.
4. **Temporary file system**: `t.TempDir()` for config files, JUnit XML, test scripts.
5. **Exit code assertions**: Check the returned `error` to verify exit codes (0 = nil, 1 = test failures, 2 = quarantine error).
6. **Output assertions**: Capture stdout/stderr via `rootCmd.SetOut(buf)` / `rootCmd.SetErr(buf)`.
7. **Output routing**: `run` writes diagnostics to stderr; `init` writes to stdout. Assert on the right stream.

### CLI test template

```go
func TestRunCommand_DescriptiveName(t *testing.T) {
    // Arrange: mock GitHub API
    gh := fakeGitHubAPI(t, true)
    defer gh.Close()

    // Arrange: temp dir with config and test script
    dir := t.TempDir()
    xmlPath := filepath.Join(dir, "results.xml")
    scriptPath := writeTestScript(t, dir, xmlPath, 0) // exit 0
    writeSuiteConfigFull(t, dir, "owner", "repo", xmlPath, scriptPath, scriptPath)

    // Act
    output, err := executeRunCmd(t, []string{"unit"}, map[string]string{
        "QUARANTINE_GITHUB_TOKEN":   "fake-token",
        "QUARANTINE_GITHUB_API_URL": gh.URL,
    })

    // Assert
    riteway.Assert(t, riteway.Case[error]{
        Given:    "a passing test suite with quarantine state branch",
        Should:   "exit without error",
        Actual:   err,
        Expected: nil,
    })
}
```

---

## Dashboard Interface Tests (TypeScript)

### Location and framework

- File: `dashboard/test/<descriptive-name>.interface.test.ts`
- Framework: `riteway` (`describe` + `assert`)
- Assertion: `assert({ given, should, actual, expected })`

### Pattern: createTestApp + router.fetch

The dashboard entry point is `router.fetch(new Request(...))`. Interface tests create an isolated app instance with a temp config and SQLite database:

```typescript
import { describe } from "riteway"
import { createTestApp, seedTestDb } from "./helpers.js"
import { bodyText } from "../app/test-helpers.js"

describe("GET /route — context", async (assert) => {
  const { router, dbPath, cleanup } = createTestApp({ repos: [...] })
  seedTestDb(dbPath, [...])

  try {
    const response = await router.fetch(new Request("http://localhost/route"))
    const html = await bodyText(response)

    assert({
      given: "context description",
      should: "expected behavior",
      actual: response.status,
      expected: 200,
    })
  } finally {
    cleanup()
  }
})
```

### Existing helpers

Read these from `dashboard/test/helpers.ts` before writing new tests:

| Helper | Purpose |
|--------|---------|
| `createTestApp(opts)` | Creates router + temp config + temp DB. Returns `{ router, dbPath, configPath, cleanup }` |
| `seedTestDb(dbPath, seeds)` | Inserts projects and quarantined test rows into temp SQLite DB |

### Key patterns

1. **Entry point**: `router.fetch(new Request(...))` — exercises the full Remix routing stack (route matching, param extraction, controller, rendering).
2. **Mock external APIs**: `token: ""` prevents GitHub sync. No mock HTTP server needed.
3. **Real internal dependencies**: SQLite (temp file per test), config parsing, filtering, rendering.
4. **Test data**: `seedTestDb(dbPath, [...])` pre-populates the database with projects and tests.
5. **Cleanup**: `finally { cleanup() }` removes temp config and DB files.
6. **Response assertions**: Check `response.status` and parse HTML via `bodyText()`.
7. **Describe blocks**: One `describe` per route + condition combination (e.g., `"GET / — valid config, empty repos"`).

### Dashboard test template

```typescript
import { describe } from "riteway"
import { createTestApp, seedTestDb } from "./helpers.js"
import { bodyText } from "../app/test-helpers.js"

describe("GET /route — scenario description", async (assert) => {
  const repos = [{ owner: "acme", repo: "api" }]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [
    {
      owner: "acme",
      repo: "api",
      tests: [
        {
          testId: "suite::test::name",
          name: "test name",
          quarantinedAt: "2026-01-01T00:00:00Z",
        },
      ],
    },
  ])

  try {
    const response = await router.fetch(new Request("http://localhost/route"))
    const html = await bodyText(response)

    assert({
      given: "a GET /route with seeded data",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET /route with seeded data",
      should: "include the test name in the response",
      actual: html.includes("test name"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})
```

---

## Step 3 — Write the test

Follow the component-specific pattern above. Key rules:

1. **Exercise the real entry point** — `rootCmd.Execute()` for CLI, `router.fetch()` for Dashboard
2. **Mock only external services** — `httptest.NewServer` for GitHub API (CLI), `token: ""` for GitHub (Dashboard)
3. **Keep internal dependencies real** — SQLite, config parsing, XML parsing, file system
4. **Use RITEway assertions** — `given`, `should`, `actual`, `expected`
5. **Isolate test state** — temp directories, temp DB files, cleanup in `finally`/`defer`
6. **One behavior per test** — each `assert` block tests one observable outcome

## Step 4 — Run and verify

- CLI: `make cli-test`
- Dashboard: `make dash-test`

## Step 5 — Lint

- CLI: `make cli-lint`
- Dashboard: `make dash-lint`

Fix any issues before committing.

## Anti-patterns to reject

| Anti-pattern | Why it's wrong | Fix |
|---|---|---|
| Calling internal Go functions directly | Tests implementation, not the interface | Use `newRootCmd().Execute()` |
| Importing internal packages in test | Bypasses the public entry point | Use `executeRunCmd` / `executeInitCmd` helpers |
| Calling controllers directly (Dashboard) | Skips routing, param extraction | Use `router.fetch(new Request(...))` |
| Mocking internal dependencies (config parser, DB, XML parser) | Hides real integration bugs | Use real dependencies; mock only external APIs |
| Hitting real external APIs (GitHub) | That's E2E, not interface | Use `httptest.NewServer` or `token: ""` |
| Testing only exit codes / status codes | Incomplete — misses output regressions | Assert on both status AND response content |
| Shared state between tests | Breaks isolation, causes flaky tests | Use `t.TempDir()` / temp files per test |
| No cleanup | File/process leaks in CI | Use `defer`/`finally` for cleanup |
| `t.Skip()` or conditional guards | Silently hides missing coverage | Hard-fail with a descriptive assertion |
