# Contract Testing Spike: Specmatic

> Date: 2026-03-28

## Goal

Evaluate Specmatic as a contract testing tool for this project. The requirements:
- Serve a fake HTTP service from vendored OpenAPI specs
- Validate requests and responses against the spec
- Support multiple providers (GitHub, Jenkins, GitLab, Jira)
- Test ETag/304 conditional request round-trips
- Test artifact download (302 redirect → zip binary)

## Setup

- **Specmatic version:** v2.28.0 (Docker image `znsio/specmatic:latest`)
- **Spec used:** `schemas/github-api-artifacts.json` — a 12KB subset of GitHub's published OpenAPI spec (`github/rest-api-description`) containing 2 endpoints (list artifacts, download artifact)
- **Examples:** External JSON files in `schemas/github-api-artifacts_examples/`

## What worked

### Spec loading and stub generation

Specmatic loads the vendored OpenAPI subset and generates a stub server with zero configuration:

```bash
docker run -d -p 9000:9000 -v ./schemas:/specs \
  znsio/specmatic stub /specs/github-api-artifacts.json --port 9000
```

Output: `API Paths: 2, API Operations: 2, Schema components: 2`

The full 12MB GitHub spec (1107 operations) caused a `java.lang.Integer cannot be cast to java.lang.String` error and returned `400 No valid API specifications loaded`. The subset approach is necessary.

### Example-driven responses

External example files in `{spec-name}_examples/` directory are auto-discovered. Example format:

```json
{
  "http-request": {
    "method": "GET",
    "path": "/repos/(owner:string)/(repo:string)/actions/artifacts",
    "query": { "per_page": "(number)" }
  },
  "http-response": {
    "status": 200,
    "headers": { "ETag": "\"abc123\"" },
    "body": { "total_count": 1, "artifacts": [...] }
  }
}
```

Path params use `(name:type)` syntax. Query params with `(type)` act as wildcards.

### Response schema validation

Specmatic validates example responses against the OpenAPI schema on load. When the example included an `ETag` header not defined in the spec, Specmatic rejected it with:

```
Header named ETag in the stub was not in the contract
```

This is the core value: the spec is the contract, and Specmatic enforces it.

### Request validation

Unknown paths return `400 No matching REST stub or contract found`. Requests matching a known path but not matching any example fall through to random schema-conformant generation (marked `X-Specmatic-Type: random`).

### Integration with dashboard code

The dashboard's `listArtifacts` function worked against Specmatic by replacing the base URL:

```typescript
const fakeFetch = (url, init) => {
  const fakeUrl = url.replace('https://api.github.com', 'http://localhost:9000')
  return fetch(fakeUrl, init)
}
const result = await listArtifacts('owner', 'repo', 'token', null, fakeFetch)
// Returns example data with correct ETag, artifact name, etc.
```

## What did NOT work

### 304 Not Modified responses (BLOCKER)

Created an example with `"status": 304` and `"If-None-Match": "(string)"` in the request. Specmatic rejected it:

```
get_artifacts_304.json didn't match any of the contracts from github-api-artifacts.json
```

**Root cause:** The OpenAPI spec only defines a `200` response for the list endpoint. 304 is implicit HTTP caching behavior, not an endpoint-specific response. Specmatic strictly enforces that stubs match defined responses.

**Impact:** Cannot test ETag/If-None-Match → 304 round-trips, which is the #1 mock-fidelity risk for the dashboard's artifact polling.

### Binary content / redirect-then-download (BLOCKER)

The download endpoint returns a 302 redirect. Specmatic generates:

```
HTTP/1.1 302 Found
Location: LBAHF    <-- random string, not a URL
```

There is no mechanism to:
1. Serve a realistic Location URL that points to a second endpoint
2. Serve binary zip content at that second endpoint
3. Chain the redirect → download flow

**Impact:** Cannot test `downloadAndExtractJson` (302 redirect → zip extraction).

### Auth header validation (GAP)

Specmatic accepted requests with no auth header, wrong auth scheme (`Basic` instead of `Bearer`), and fake tokens. All returned 200.

**Root cause:** GitHub's published OpenAPI spec does not define any security schemes (`securitySchemes: {}`, `security: not defined`). This is true of the full 12MB spec, not just our subset. Without security schemes in the spec, Specmatic has nothing to validate against.

**Impact:** Cannot catch auth header format bugs (Bearer vs Basic vs token header).

### Query param matching requires explicit examples

`listArtifacts` sends `?per_page=100`. Without `"query": { "per_page": "(number)" }` in the example, Specmatic fell back to random generation instead of returning example data.

**Impact:** Each distinct query param combination needs its own example or a wildcard. This is manageable but creates example file proliferation for endpoints with many optional params.

## Spec accuracy

Compared the extracted artifact schema against the full GitHub spec:
- **Required fields:** match exactly (10 fields)
- **Properties:** match exactly (12 fields including `digest` and `workflow_run`)
- **Nullable markers:** no mismatches
- **Missing from GitHub's spec:** ETag response header (returned by real API but not documented)

## Assessment

| Capability | Status |
|---|---|
| Load OpenAPI spec and serve stubs | Works |
| External example files | Works |
| Response schema validation on load | Works |
| Request path validation | Works |
| Integration with project code | Works |
| ETag/304 conditional requests | **Does not work** |
| 302 redirect → binary download | **Does not work** |
| Auth header validation | Does not work (spec limitation) |
| Full GitHub spec (12MB) | Does not work (parse error) |

**Verdict:** Specmatic is valuable for schema conformance testing (are my requests and responses the right shape?) but cannot simulate the stateful HTTP behaviors that represent the highest mock-fidelity risks in this project. It would need to be supplemented by either real API tests or a custom stateful layer.
