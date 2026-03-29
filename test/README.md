# Test Infrastructure

External test suites live here. These are separate from the CLI's Go unit/integration tests (`cli/`) and the dashboard's TypeScript tests (`dashboard/`).

## Suites

| Suite | Directory | Purpose | Credentials |
|---|---|---|---|
| Contract | `contract/` | Verify request/response shapes against vendored OpenAPI specs (Prism, offline) | None |
| E2E | `e2e/` | Verify real GitHub API behavior against a dedicated test fixture repo | GitHub PAT required |

## Make targets

```bash
make test-build      # Install dependencies (pnpm install)
make contract-test   # Run contract tests (fast, offline, no credentials)
make e2e-test        # Run E2E tests (slow, requires credentials)
make test-lint       # Lint all test code
make test-all        # Run everything (CLI + dashboard + contract + e2e)
```

## When to use each suite

**Contract tests** (`contract/`) — use when production code interacts with an external API and you want fast, offline shape validation. Tests use Prism to mock the API from vendored OpenAPI specs in `schemas/`. See ADR-024 for the full rationale.

**E2E tests** (`e2e/`) — use when behavior depends on real-world API responses: stateful round-trips, redirects, pagination, eventual consistency, or auth edge cases. Requires GitHub credentials. See `e2e/README.md` for setup.

## Shared dependencies

Both suites share a single `package.json` and `node_modules` at this level. Run `pnpm install` from `test/` (or `make test-build`) before running either suite.
