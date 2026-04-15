# CLI (Go)

## Commands

```bash
make cli-build   # Or: go build -o bin/quarantine ./cli/cmd/quarantine
make cli-test    # Or: go test ./cli/...
make cli-lint    # Or: golangci-lint run ./cli/...

# Run a single test
go test ./cli/... -run TestName

# Build with version injected (matches release builds)
go build -ldflags "-X main.version=v0.1.0" -o bin/quarantine ./cli/cmd/quarantine
```

## Packages

| Package | Purpose |
|---------|---------|
| `cmd/quarantine` | Entry point, cobra subcommands: `init`, `run`, `doctor`, `version` |
| `internal/cas` | Compare-and-swap write logic for `quarantine.json` via GitHub Contents API (SHA-based retry on 409) |
| `internal/config` | `.quarantine/config.yml` parsing, validation, unknown-field tracking (forward-compatible) |
| `internal/git` | Git remote URL parsing |
| `internal/github` | GitHub API client (Contents, Issues, Search, Comments); rate-limit warning callbacks |
| `internal/parser` | JUnit XML parsing, deterministic `test_id` construction (`<file>::<classname>::<name>`) |
| `internal/quarantine` | `quarantine.json` read/write/merge; pure functions only — no I/O |
| `internal/result` | Builds structured JSON output for `.quarantine/results.json` |
| `internal/runner` | Test command execution, signal forwarding, rerun command construction |

## Conventions

- Test assertions: `riteway.Assert(t, riteway.Case[T]{...})` from `github.com/mycargus/riteway-golang`.
- Test fixtures live in `testdata/` at the repo root.
- E2E tests use build tag `e2e`.
- Error handling: never break the build. See `docs/specs/error-handling.md`.
- Exit codes: 0 = success, 1 = test failures (never used for quarantine errors), 2 = quarantine error.
- **Output routing:** `init` writes to stdout. `run` writes diagnostic/status output to stderr so it doesn't contaminate the test runner's stdout.
- **Suite mode:** `quarantine run <suite-name>` executes the named suite's command from `.quarantine/config.yml`. The `--` separator is rejected.
- **Signal forwarding:** `runner.Run()` forwards SIGINT/SIGTERM to the child test process so interrupts work naturally.
