---
name: create-e2e-test
description: Create an E2E test in e2e/ that verifies real GitHub API behavior matches what integration test mocks assume. Use when a scenario introduces external API interactions.
argument-hint: "<description of what to test>"
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, Bash, Agent
---

Create an E2E test for the described interaction.

E2E tests verify that **real external APIs** behave as integration test mocks assume. They catch mock-fidelity drift: response shape changes, header behavior, redirect chains, pagination, and eventual consistency.

## When to use this skill

Any time production code calls an external API — even if integration tests use injected mocks (e.g., `fetchFn`). Dependency injection makes unit testing possible; it does **not** eliminate mock-fidelity risk. The real API is the source of truth.

## Principles

1. **Tests MUST NOT skip.** Never use `t.skip()`. If a precondition isn't met, fail with a descriptive assertion — that precondition failure is a real problem.
2. **Tests MUST NOT guard with `if`.** Don't wrap assertions in `if (data.length > 0)`. Assert the precondition, then assert the behavior.
3. **All components share `e2e/`.** CLI, dashboard, and shared library API interactions all live in `e2e/`. "No E2E infrastructure for this component" is never a valid excuse.
4. **Test the production code path, not the mock.** The point is to verify what the mock assumed. If the mock returns `{ artifacts: [...] }`, the E2E test verifies the real API returns that shape.
5. **Clean up after yourself.** Resources created during the test (issues, comments, branches) must be closed/deleted in `afterEach`.

## Step 1 — Identify the mock-fidelity risk

Read the production code that makes external API calls. For each call, answer:

- What endpoint does it hit? (method + URL pattern)
- What does the integration test mock return? (response shape, headers, status codes)
- Could the real API diverge from this mock? (shape drift, pagination, redirect behavior, header format)

If the answer to the third question is "yes" for any call, that call needs E2E coverage.

**High-risk interactions** (always need E2E):
- Response shapes the code destructures (e.g., `data.artifacts`, `response.headers.get("etag")`)
- ETag/conditional request round-trips (If-None-Match → 304)
- Redirect chains (302 → blob storage download)
- Search API query format (query string must match what GitHub indexes)
- Pagination (code assumes single page with `per_page=100`)
- Sequential state (second call depends on state from first call)

**Low-risk interactions** (skip E2E):
- Status code checks (`if (response.status === 401)`) — no shape to drift
- Pure client-side logic — no external API involved

## Step 2 — Read existing E2E tests for overlap

```
e2e/*.test.js
```

Check whether any existing test already exercises the same API interaction. Don't duplicate coverage.

## Step 3 — Write the test

### Location and framework

- File: `e2e/<component>-<feature>.test.js` (e.g., `dashboard-artifacts.test.js`)
- Framework: `vitest` + `riteway/vitest`
- Imports:
  ```js
  import { assert } from "riteway/vitest"
  import { beforeAll, describe, test } from "vitest"
  ```

### Environment variables

All E2E tests use these env vars (loaded from `e2e/.env` by `vitest.config.js`):

```js
const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO
```

Fail in `beforeAll` if any are missing — don't let tests run without credentials.

### GitHub API helper

Use the standard pattern from existing tests:

```js
async function ghRequest(method, path, headers = {}) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
      ...headers,
    },
  })
}
```

### Assertion style

Use riteway's Given/Should/Actual/Expected:

```js
assert({
  given: "a GET request to the artifacts API",
  should: "return status 200",
  actual: res.status,
  expected: 200,
})
```

### Retry pattern for eventual consistency

When verifying state created by a prior step (search index propagation, CDN caching):

```js
for (let attempt = 0; attempt < 3; attempt++) {
  if (attempt > 0) await new Promise((r) => setTimeout(r, 2000))
  // ... check for expected state
  if (found) break
}
```

### Cleanup

```js
afterEach(async () => {
  await closeIssue(issueNumber)
  // ... clean up any created resources
})
```

## Step 4 — Run and verify

```bash
make e2e-test
```

E2E tests require real network access and valid credentials. They run sequentially (`fileParallelism: false` in vitest config) because all tests share the same GitHub repo state.

## Step 5 — Lint

```bash
cd e2e && pnpm run lint
```

Fix any issues before committing.

## Anti-patterns to reject

| Anti-pattern | Why it's wrong | Fix |
|---|---|---|
| `t.skip("no data")` | Silently stops verifying | Assert the precondition instead |
| `if (arr.length > 0) { assert(...) }` | Silently passes when empty | Assert `arr.length >= 1` first |
| "Mocks cover this" | Mocks verify YOUR assumptions, not reality | E2E verifies the API's actual behavior |
| "No E2E infra for dashboard" | `e2e/` serves all components | Write the test in `e2e/` |
| Importing production code | Creates coupling between test and implementation | Use `fetch` directly, same as `ghRequest` pattern |
| Testing implementation details | E2E should verify observable API contracts | Test response shapes, not internal function calls |
