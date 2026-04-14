# Plan: MTP Adherence — Full Test Pyramid Coverage

> **Status:** Active
>
> **Trigger:** Gap analysis against Mikey's Test Pyramid (MTP).
> test-strategy.md terminology already aligned (committed separately).
> This plan covers the remaining structural and process gaps.

## Context

The Quarantine project's CLI has a healthy pyramid: Unit (500+), Interface
(binary invocation + mock HTTP), Contract (Prism), E2E (real GitHub). The
Dashboard has only Unit tests (sociable — controllers and lib functions called
directly with real SQLite). It has no Interface tests (HTTP requests to Remix
routes) and no E2E tests (real GitHub Artifacts).

`contracts.md` is stale regarding completed ADR-025 work, and one convention
contract is unvalidated.

## Remaining Gaps

| Gap | MTP Layer | Severity | Effort |
|-----|-----------|----------|--------|
| contracts.md stale on ADR-025 status | Contract | Low | Trivial |
| Artifact naming prefix convention unvalidated | Contract | Medium | Small |
| Dashboard Interface tests don't exist | Interface | High | Medium |
| Dashboard E2E tests don't exist | E2E | High | Medium |

**Dropped from scope:** Test ID format cross-component test. The `test_id`
field in `results.json` is schema-validated (ADR-025). The Dashboard treats it
as an opaque string — it stores and displays it but never parses the `::`
delimiter. The contract is already protected by schema validation + parser
unit tests.

## Phase 1: Close Contract Gaps

**Goal:** Every intra-service contract documented in `contracts.md` either has
validation or an explicit rationale for skipping.

### 1.1 Update contracts.md — reflect completed ADR-025 work

The schema summary table says "Go marshal validation planned" for
`test-result.schema.json`. This is done — `result_schema_test.go` validates Go
marshal output against the schema with 7 test cases (positive, negative, flaky,
quarantined, error-mapped-to-failed, all-retries-fail, missing-field).

**Action:** Update the "Tested" column for `test-result.schema.json` in the
schema table and the `results.json` contract detail section.

### 1.2 Artifact naming prefix validation

**Contract:** CI workflow uploads artifacts named
`quarantine-results-{suite-name}-{run_id}`. Dashboard's
`filterArtifactsByPrefix()` in `sync.server.ts` filters by the
`"quarantine-results"` prefix (hardcoded string literal on line 41).

**Risk:** If someone changes the prefix in `sync.server.ts`, the Dashboard
silently stops ingesting artifacts. The existing unit tests for
`filterArtifactsByPrefix()` use their own fixture strings — they'd still pass
even if the production prefix changed.

**Why not a cross-component contract test:** The producer is CI workflow YAML,
not testable code. Pact/Prism don't apply. This is a naming convention, not a
data schema.

**Approach:**
1. Extract the prefix to a named constant in the Dashboard
   (e.g., `ARTIFACT_PREFIX = "quarantine-results"` in a shared constants
   module).
2. Update `sync.server.ts` to use the constant.
3. Add a unit test asserting the constant's value matches the documented
   convention.
4. Update unit tests for `filterArtifactsByPrefix()` to reference the constant
   instead of hardcoded strings.
5. Update `contracts.md` "Tested" column to note the validation.

**Location:** Dashboard code + unit tests (not `test/contract/`).

## Phase 2: Dashboard Interface Tests

**Goal:** Exercise the Dashboard through its HTTP interface — the same entry
point real users interact with. This is the MTP's "sweet spot" layer.

### Decisions (resolved)

**Interface test boundary: `router.fetch()`**, not a real HTTP server.
`router.fetch(new Request(...))` exercises route matching, parameter extraction,
controller invocation, and response rendering — everything except TCP, which is
Remix/Node infrastructure we trust. This is analogous to how CLI Interface tests
use `exec.Command` without testing TCP.

**Test isolation: per-test temp files.** Each test creates its own `configPath`
(via `tmpdir()` + `Date.now()`) and `dbPath` (same pattern). This is the
existing pattern in `home.test.ts`. No shared state between tests.

**No sync during Interface tests.** Interface tests exercise the request →
response path with pre-seeded SQLite data. GitHub API calls are not triggered.
The `token` option in `HomeOptions` controls whether sync is attempted — omit
it to prevent network calls.

### 2.1 Extract `createApp()` factory from server.ts

**Current state:** `server.ts` creates the router, maps routes, and starts the
HTTP server all inline.

**Controller signatures (already support injection):**
- `home(options: HomeOptions = {})` — `HomeOptions` has `configPath?`,
  `dbPath?`, `token?`, `fetchFn?`
- `project(owner, repo, url, dbPath?)` — 4th param already optional

**Refactor:**
```
app/app.ts   — exports createApp(opts?) → router
app/server.ts — imports createApp(), starts HTTP server (3 lines)
```

```typescript
// app/app.ts
import { createRouter } from "remix/fetch-router"
import { home } from "./controllers/home.js"
import { project } from "./controllers/project.js"
import { routes } from "./routes.js"

export interface AppOptions {
  configPath?: string
  dbPath?: string
  token?: string
  fetchFn?: typeof fetch
}

export function createApp(opts: AppOptions = {}) {
  const router = createRouter()
  router.map(routes, {
    actions: {
      home: () => home(opts),
      projectDetail: (ctx) =>
        project(ctx.params.owner, ctx.params.repo, ctx.request.url, opts.dbPath),
    },
  })
  return router
}
```

```typescript
// app/server.ts (thin shell)
import * as http from "node:http"
import { createRequestListener } from "remix/node-fetch-server"
import { createApp } from "./app.js"

const router = createApp()
const port = Number(process.env.PORT ?? 3000)
const server = http.createServer(createRequestListener((req) => router.fetch(req)))
server.listen(port, () => {
  console.log(`Quarantine dashboard running at http://localhost:${port}`)
})
```

**Risk:** Minimal. This is a pure extraction — no behavior change. The existing
controller tests continue to call `home()` and `project()` directly (unit
layer). Interface tests use `createApp()` (interface layer).

### 2.2 Create dashboard/test/ directory and infrastructure

```
dashboard/test/
  helpers.ts                   — createTestApp(), seedTestDb(), bodyText()
  home.interface.test.ts       — GET / scenarios
  project.interface.test.ts    — GET /projects/:owner/:repo scenarios
```

**Test helper pattern:**
```typescript
import { createApp } from "../app/app.js"

export function createTestApp(opts) {
  const configPath = writeTempConfig(opts.repos ?? [])
  const dbPath = createTempDb()
  return {
    router: createApp({ configPath, dbPath }),
    dbPath,
    configPath,
    cleanup: () => { unlinkSync(configPath); unlinkSync(dbPath) },
  }
}
```

**Makefile:** Integrate into existing `make dash-test`. The `pnpm test` script
already uses Node's `--test` runner with a glob pattern — extend it to include
`test/*.interface.test.ts` alongside `app/**/*.test.ts`. No separate target.

### 2.3 Home route Interface tests (GET /)

Test through `router.fetch(new Request("http://localhost/"))`:

- Valid config, empty repos → 200, HTML includes "Quarantine Dashboard"
- Valid config, seeded DB with projects → 200, HTML includes project names
  and quarantine counts
- Missing config file → 500, HTML includes "Configuration Error"
- Invalid config content → 500, HTML includes validation error
- Unknown route (GET /nonexistent) → 404

### 2.4 Project detail Interface tests (GET /projects/:owner/:repo)

Test through `router.fetch(new Request("http://localhost/projects/acme/repo"))`:

- Known project with test results → 200, HTML includes test names and statuses
- Known project with no results → 200, HTML shows empty state
- Unknown project → appropriate response (verify current behavior: 404 page)
- Query parameter filtering → filtered results reflected in HTML
- Route param extraction → `:owner` and `:repo` correctly extracted from URL

## Phase 3: Dashboard E2E Tests

**Goal:** Exercise the Dashboard against real external dependencies — real
GitHub Artifacts API, real artifact download, real ingest into SQLite, real
rendered output.

### Prerequisites (verify before starting)

1. **Fixture repo exists and produces artifacts.** Confirm
   `mycargus/quarantine-app-test-fixture` is operational and its CI generates
   `quarantine-results-*` artifacts. If not, set it up first.
2. **Phase 2 complete.** `createApp()` factory exists and Interface tests pass.

### Decision: `createApp()` with real GitHub config

E2E tests use `createApp({ configPath, dbPath, token })` with a real GitHub
token, pointed at the fixture repo. This exercises the full stack: real GitHub
API → real artifact download → real ZIP extraction → real SQLite ingest → real
route rendering. The only thing not exercised is TCP (same as Interface tests).

If TCP-level issues are suspected, a follow-up can add a real HTTP server test.
For now, `createApp()` provides sufficient E2E confidence.

### 3.1 E2E test infrastructure

**Location:** `test/e2e/dashboard-sync.test.js` (alongside CLI E2E tests).

**Credential handling:** Check for `QUARANTINE_GITHUB_TOKEN` and
`QUARANTINE_TEST_OWNER` / `QUARANTINE_TEST_REPO` env vars. If absent, skip
the entire suite via Vitest's `describe.skipIf()`. This matches the existing
CLI E2E pattern.

**Per-test isolation:** Temp SQLite DB per test. Temp config file pointing at
the fixture repo. Cleanup in `afterEach`.

**Timeout:** 120 seconds per test (same as CLI E2E), since artifact download
involves real network I/O.

### 3.2 Core E2E scenarios

1. **Sync happy path:** Dashboard polls fixture repo, downloads artifacts,
   ingests into SQLite. GET / shows the project with correct quarantine counts.
   GET /projects/:owner/:repo shows individual test results.

2. **Incremental sync:** First sync ingests all artifacts. Second sync only
   ingests new artifacts (since-based filtering). Verify no duplicate rows.

3. **Empty state → populated:** New Dashboard with no prior sync. GET / shows
   empty. Trigger sync. GET / shows populated.

### 3.3 CI integration

- Add to existing `e2e` CI job (shared credentials, shared fixture repo).
- Same skip-when-no-credentials behavior as CLI E2E.
- Sequential execution (not parallelized with CLI E2E) to avoid rate limit
  contention.

## Phase 4: Ongoing Adherence

**Goal:** Prevent regression — new code should maintain the full pyramid.

### 4.1 Add MTP layer checklist to test strategy

Add a brief "New Feature Checklist" section to `test-strategy.md`:

> When adding a new feature or fixing a bug:
> 1. **Unit:** Pure logic has unit tests. No mocks needed.
> 2. **Interface:** If the feature has user-facing behavior, test through the
>    public interface (CLI binary or HTTP routes).
> 3. **Contract:** If the feature changes a data format exchanged between
>    components, update contract tests.
> 4. **E2E:** If the feature changes interaction with real external services,
>    verify in E2E.
> 5. **Bug replication (Principle 9):** If a bug was found at a higher layer,
>    write a lower-layer test first.

## Execution Order

Phases are sequential — each builds on the previous:

1. **Phase 1** first: trivial doc update + small constant extraction. No risk.
2. **Phase 2** next: the MTP "sweet spot". Highest return on investment.
   Requires `createApp()` extraction (2.1) before tests can be written (2.2-2.4).
3. **Phase 3** after Phase 2 is stable: E2E tests reuse `createApp()` from
   Phase 2. The Interface tests reduce the number of E2E scenarios needed
   (per MTP: push tests down the pyramid).
4. **Phase 4** is a doc update — can be done alongside any phase.
