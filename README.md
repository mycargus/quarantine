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

1. Add `quarantine.yml` to your repo root:

```yaml
github:
  owner: my-org
  repo: my-repo
retries: 3
```

2. Set `QUARANTINE_GITHUB_TOKEN` (or `GITHUB_TOKEN`) in your CI environment.

3. Wrap your test command:

```sh
quarantine run -- <your test command>
```

That's it. Quarantine handles detection, quarantine state, GitHub Issues, and PR comments automatically.

## Features (v1)

- **Zero-friction integration:** one command wraps your existing test runner
- **Flaky detection:** re-runs failing tests N times (default 3); a test that fails then passes is flagged as flaky
- **Build protection:** quarantined failures become skips in JUnit XML output; build exits 0 if only quarantined tests failed
- **GitHub-native state:** quarantine state stored on a dedicated `quarantine/state` branch — no external database
- **GitHub Issues:** one issue per flaky test; closing the issue unquarantines the test
- **PR comments:** summary of flaky test results posted on each PR
- **Dashboard:** React Router v7 web UI with trends, cross-repo analytics, and quarantine management (pulls from GitHub Artifacts)
- **Supported frameworks:** RSpec, Jest, Vitest

## Architecture

Quarantine follows a GitHub-native architecture. The CLI handles the CI-critical path with no dependencies beyond GitHub. The dashboard is non-critical and discovers data autonomously by polling GitHub Artifacts.

See [`docs/architecture.md`](docs/architecture.md) for the full system design.

## Credit

Inspired by the [quarantine gem] by Flexport.

[quarantine gem]: https://github.com/flexport/quarantine
