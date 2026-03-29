# Contract Tests

Prism-based contract tests that verify production code sends correctly-shaped requests and handles response shapes from vendored OpenAPI specs — without network access or credentials.

See **ADR-024** (`docs/adr/024-contract-testing-tool.md`) for the full decision rationale.

## What contract tests verify

- Request shape (method, path, headers, body)
- Response shape (fields, types, nesting)
- Error response shapes (404, 410, 422, etc.)

They do **not** verify real API behavior, latency, rate limits, or stateful round-trips. Use E2E tests (`../e2e/`) for those.

## Running

```bash
make contract-test
```

No credentials or network access required. Prism starts as a local mock server from the vendored spec in `schemas/`.

## Adding new contract tests

Use the `/create-contract-test` skill. It guides you through identifying contract risks, ensuring a vendored spec exists, and writing the test following project conventions.

Vendored OpenAPI specs live in `schemas/` at the repository root.
