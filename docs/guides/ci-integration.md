# CI Integration

This guide covers integrating Quarantine into your CI pipeline for each
supported test framework, with both PAT and GitHub App authentication.

**Who is this for?** Engineers adding Quarantine to their CI workflows.

**Prerequisites:** `quarantine init` has been run in the repo.

---

## Authentication

Quarantine supports two authentication methods. Both result in a Bearer token
passed to the CLI via environment variable.

| Method | Token source | Rate limit | Setup |
|--------|-------------|------------|-------|
| **PAT** (v1) | `QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN` | 1,000 req/hr (`GITHUB_TOKEN`) or 5,000 req/hr (PAT) | One secret per repo |
| **GitHub App** (v2) | `actions/create-github-app-token` | 5,000-12,500 req/hr | One App per org |

**Token resolution order:**

1. `QUARANTINE_GITHUB_TOKEN` env var (preferred)
2. `GITHUB_TOKEN` env var (fallback)
3. Neither set -- CLI runs in degraded mode (warning, not fatal)

### PAT setup

Create a PAT with `repo` scope (or use the default `GITHUB_TOKEN` in GitHub
Actions):

```yaml
env:
  QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

For higher rate limits or cross-repo access, create a classic PAT with `repo`
scope and store it as a repository secret.

### GitHub App setup

Use `actions/create-github-app-token` to generate a short-lived installation
token:

```yaml
- uses: actions/create-github-app-token@v3
  id: app-token
  with:
    app-id: ${{ vars.QUARANTINE_APP_ID }}
    private-key: ${{ secrets.QUARANTINE_APP_PRIVATE_KEY }}

- name: Run tests
  run: quarantine run -- <your test command>
  env:
    QUARANTINE_GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}
```

The token expires after 1 hour (CI runs are typically minutes). See the
[GitHub App Setup Guide](github-app-setup.md) for App registration.

---

## Framework-Specific Workflows

Each workflow includes: installing quarantine, running tests, and uploading
result artifacts for the dashboard.

### Jest

**Prerequisite:** Install `jest-junit`:

```sh
npm install --save-dev jest-junit
```

**GitHub Actions workflow:**

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install quarantine
        run: curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash
        env:
          VERSION: v0.1.0

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

**JUnit XML path:** Jest with `jest-junit` writes to `junit.xml` by default.
Override with `JEST_JUNIT_OUTPUT_DIR` and `JEST_JUNIT_OUTPUT_NAME` env vars
if needed, and set `junitxml` in `quarantine.yml` to match.

### RSpec

**Prerequisite:** Add `rspec_junit_formatter` to your Gemfile:

```ruby
gem 'rspec_junit_formatter', group: :test
```

**GitHub Actions workflow:**

```yaml
      - name: Run tests
        run: quarantine run -- bundle exec rspec --format RspecJunitFormatter --out rspec.xml
        env:
          QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**quarantine.yml:**

```yaml
framework: rspec
junitxml: rspec.xml
```

**Note:** RSpec supports flaky detection but not automatic exclusion of
quarantined tests from subsequent builds. Quarantined tests still run but
their failures are forgiven.

### Vitest

Vitest has built-in JUnit support -- no extra dependencies needed.

**GitHub Actions workflow:**

```yaml
      - name: Run tests
        run: quarantine run -- vitest run --reporter=junit --outputFile=junit-report.xml
        env:
          QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**quarantine.yml:**

```yaml
framework: vitest
junitxml: junit-report.xml
```

---

## Complete Workflow Example

A full workflow with App token auth, artifact upload, and all recommended
steps:

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/create-github-app-token@v3
        id: app-token
        with:
          app-id: ${{ vars.QUARANTINE_APP_ID }}
          private-key: ${{ secrets.QUARANTINE_APP_PRIVATE_KEY }}

      - name: Install quarantine
        run: curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash

      - name: Run tests
        run: quarantine run -- jest --ci --reporters=default --reporters=jest-junit
        env:
          QUARANTINE_GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}

      - name: Upload quarantine results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: quarantine-results-${{ github.run_id }}
          path: .quarantine/results.json
```

---

## Non-Standard Setups

### Custom package managers (pnpm, bun)

Set `rerun_command` in `quarantine.yml` to override the default rerun
invocation:

```yaml
# pnpm
rerun_command: "pnpm exec jest --testNamePattern '{name}'"

# bun
rerun_command: "bunx jest --testNamePattern '{name}'"

# custom jest config
rerun_command: "npx jest --config jest.ci.config.js --testNamePattern '{name}'"
```

`{name}`, `{classname}`, and `{file}` are substituted with values from the
failing test's JUnit XML entry. See
[`docs/specs/config-schema.md`](../specs/config-schema.md#rerun_command) for
the full reference.

### Multiple JUnit XML files

Use a glob pattern in `quarantine.yml`:

```yaml
junitxml: "results/*.xml"
```

The CLI merges all matching files before processing.

### Custom JUnit XML path

Override per-run with `--junitxml`:

```sh
quarantine run --junitxml path/to/output.xml -- <your test command>
```

---

## Troubleshooting

### "GitHub API returned 401 (unauthorized)"

The token is missing or invalid. The CLI enters degraded mode (tests still
run, quarantine state is not updated).

**Fix:** Verify `QUARANTINE_GITHUB_TOKEN` is set. If using a PAT, check it
has `repo` scope. If using an App token, check the
`actions/create-github-app-token` step succeeded.

### "No JUnit XML found"

The test runner didn't produce XML, or it wrote to a different path.

**Fix:** Run `quarantine doctor` to see the resolved `junitxml` path. Check
that your test runner is configured to output JUnit XML. See the
framework-specific sections above.

### "branch not found: quarantine/state"

The state branch hasn't been created.

**Fix:** Run `quarantine init` to create it.

### Rate limit warnings

```
[quarantine] WARNING: GitHub API rate limit low (42/1000 remaining, resets at 14:30 UTC)
```

You're approaching the rate limit. This is common with `GITHUB_TOKEN` (1,000
req/hr).

**Fix:** Switch to a PAT (5,000 req/hr) or a GitHub App token (5,000-12,500
req/hr). See [Authentication](#authentication) above.

### Degraded mode

```
[quarantine] WARNING: running in degraded mode (GitHub API unreachable, using cached state)
```

The CLI can't reach GitHub but continues running tests using cached quarantine
state from the Actions cache. This is by design -- Quarantine never breaks
your build due to its own infrastructure issues.

**Fix:** Check GitHub status (githubstatus.com). The CLI will resume normal
operation on the next run when GitHub is reachable.

---

*References: [config-schema.md](../specs/config-schema.md) (full config reference),
[cli-spec.md](../specs/cli-spec.md) (CLI commands and flags),
[GitHub App Setup Guide](github-app-setup.md) (App registration),
[error-handling.md](../specs/error-handling.md) (degradation strategy).*
