# ADR-024: Contract Testing Tool — Stoplight Prism

**Status:** Accepted
**Date:** 2026-03-28

## Context

Quarantine (Go CLI + React dashboard) interacts with GitHub APIs across multiple surfaces: Contents API for state, Issues API for tickets, Artifacts API for results, PR Comments API.

Integration tests use dependency injection (e.g., `fetchFn`) to mock HTTP calls, but this creates mock-fidelity risk — the real API could diverge from mocks.

Contract tests address this by serving mock responses from vendored OpenAPI specs and validating request/response shapes against the spec.

Two tools were evaluated in hands-on spikes: Specmatic (JVM/Docker) and Stoplight Prism (Node.js). Full spike findings are documented in:

- `docs/research/contract-testing-specmatic-spike.md`
- `docs/research/contract-testing-prism-spike.md`

## Decision

Use Stoplight Prism (`@stoplight/prism-cli`) for contract testing against vendored OpenAPI specs.

Key reasons:

1. **Toolchain fit:** The e2e/ suite is Node.js/pnpm. Prism is a pnpm dev dependency (already installed). No Docker or JVM required.
2. **Full spec support:** Prism loads GitHub's full 12MB OpenAPI spec (1,107 operations, 3-second startup). Specmatic crashes on the same file due to a nullable enum parser bug.
3. **Realistic mock responses:** Prism returns spec example values (realistic redirect URLs, pagination links). Specmatic generates random strings for some fields (e.g., `Location: LBAHF` for a 302 redirect).
4. **Error path testing:** Prism's `Prefer: code=NNN` header selects which defined response to return, enabling clean error path testing without external files.
5. **Deterministic seeds:** `--seed` flag provides reproducible responses across CI runs.

Contract tests live in `test/contract/` (separate from `test/e2e/` which holds credential-requiring E2E tests). Vendored OpenAPI specs live in `schemas/`. Prism runs as a local mock server during test execution.

**Shared limitations (neither tool supports):**

- 304 Not Modified responses (not defined in GitHub's OpenAPI spec — implicit HTTP caching behavior)
- Binary content at redirect targets (302 → zip download requires custom handling)
- Auth header validation (GitHub's spec defines no `securitySchemes`)

These are spec-level gaps, not tool-level gaps. They apply regardless of tool choice and are covered by E2E tests against the real API.

## Alternatives Considered

- **Specmatic (JVM/Docker):** Stronger on-load schema validation of external example files, but crashes on the full GitHub spec, requires Docker+JVM as an additional runtime, and generates unrealistic mock data for redirect Location headers. The schema validation advantage is replicable via Spectral linting in the Prism/Node.js ecosystem. Rejected.
- **Custom mock server:** Maximum flexibility but high maintenance burden. Rejected — Prism provides sufficient fidelity with minimal code.
- **No contract tests (E2E only):** E2E tests against real APIs are slow, require credentials, and are rate-limited. Contract tests provide fast, credential-free validation of request/response shapes. Rejected as insufficient on its own.

## Consequences

- (+) Contract tests run without network access or API credentials — fast and reliable in CI.
- (+) Vendored specs are the single source of truth for API shape expectations.
- (+) Prism integrates into the existing Node.js/pnpm toolchain with zero new runtime dependencies.
- (+) `Prefer` header enables testing error responses (410 Gone, etc.) declaratively in test code.
- (-) Prism cannot simulate stateful HTTP behaviors (ETag/304 round-trips). These remain covered by E2E tests.
- (-) Vendored specs must be kept in sync with the real API. Mitigated by E2E tests that catch real-API divergence.
- (-) Prism strictly validates Accept headers. The vendored spec needs `application/vnd.github+json` added as a content type (one-time fix).
