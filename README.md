# Quarantine

Copyright (C) 2026 Michael Hargiss. Licensed under the [GNU Affero General Public License v3.0](LICENSE).

Quarantine automatically detects, quarantines, and tracks flaky (non-deterministic) tests in CI pipelines.

> "A test is non-deterministic when it passes sometimes and fails sometimes,
> without any noticeable change in the code, tests, or environment. Such tests
> fail, then you re-run them and they pass. Test failures for such tests are
> seemingly random." — Martin Fowler, [Eradicating Non-Determinism in Tests]

[Eradicating Non-Determinism in Tests]: https://martinfowler.com/articles/nonDeterminism.html

Quarantining flaky tests manually is tedious and error-prone, especially as a test suite grows. Quarantine automates it.

## How It Works

```text
/--------------------------------------------\
| A test fails and passes on the same build. |  <---\
|                    ಠ_ಠ                     |      |
\--------------------------------------------/      |
                      ||                            |
                      \/                            |
/--------------------------------------------\      |
| The build still passes. The test is        |      |
| quarantined. The team is notified. ヾ(＾∇＾)|      |
\--------------------------------------------/      |
                      ||                            |
                      \/                            |
/--------------------------------------------\      |
| A GitHub Issue is created for the          |      |
| flaky test.                                |      |
\--------------------------------------------/      |
                      ||                            |
                      \/                            |
/--------------------------------------------\      |
| When the issue is closed, the test is      |      |
| released from quarantine. It will          |  ----/
| run in builds again.  \o/                  |
\--------------------------------------------/
```

## Quick Start

1. Run `quarantine init` in your repo root:

```sh
quarantine init
```

This interactively creates `quarantine.yml` and the `quarantine/state` branch on GitHub. The minimal config it produces looks like:

```yaml
version: 1
framework: jest  # or rspec or vitest
```

`github.owner` and `github.repo` are auto-detected from your git remote.

2. Set `QUARANTINE_GITHUB_TOKEN` (or `GITHUB_TOKEN`) in your CI environment.

3. Wrap your test command in CI:

```yaml
- name: Run tests
  run: quarantine run -- jest --ci --reporters=default --reporters=jest-junit
  env:
    QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

- name: Upload quarantine results
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: quarantine-results-${{ github.run_id }}
    path: .quarantine/results.json
```

That's it. Quarantine handles detection, quarantine state, GitHub Issues, and PR comments automatically.

## Features (v1)

- **Zero-friction integration:** one command wraps your existing test runner
- **Flaky detection:** re-runs failing tests N times (default 3); a test that fails then passes is flagged as flaky
- **Build protection:** build exits 0 if only newly-quarantined tests failed; quarantined tests are excluded from future builds entirely (*supported test frameworks only)
- **GitHub-native state:** quarantine state stored on a dedicated `quarantine/state` branch — no external database
- **GitHub Issues:** one issue per flaky test; closing the issue unquarantines the test
- **PR comments:** summary of flaky test results posted on each PR
- **Dashboard:** Web UI with trends and cross-repo analytics (pulls from GitHub Artifacts; read-only in v1)
- **Supported frameworks:** RSpec, Jest, Vitest
- **Frameworks with automatic exclusion of flaky tests from new builds:** Jest, Vitest

## Commands

| Command | Description |
|---------|-------------|
| `quarantine init` | Initialize quarantine for a repo (creates `quarantine.yml` and the state branch) |
| `quarantine run -- <cmd>` | Wrap your test command with flaky detection and quarantine enforcement |
| `quarantine doctor` | Validate `quarantine.yml` and print the resolved configuration |
| `quarantine version` | Print the CLI version |

## Architecture

Quarantine follows a GitHub-native architecture. The CLI handles the CI-critical path with no dependencies beyond GitHub. The dashboard is non-critical and discovers data autonomously by polling GitHub Artifacts.

See [`docs/planning/architecture.md`](docs/planning/architecture.md) for the full system design.

## Credit

Inspired by the [quarantine gem] by Flexport.

[quarantine gem]: https://github.com/flexport/quarantine
