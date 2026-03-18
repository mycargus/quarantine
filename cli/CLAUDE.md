# CLI (Go)

## Build & Test Commands

- Build: `go build -o bin/quarantine ./cmd/quarantine`
- Test: `go test ./...`
- Lint: `golangci-lint run`
- Test assertions: `github.com/mycargus/riteway-golang` (Given/Should/Actual/Expected pattern)
- Module: `github.com/mycargus/quarantine`

## Package Responsibilities

| Package | Purpose |
|---------|---------|
| `cmd/quarantine` | Entry point, cobra command setup |
| `internal/parser` | JUnit XML parsing, test_id construction |
| `internal/config` | `quarantine.yml` parsing and validation |
| `internal/runner` | Test command execution, rerun command construction |
| `internal/github` | GitHub API client (Contents, Issues, Search, Comments) |
| `internal/quarantine` | `quarantine.json` read/write/merge, quarantine state logic |

## Conventions

- All exported functions have doc comments.
- Tests use riteway-golang for assertions: `riteway.Assert(t, riteway.Case[T]{...})`.
- Golden test fixtures live in `testdata/` at the repo root.
- Integration tests use build tag `integration`. E2E tests use build tag `e2e`.
- Error handling follows `docs/error-handling.md`: never break the build due to quarantine's own failure.
- Exit codes: 0 = success, 1 = test failures, 2 = quarantine error.

## Scope

- Do NOT modify dashboard code (`dashboard/` directory).
- Do NOT add frameworks beyond v1 scope (rspec, jest, vitest only).
- Do NOT add SaaS dependencies to the CLI critical path.
- Do NOT store secrets in configuration files.
