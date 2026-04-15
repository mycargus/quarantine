# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-04-15

Initial release. Flaky test detection and quarantine for CI pipelines.

### CLI

- `quarantine init` — auto-detects Jest, Vitest, and RSpec; creates `.quarantine/config.yml` with pre-filled suite entries; creates `quarantine/state` branch; idempotent on re-run
- `quarantine run [suite-name]` — wraps test commands, parses JUnit XML, retries failures, manages quarantine state, creates GitHub Issues, posts PR comments, writes result artifacts
- `quarantine status [suite-name]` — shows quarantined test count, oldest tests, duration estimates from artifacts
- `quarantine suite list` — prints configured suites with name, command, and junitxml path
- `quarantine suite remove <name>` — removes a suite from config with confirmation; preserves state branch history
- `quarantine doctor` — validates `.quarantine/config.yml` and prints resolved configuration
- `quarantine version` — prints CLI version
- Multi-suite support: each suite has independent state, issues, and PR comments
- Flaky detection via configurable retry (1-10 attempts, default 3)
- Quarantine state on `quarantine/state` branch with SHA-based compare-and-swap (conflict resolution: quarantine wins)
- GitHub Issues with deterministic dedup labels (`quarantine:<suite>:<hash>`)
- Per-suite PR comments with `<!-- quarantine:<suite-name> -->` markers
- Per-suite result artifacts (`.quarantine/<suite>/results.json`)
- Pre-execution `quarantined-files.txt` for downstream consumers
- Configurable per-suite timeout and rerun timeout with SIGTERM/SIGKILL graceful shutdown
- Command crash detection (non-zero exit + no JUnit XML) with diagnostic messages
- Unresolved test classification when reruns fail
- Degraded mode: falls back to cached state on GitHub API errors (401, 403, 5xx, 429, timeout) without breaking the build
- `--dry-run`, `--quiet`, `--pr <number>`, `--timeout <duration>` flags
- `--yes`/`-y`, `--retries`, `--junitxml` flags on `quarantine init` for non-interactive use
- TTY detection: warns when stdin is not a terminal and `--yes` is absent
- Structured logging with `[quarantine]` prefix; `QUARANTINE_DEBUG` env var for debug output
- Parameterized test support: Jest `test.each`, RSpec shared examples, Vitest `test.each`
- JUnit XML parsing for Jest (`jest-junit`), RSpec (`rspec_junit_formatter`), and Vitest (built-in)
- Cross-compiled binaries for linux/darwin (amd64/arm64)
- Install script (`scripts/install.sh`) with SHA-256 checksum verification

### Dashboard

- Remix 3 web UI with SQLite backend
- Org-wide overview: total quarantined tests across all configured repos
- Project detail pages with quarantined test list, issue links, and trend chart
- Date range filter and search by test name
- On-demand sync from GitHub Artifacts on page load (debounced, 5-minute cooldown)
- Artifact ingestion: parses result JSON, upserts test runs and quarantined tests
- Flaky count tracking and last-flaky-at timestamps
- Circuit breaker: pauses polling after 3 consecutive failures per repo (30-minute cooldown)
- ETag-based conditional requests to avoid re-downloading unchanged artifacts
- Graceful degradation: sync failures render page with existing data
- Manual repo configuration via `dashboard.yml`

### Infrastructure

- Two-phase release process: rc prerelease with E2E validation, then final release promotion
- Contract tests against vendored GitHub OpenAPI spec (Prism-based, offline)
- E2E tests that observe real fixture repo CI output (never run the binary or arrange state)
- Interface tests for CLI binary and dashboard HTTP routes

[Unreleased]: https://github.com/mycargus/quarantine/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mycargus/quarantine/releases/tag/v0.1.0
