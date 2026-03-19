# CLI (Go)

## Commands

```bash
make cli-build   # Or: go build -o bin/quarantine ./cmd/quarantine
make cli-test    # Or: go test ./...
make cli-lint    # Or: golangci-lint run
```

## Packages

| Package | Purpose |
|---------|---------|
| `cmd/quarantine` | Entry point, cobra command setup |
| `internal/config` | `quarantine.yml` parsing and validation |
| `internal/git` | Git remote URL parsing |
| `internal/github` | GitHub API client (Contents, Issues, Search, Comments) |
| `internal/parser` | JUnit XML parsing, test_id construction |
| `internal/quarantine` | `quarantine.json` read/write/merge, quarantine state logic |
| `internal/runner` | Test command execution, rerun command construction |

## Conventions

- Test assertions: `riteway.Assert(t, riteway.Case[T]{...})` from `github.com/mycargus/riteway-golang`.
- Golden test fixtures live in `testdata/` at the repo root.
- Integration tests use build tag `integration`. E2E tests use build tag `e2e`.
- Error handling: never break the build. See `docs/specs/error-handling.md`.
- Exit codes: 0 = success, 1 = test failures, 2 = quarantine error.
