# Quarantine

Quarantine automatically detects, quarantines, and tracks flaky tests in CI pipelines. A Go CLI wraps test commands, detects flakiness via retry, and manages quarantine state on GitHub. A React Router v7 dashboard provides analytics by pulling data from GitHub Artifacts.

## Architecture

See `docs/planning/architecture.md` for the full system design. Key points:

- **Model C (ADR-011):** GitHub-native CLI + standalone dashboard. The CI-critical path depends only on GitHub. The dashboard is non-critical.
- **CLI (Go):** Wraps test commands, parses JUnit XML, retries failures, suppresses quarantined tests, updates state on a dedicated GitHub branch (`quarantine/state`), creates GitHub Issues, posts PR comments, uploads results as GitHub Artifacts.
- **Dashboard (React Router v7 + SQLite):** Pulls results from GitHub Artifacts via hybrid polling. Provides trends, cross-repo visibility, and flaky test analytics.
- **No SaaS dependency in the CI path.** The CLI never talks to the dashboard. The dashboard discovers data autonomously.

## v1 Scope

Strict boundaries -- do not expand without discussion:

- **Frameworks:** RSpec, Jest, Vitest only (ADR-016). Python/Go/Java are v2+.
- **CI:** GitHub Actions only for full features (artifacts, cache). CLI binary runs anywhere.
- **Tickets:** GitHub Issues only. Jira is v2+.
- **Notifications:** PR comments only. Slack/email are v2+.
- **Auth:** PAT via `QUARANTINE_GITHUB_TOKEN` env var (falls back to `GITHUB_TOKEN`). GitHub App + OAuth is v2+.
- **No auto-healing (ADR-017).** The tool does not fix or re-test quarantined tests. Unquarantine happens only when a human closes the GitHub Issue.

## Tech Stack

- **CLI:** Go. Single binary, no runtime deps. `cobra` for CLI framework. `encoding/xml` for JUnit XML.
- **Dashboard:** React Router v7 (framework mode), TypeScript, SQLite (WAL mode), Tailwind CSS.
- **Config:** `quarantine.yml` in repo root (ADR-010).

## Key Design Principles

1. **Never break the build due to Quarantine's own failure.** All infrastructure errors are warnings, never fatal. Degraded mode uses cached `quarantine.json` from GitHub Actions cache.
2. **Zero friction adoption.** `quarantine run -- <test command>` is the entire integration. One env var for auth.
3. **GitHub IS the backend.** State on a branch, results in artifacts, tickets as issues. No external database in the CI path.
4. **Quarantine wins on conflict.** Concurrent builds use optimistic concurrency (SHA-based CAS). When quarantine and unquarantine race, quarantine wins (ADR-012).

## Documentation

Docs are organized by purpose:

- `docs/specs/` -- Implementation references (cli-spec, config-schema, github-api-inventory, error-handling, sequence-diagrams, test-strategy)
- `docs/planning/` -- Architecture, milestones, functional and non-functional requirements
- `docs/research/` -- Decision inputs (junit-xml-research, ci-artifact-api-research, competitive-landscape)
- `docs/scenarios/` -- 66 given-when-then scenarios (v1) + 9 v2+ scenarios, organized by topic
- `docs/milestones/` -- Milestone manifests: the entry point for agents implementing a milestone
- `docs/adr/` -- 21 Architecture Decision Records

## Agents and Skills

Custom agents and skills are in `.claude/agents/` and `.claude/commands/`.

- **`cli-dev` agent:** Scoped to CLI (Go) work. Use for M1-M5.
- **`dashboard-dev` agent:** Scoped to dashboard (TypeScript) work. Use for M6-M7.
- **`/milestone N`:** Sets current milestone context in CLAUDE.md.
- **`/review-adr "description"`:** Checks a proposed change against all ADRs for conflicts.
- **`/sync-docs [scope]`:** Audits code vs documentation for inconsistencies.

Agent path scoping is trust-based (system prompt instructions), not structurally enforced. If structural enforcement is needed, use `PreToolUse` hooks to validate file paths.

## Implementation Notes

- **Milestones** are in `docs/planning/milestones.md` (M1-M8). Milestone manifests in `docs/milestones/` are the entry point for agents.
- **Rate limits:** `GITHUB_TOKEN` = 1,000 req/hr. PAT = 5,000/hr. GitHub App = 5,000-12,500/hr. Design for the lowest.
- **Concurrency:** quarantine.json uses SHA-based compare-and-swap via GitHub Contents API. Retry on 409, max 3. Issue creation uses check-before-create with deterministic labels.
- **JUnit XML caveats:** No official schema exists. Jest needs `jest-junit`, RSpec needs `rspec_junit_formatter`, Vitest has built-in support. Rerun commands require framework-specific classname-to-invocation mapping.

## What NOT to Do

- Do not add frameworks beyond v1 scope without discussion
- Do not add SaaS infrastructure dependencies to the CLI critical path
- Do not add features beyond the current milestone
- Do not attempt auto-healing or auto-unquarantine of flaky tests
- Do not assume Docker is required -- it is a deployment convenience
- Do not store secrets in `quarantine.yml` -- tokens come from env vars only

## Development Workflow

- **Use `/mikey:tdd` when writing code.** All new code is written via TDD with Given/When/Then specs. Pass the relevant scenarios file as the spec path (e.g., `/mikey:tdd docs/scenarios/v1/01-initialization.md`).
- **Use `/mikey:testify` when validating code.** Review and align tests with test philosophy after writing.
- **Keep scope small per change.** Avoid drift from intention. One concern per change.
- **NEVER modify existing scenarios in `docs/scenarios/` without user confirmation.** Adding new scenarios is OK.

## Instructions

- NEVER make assumptions
- NEVER use APIs, fields, config options, or features without first verifying they exist in official documentation
- ALWAYS admit when you're not sure
- ALWAYS consult official resources (code, documentation, etc) before making a recommendation or decision
- ALWAYS ask clarification questions as needed