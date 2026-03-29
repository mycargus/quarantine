---
name: create-contract-test
description: Create a Prism-based contract test in test/contract/ that verifies production code sends correctly-shaped requests and handles response shapes from vendored OpenAPI specs — without network access or credentials. Use when code interacts with an external API and you want fast, offline shape validation.
argument-hint: "<description of the API interaction to contract-test>"
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, Bash, Agent
---

Create a contract test for the described API interaction.

Contract tests verify that production code sends **correctly-shaped requests** and handles **response shapes** as defined in vendored OpenAPI specs. They use Stoplight Prism as a local mock server — no network access or API credentials required. See ADR-024 (`docs/adr/024-contract-testing-tool.md`) for the full decision rationale.

## When to use this skill

Any time production code interacts with an external API and you want to verify request/response shape conformance against the vendored spec — without real network calls. Contract tests complement E2E tests:

| Concern | Contract test (this skill) | E2E test (`/create-e2e-test`) |
|---|---|---|
| Request shape (method, path, headers, body) | Yes | Indirectly |
| Response shape (fields, types, nesting) | Yes | Yes |
| Real API behavior (latency, rate limits, 304) | No | Yes |
| Credentials required | No | Yes |
| Network required | No | Yes |
| Speed | Fast (localhost Prism) | Slow (real API) |

Use contract tests when:
- Code destructures response bodies (e.g., `data.artifacts`, `issue.number`)
- Code sends specific headers the API requires (e.g., `Accept`, `X-GitHub-Api-Version`)
- Code handles error responses (410 Gone, 404, 422) based on response shape
- You want fast CI feedback without credentials

Use E2E tests (not contract tests) when:
- Behavior depends on stateful HTTP round-trips (ETag/304, pagination across pages)
- Behavior depends on real-world timing (eventual consistency, queue propagation)
- The API has no OpenAPI spec to vendor

## Principles

1. **Tests MUST NOT skip.** Never use `t.skip()`. If a precondition isn't met, fail with a descriptive assertion.
2. **Tests MUST NOT guard with `if`.** Don't wrap assertions in `if (data.length > 0)`. Assert the precondition, then assert the behavior.
3. **Assert shapes, not exact values.** Contract tests verify that fields exist and have the expected types. Don't hardcode specific response body values — Prism returns example data from the spec, which may change when the spec is updated.
4. **One spec, one source of truth.** Every contract test must reference a vendored OpenAPI spec in `schemas/`. The spec is the authority on what shapes are valid.
5. **No real API calls.** Contract tests hit localhost Prism only. If you need to verify real API behavior, use `/create-e2e-test` instead.

## Step 1 — Identify the contract risk

Read the production code that makes external API calls. For each call, answer:

- What endpoint does it hit? (method + URL pattern)
- What request headers and body does the code send?
- What response shape does the code assume? (fields it destructures, status codes it checks)
- Is there a vendored OpenAPI spec that defines this endpoint?

Focus on **shape assumptions**: field names, nesting structure, types, and required vs. optional fields.

## Step 2 — Ensure a vendored spec exists

Check `schemas/` for an existing spec that covers the provider and endpoints:

```
schemas/*.json
```

### Currently vendored

| Provider | Spec file | Covers |
|----------|-----------|--------|
| GitHub (Artifacts) | `schemas/github-api-artifacts.json` | Artifacts API subset |

### Adding a new vendored spec

When the endpoint isn't covered by an existing spec:

1. Find the provider's published OpenAPI spec (e.g., GitHub's `api.github.com.json`)
2. Extract only the paths and schemas your code uses into a new file in `schemas/`
3. Name it `schemas/<provider>-<feature>.json` (e.g., `schemas/github-api-issues.json`)
4. Validate the extracted spec loads cleanly: `cd test && ./node_modules/.bin/prism mock ../schemas/<spec-file> -p 4010`
5. If the provider uses a custom media type (e.g., `application/vnd.github+json`), ensure it's included as a content type in the spec's response definitions — Prism strictly validates Accept headers

## Step 3 — Check existing contract tests for overlap

```
test/contract/*.test.js
```

Don't duplicate coverage. If an existing contract test already exercises the same endpoint and response shape, extend it rather than creating a new file.

## Step 4 — Write the test

### Location and framework

- File: `test/contract/<provider>-<feature>.test.js` (e.g., `github-artifacts.test.js`)
- Framework: `vitest` + `riteway/vitest`
- Imports:
  ```js
  import { spawn } from "node:child_process"
  import { assert } from "riteway/vitest"
  import { afterAll, beforeAll, describe, test } from "vitest"
  ```

### Test template

```js
import { spawn } from "node:child_process"
import { assert } from "riteway/vitest"
import { afterAll, beforeAll, describe, test } from "vitest"

const PRISM_PORT = 4010
const BASE_URL = `http://127.0.0.1:${PRISM_PORT}`
const SPEC_PATH = "../../schemas/<provider>-<feature>.json"

let prism

beforeAll(async () => {
  prism = spawn(
    "../node_modules/.bin/prism",
    ["mock", SPEC_PATH, "-p", String(PRISM_PORT)],
    { cwd: import.meta.dirname, stdio: "pipe" },
  )

  // Wait for Prism to be ready
  await new Promise((resolve, reject) => {
    const timeout = setTimeout(
      () => reject(new Error("Prism failed to start within 10s")),
      10_000,
    )
    prism.stderr.on("data", (chunk) => {
      if (chunk.toString().includes("Prism is listening")) {
        clearTimeout(timeout)
        resolve()
      }
    })
    prism.on("error", (err) => {
      clearTimeout(timeout)
      reject(err)
    })
  })
})

afterAll(() => {
  if (prism) {
    prism.kill("SIGTERM")
  }
})

describe("<Provider> <Feature> contract", () => {
  test("GET /endpoint returns expected shape", async () => {
    const res = await fetch(`${BASE_URL}/endpoint`, {
      headers: { Accept: "application/json" },
    })

    assert({
      given: "a GET request to /endpoint",
      should: "return status 200",
      actual: res.status,
      expected: 200,
    })

    const data = await res.json()

    assert({
      given: "a successful response",
      should: "include the expected field",
      actual: typeof data.expected_field,
      expected: "string",
    })
  })

  test("GET /endpoint with Prefer: code=404 returns not found shape", async () => {
    const res = await fetch(`${BASE_URL}/endpoint`, {
      headers: {
        Accept: "application/json",
        Prefer: "code=404",
      },
    })

    assert({
      given: "a request triggering a 404 response",
      should: "return status 404",
      actual: res.status,
      expected: 404,
    })

    const data = await res.json()

    assert({
      given: "a 404 response",
      should: "include a message field",
      actual: typeof data.message,
      expected: "string",
    })
  })
})
```

### Key patterns

**Selecting error responses with `Prefer` header:**

```js
const res = await fetch(`${BASE_URL}/endpoint`, {
  headers: {
    Accept: "application/json",
    Prefer: "code=410",
  },
})
```

This tells Prism to return the 410 response defined in the spec. Use this to test error path handling (404, 410, 422, etc.) without needing to trigger real error conditions.

**Asserting shapes and types (not exact values):**

```js
// Good — asserts shape
assert({
  given: "a list response",
  should: "return an array",
  actual: Array.isArray(data.items),
  expected: true,
})

assert({
  given: "each item in the list",
  should: "have a numeric id",
  actual: typeof data.items[0].id,
  expected: "number",
})

// Bad — hardcodes exact value from spec examples
assert({
  given: "a list response",
  should: "return the example item",
  actual: data.items[0].name,
  expected: "my-artifact",  // Brittle — breaks when spec examples change
})
```

**GitHub-specific Accept header:**

When testing against a GitHub vendored spec, check whether the spec defines responses under `application/json` or `application/vnd.github+json`. Prism strictly validates the Accept header against what the spec declares. If the spec only declares `application/vnd.github+json`, use that:

```js
const res = await fetch(`${BASE_URL}/repos/owner/repo/actions/artifacts`, {
  headers: { Accept: "application/vnd.github+json" },
})
```

## Step 5 — Run and verify

```bash
make contract-test
```

Contract tests run in their own vitest suite, separate from E2E tests. They require no credentials or network access — only that Prism can start and load the vendored spec.

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
| Hitting real APIs in contract tests | That's what E2E tests are for | Use Prism on localhost only |
| Hardcoding response body values | Breaks when spec examples change | Assert field existence and types instead |
| No vendored spec | Contract test has nothing to test against | Create the spec in `schemas/` first (Step 2) |
| Testing against the full provider spec | Slow startup, noisy, hard to maintain | Vendor only the paths your code uses |
| Importing production code | Creates coupling between test and implementation | Use `fetch` directly against Prism |
| Skipping the Accept header | Prism may reject with 406 Not Acceptable | Always set Accept to match what the spec declares |
| Using a random port without checking availability | Port conflicts in CI | Use a fixed port (4010) or implement port-finding logic |
| Forgetting to kill Prism in afterAll | Orphaned process blocks the port | Always `prism.kill("SIGTERM")` in afterAll |
