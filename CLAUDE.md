# Quarantine

Flaky test detection and quarantine for CI pipelines. Go CLI + React dashboard.

## Commands

```bash
make dev               # One-time setup: install hooks + all dependencies
make check             # Lint + typecheck (runs on pre-commit)
make test-all          # Run all tests (CLI + dashboard + contract + e2e)
make lint-all          # Run all linters (CLI + dashboard + test)
make cli-build         # Build CLI binary to bin/quarantine
make cli-test          # Go tests
make cli-lint          # golangci-lint
make dash-build        # Dashboard production build
make dash-test         # Dashboard tests
make dash-lint         # Biome lint
make dash-typecheck    # TypeScript type checking
make contract-test     # Contract tests (Prism, offline, no credentials)
make e2e-test          # E2E tests (requires test/e2e/.env with GitHub credentials)
make test-lint         # Lint all test code
```

The pre-commit hook runs `make check` automatically.

## Architecture

See `docs/planning/architecture.md` for full design. See `cli/CLAUDE.md` and `dashboard/CLAUDE.md` for component-specific context.

- **Model C (ADR-011):** GitHub-native CLI + standalone dashboard. CI path depends only on GitHub.
- **CLI (Go):** Wraps test commands, parses JUnit XML, retries failures, manages quarantine state on `quarantine/state` branch, creates Issues, posts PR comments, uploads Artifacts.
- **Dashboard (React Router v7 + SQLite):** Pulls from GitHub Artifacts. Read-only analytics.
- **No SaaS in the CI path.** CLI never talks to the dashboard.

## Key Design Principles

1. **Never break the build.** Quarantine errors are warnings, never fatal. Degraded mode uses cached `quarantine.json`.
2. **Zero friction.** `quarantine run -- <test command>` is the entire integration. One env var for auth.
3. **GitHub IS the backend.** State on a branch, results in artifacts, tickets as issues.
4. **Quarantine wins on conflict.** SHA-based CAS with retry on 409. When quarantine and unquarantine race, quarantine wins (ADR-012).

## Implementation Notes

- **Milestones** are in `docs/planning/milestones.md`. Manifests in `docs/milestones/` are the entry point for implementation.
- **Rate limits:** Design for `GITHUB_TOKEN` (1,000 req/hr), not PAT (5,000/hr).
- **Concurrency:** `quarantine.json` uses compare-and-swap via GitHub Contents API. Issue creation uses check-before-create with deterministic labels.
- **JUnit XML:** No official schema. Jest needs `jest-junit`, RSpec needs `rspec_junit_formatter`, Vitest has built-in support.
- **Test fixtures:** `testdata/` at repo root (shared across packages).

## Skills

Project skills (invoke with `/skill-name`):

- `/implement-milestone` -- Implement a milestone using TDD and atomic commits
- `/verify-milestone` -- Verify implementation against a milestone manifest
- `/create-manifest` -- Generate a milestone manifest routing document
- `/create-contract-test` -- Create a Prism-based contract test (offline, no credentials)
- `/create-e2e-test` -- Create an E2E test verifying real API behavior matches mocks
- `/review-adr` -- Check if a change contradicts an existing ADR
- `/sync-docs` -- Scan for inconsistencies between code and documentation

## Documentation

- `docs/specs/` -- Implementation references (cli-spec, config-schema, error-handling, test-strategy, etc.)
- `docs/planning/` -- Architecture, milestones, requirements
- `docs/research/` -- Decision inputs (junit-xml, ci-artifacts, competitive landscape)
- `docs/scenarios/` -- Given/when/then user scenarios
- `docs/milestones/` -- Milestone manifests (agent entry points)
- `docs/prompts/` -- Reusable prompts (scenario-authoring, adr-proposal)
- `docs/adr/` -- Architecture Decision Records

## Boundaries

Do not expand without discussion:

- **v1 frameworks:** RSpec, Jest, Vitest only (ADR-016)
- **v1 CI:** GitHub Actions only for full features
- **v1 tickets:** GitHub Issues only (no Jira)
- **v1 auth:** PAT via `QUARANTINE_GITHUB_TOKEN` (falls back to `GITHUB_TOKEN`)
- **No auto-healing (ADR-017).** Unquarantine only when a human closes the Issue.
- **No features beyond the current milestone.**
- **No secrets in `quarantine.yml`** -- tokens come from env vars only.

## Rules

- Do not make assumptions -- verify APIs, fields, and features exist in official docs first.
- Admit when you're not sure. Ask clarification questions.
- Consult `docs/` before making design decisions.
- Do not use milestone identifiers (M1, M2, etc.) in file names, code comments, or variable names. The only acceptable place is the `milestone N:` prefix in git commit subjects.
