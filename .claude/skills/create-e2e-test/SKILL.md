---
name: create-e2e-test
description: Create an E2E test in test/e2e/ that verifies real external API behavior matches what integration test mocks assume. Use when a scenario introduces external API interactions (GitHub, Jenkins, GitLab, etc.).
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
3. **All components share `test/e2e/`.** CLI, dashboard, and shared library API interactions all live in `test/e2e/`. "No E2E infrastructure for this component" is never a valid excuse.
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
- Conditional request round-trips (ETag/If-None-Match, Last-Modified/If-Modified-Since)
- Redirect chains (302 → blob storage download)
- Search/query API formats (query string must match what the provider actually indexes)
- Pagination (code assumes single page with `per_page=100` or similar)
- Sequential state (second call depends on state from first call)
- Auth header formats (Bearer vs token vs Basic — varies by provider)

**Low-risk interactions** (skip E2E):
- Status code checks (`if (response.status === 401)`) — no shape to drift
- Pure client-side logic — no external API involved

## Step 2 — Read existing E2E tests for overlap

```
test/e2e/*.test.js
```

Check whether any existing test already exercises the same API interaction. Don't duplicate coverage.

## Step 3 — Determine the provider and credentials

Read `test/e2e/.env.example` for the currently supported env vars. Each external provider needs its own credentials and test fixture target.

### Currently supported

| Provider | Token env var | Target env vars | API base |
|----------|---------------|-----------------|----------|
| GitHub | `QUARANTINE_GITHUB_TOKEN` | `QUARANTINE_TEST_OWNER`, `QUARANTINE_TEST_REPO` | `https://api.github.com` |

### Adding a new provider

When the test targets a provider not yet in `test/e2e/.env.example`:

1. Add the new env vars to `test/e2e/.env.example` with comments explaining what they are
2. Guard your test in `beforeAll` — fail with a clear message if the required vars are missing
3. Keep provider-specific helpers (API request wrappers) local to the test file — don't force all tests to import every provider's helpers

## Step 4 — Write the test

### Location and framework

- File: `test/e2e/<provider>-<feature>.test.js` (e.g., `github-artifacts.test.js`, `jenkins-builds.test.js`)
- Framework: `vitest` + `riteway/vitest`
- Imports:
  ```js
  import { assert } from "riteway/vitest"
  import { beforeAll, describe, test } from "vitest"
  ```

### Credential guard

Fail immediately if required env vars are missing:

```js
beforeAll(() => {
  if (!token || !baseUrl) {
    throw new Error(
      "E2E tests require <LIST_REQUIRED_VARS>. See test/e2e/.env.example."
    )
  }
})
```

### API request helper

Create a provider-specific helper local to the test file. Match the provider's auth and header conventions:

**GitHub:**
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

**Jenkins (example pattern):**
```js
async function jenkinsRequest(method, path, headers = {}) {
  return fetch(`${jenkinsUrl}${path}`, {
    method,
    headers: {
      Authorization: `Basic ${Buffer.from(`${user}:${apiToken}`).toString("base64")}`,
      Accept: "application/json",
      ...headers,
    },
  })
}
```

**GitLab (example pattern):**
```js
async function gitlabRequest(method, path, headers = {}) {
  return fetch(`${gitlabUrl}/api/v4${path}`, {
    method,
    headers: {
      "PRIVATE-TOKEN": token,
      Accept: "application/json",
      ...headers,
    },
  })
}
```

These are starting patterns — read the provider's actual API docs before writing. Don't assume the example is correct.

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

When verifying state created by a prior step (search index propagation, CDN caching, Jenkins build queue):

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
  // Close/delete any resources created during the test
})
```

## Step 5 — Run and verify

```bash
make e2e-test
```

E2E tests require real network access and valid credentials. They run sequentially (`fileParallelism: false` in vitest config) because tests may share external state.

## Step 6 — Lint

```bash
cd test && pnpm run lint
```

Fix any issues before committing.

## Anti-patterns to reject

| Anti-pattern | Why it's wrong | Fix |
|---|---|---|
| `t.skip("no data")` | Silently stops verifying | Assert the precondition instead |
| `if (arr.length > 0) { assert(...) }` | Silently passes when empty | Assert `arr.length >= 1` first |
| "Mocks cover this" | Mocks verify YOUR assumptions, not reality | E2E verifies the API's actual behavior |
| "No E2E infra for this component" | `test/e2e/` serves all components | Write the test in `test/e2e/` |
| "This provider isn't set up yet" | Add the env vars and write the test | See Step 3: Adding a new provider |
| Importing production code | Creates coupling between test and implementation | Use `fetch` directly with a provider-specific helper |
| Testing implementation details | E2E should verify observable API contracts | Test response shapes, not internal function calls |
| Hardcoding a provider's base URL | Breaks when targeting different instances | Use env vars for base URLs (especially Jenkins/GitLab which are self-hosted) |
