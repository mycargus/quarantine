# Contract Testing Spike: Stoplight Prism

> Date: 2026-03-28

## Goal

Evaluate Stoplight Prism as a contract testing tool for this project, specifically testing the two behaviors that Specmatic failed on:
1. ETag/304 conditional request round-trips
2. 302 redirect with a usable Location header for binary downloads

## Setup

- **Prism version:** v5.14.2 (`@stoplight/prism-cli` via pnpm)
- **Spec used:** `schemas/github-api-artifacts.json` -- a 12KB OpenAPI 3.0.3 subset with 2 endpoints (list artifacts, download artifact)
- **Install:** `pnpm add -D @stoplight/prism-cli` in the `test/` directory
- **Start command:** `./node_modules/.bin/prism mock ../schemas/github-api-artifacts.json -p 4010`

## Test Results

### Test A: ETag header from spec (WORKS)

```bash
curl -s -D - http://localhost:4010/repos/owner/repo/actions/artifacts
```

Response headers:
```
HTTP/1.1 200 OK
ETag: string
Link: <https://api.github.com/resource?page=2>; rel="next", <https://api.github.com/resource?page=5>; rel="last"
Content-type: application/json
```

Prism returns the ETag header defined in the spec. In static mode (default), the value is the literal string `"string"` (the schema type). In dynamic mode (`--dynamic`), the value is random Lorem-ipsum text (e.g., `"commodo irure"`). Neither produces a realistic ETag value like `"abc123def"`, but the header is present.

The response body in static mode returns the spec's example data exactly:
```json
{"total_count":2,"artifacts":[{"id":11,"node_id":"MDg6QXJ0aWZhY3QxMQ==","name":"Rails",...}]}
```

### Test B: 304 Not Modified (DOES NOT WORK)

```bash
curl -s -D - -H 'If-None-Match: "some-etag"' http://localhost:4010/repos/owner/repo/actions/artifacts
```

Response: **200 OK** with full body. Prism ignores the `If-None-Match` header entirely.

Also tried the `Prefer` header to force a status code:
```bash
curl -s -D - -H 'Prefer: code=304' http://localhost:4010/repos/owner/repo/actions/artifacts
```

Response:
```
HTTP/1.1 404 Not Found
{"type":"https://stoplight.io/prism/errors#NOT_FOUND",
 "title":"The server cannot find the requested content",
 "status":404,
 "detail":"Requested status code 304 is not defined in the document."}
```

**Root cause:** Same as Specmatic -- the spec only defines a 200 response. Prism cannot return a status code that isn't defined in the spec. The `Prefer: code=304` header confirms this: `"Requested status code 304 is not defined in the document."` Adding a 304 response definition to the spec could potentially work, but 304 is implicit HTTP behavior, not a per-endpoint response.

**Note:** The `Prefer: code=410` header DOES work for the download endpoint because 410 is defined in the spec:
```bash
curl -s -D - -H 'Prefer: code=410' http://localhost:4010/repos/owner/repo/actions/artifacts/123/zip
# Returns: HTTP/1.1 410 Gone with basic-error body
```

### Test C: 302 redirect with Location header (WORKS)

```bash
curl -s -D - http://localhost:4010/repos/owner/repo/actions/artifacts/123/zip
```

Response:
```
HTTP/1.1 302 Found
Location: https://pipelines.actions.githubusercontent.com/OhgS4QRKqmgx7bKC27GKU83jnQjyeqG8oIMTge8eqtheppcmw8/_apis/pipelines/1/runs/176/signedlogcontent?urlExpires=2020-01-24T18%3A10%3A31.5729946Z&urlSigningMethod=HMACV1&urlSignature=agG73JakPYkHrh06seAkvmH7rBR4Ji4c2%2B6a2ejYh3E%3D
Content-Length: 0
```

This is a major improvement over Specmatic. Prism uses the `example` value from the Location header definition in the spec, producing a realistic GitHub Actions URL. Specmatic returned a random 5-character string (`LBAHF`).

**Limitation:** The redirect URL points to a real GitHub Actions URL that doesn't resolve. Prism cannot serve content at the redirect target. A full redirect-then-download flow still requires either: (a) pointing the Location at a second Prism endpoint, or (b) a custom test server.

### Test D: Auth header validation (DOES NOT WORK)

```bash
# No auth
curl -s -D - http://localhost:4010/repos/owner/repo/actions/artifacts
# Returns: 200 OK

# Wrong auth scheme
curl -s -D - -H "Authorization: Basic bad" http://localhost:4010/repos/owner/repo/actions/artifacts
# Returns: 200 OK

# Bearer token
curl -s -D - -H "Authorization: Bearer fake-token" http://localhost:4010/repos/owner/repo/actions/artifacts
# Returns: 200 OK
```

All return 200. Same root cause as Specmatic: GitHub's published spec has no `securitySchemes` defined, so there's nothing for Prism to validate against.

### Test E: Dashboard `listArtifacts` integration (WORKS with caveat)

Direct integration fails with **406 Not Acceptable** because the dashboard sends `Accept: application/vnd.github+json` but the spec only defines `application/json`:

```
{"type":"https://stoplight.io/prism/errors#NOT_ACCEPTABLE",
 "title":"The server cannot produce a representation for your accept header",
 "status":406,
 "detail":"Unable to find content for application/vnd.github+json"}
```

**Workaround:** Override the Accept header in the test's fakeFetch:

```typescript
const fakeFetch = (url, init) => {
  const fakeUrl = url.replace('https://api.github.com', 'http://localhost:4010')
  const headers = { ...init?.headers, Accept: 'application/json' }
  return fetch(fakeUrl, { ...init, headers })
}
const result = await listArtifacts('owner', 'repo', 'fake-token', null, fakeFetch)
```

Result:
```json
{
  "artifacts": [
    {"id": 11, "name": "Rails", "size_in_bytes": 556, ...},
    {"id": 13, "name": "Test output", "size_in_bytes": 453, ...}
  ],
  "etag": "string",
  "notModified": false
}
```

**Better fix:** Add `application/vnd.github+json` as an additional content type in the vendored spec. This is a one-line spec edit and would remove the need for the Accept header workaround.

### Test F: Full 12MB GitHub spec (WORKS)

```bash
./node_modules/.bin/prism mock /path/to/github-api-full.json -p 4011
```

Prism loaded all 1107 operations in ~3 seconds and served requests correctly. This is a significant advantage over Specmatic, which crashed with `java.lang.Integer cannot be cast to java.lang.String` on the same file.

Startup output listed all 1107 endpoints. Queries against `/repos/owner/repo/actions/artifacts` returned correct example data.

## Prism-specific features

### Static vs Dynamic mode

- **Static (default):** Returns example data from the spec. Deterministic, predictable. Good for integration tests.
- **Dynamic (`--dynamic`):** Generates random data conforming to the schema using json-schema-faker. Non-deterministic unless `--seed` is provided. Data is unrealistic (negative IDs, Lorem ipsum strings, dates like `1897-11-16`).

### Prefer header for status code selection

Prism supports `Prefer: code=NNN` to select which defined response to return. This is useful for testing error paths:
```bash
curl -H 'Prefer: code=410' http://localhost:4010/repos/owner/repo/actions/artifacts/123/zip
# Returns: 410 Gone
```

But it only works for status codes defined in the spec.

### Content-type strictness

Prism strictly validates the Accept header against defined content types. `application/vnd.github+json` is not `application/json` to Prism. This is arguably correct contract behavior but requires spec alignment.

## Comparison with Specmatic

| Capability | Specmatic | Prism |
|---|---|---|
| Load vendored spec subset | Works | Works |
| ETag response header | Works (via external examples) | Works (from spec directly) |
| 304 conditional requests | **Does not work** | **Does not work** |
| 302 redirect Location header | Random string (`LBAHF`) | **Realistic example URL** |
| Binary content at redirect target | Does not work | Does not work |
| Auth header validation | Does not work (spec gap) | Does not work (spec gap) |
| Accept header strictness | Lenient (accepts anything) | Strict (rejects non-matching) |
| Dashboard integration | Works out of box | Works (needs Accept header fix) |
| Full 12MB GitHub spec | **Crashes** (parse error) | **Works** (1107 ops, 3s startup) |
| External example files | Built-in support | Not supported (uses spec examples) |
| Request validation | Schema validation | Schema validation |
| Prefer header for status codes | Not supported | Supported |
| Dynamic/random generation | Random but valid | Random with `--dynamic` flag |
| Deterministic seeds | Not available | `--seed` flag available |
| Setup complexity | Docker + JVM | Node.js (pnpm/npx) |

## Assessment

| Capability | Status |
|---|---|
| Load OpenAPI spec and serve mocks | Works |
| ETag response header | Works |
| 302 redirect with realistic Location | **Works** (improvement over Specmatic) |
| Dashboard integration | Works (with spec fix for Accept header) |
| Full GitHub spec (12MB) | **Works** (improvement over Specmatic) |
| Prefer header for error responses | Works |
| Deterministic seeded responses | Works |
| ETag/304 conditional requests | **Does not work** |
| Binary content at redirect target | Does not work |
| Auth header validation | Does not work (spec limitation) |

**Verdict:** Prism solves one of the two Specmatic blockers (302 redirect with realistic Location URL) and handles the full GitHub spec without crashing. However, it shares the same fundamental limitation for 304 responses: neither tool can return a status code not defined in the spec, and 304 is implicit HTTP caching behavior.

For the 304 case, the options are:
1. Add a `304` response definition to the vendored spec (non-standard but pragmatic)
2. Use a thin wrapper/proxy in front of Prism that handles If-None-Match logic
3. Accept that 304 behavior must be tested with a custom mock or real API

Prism is the better choice between the two tools for this project due to: simpler setup (Node.js vs Docker+JVM), full spec support, realistic example values, and the `Prefer` header for error path testing.
