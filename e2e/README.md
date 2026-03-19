# E2E Tests

End-to-end tests that exercise the full Quarantine CLI against real GitHub
dependencies. Written in JavaScript with Vitest as the runner and
[`riteway`](https://github.com/paralleldrive/riteway) assertions.

These tests are intentionally separate from the CLI's unit and integration
test suite. They run against a dedicated GitHub repository and require
credentials that are not available in standard CI runs from forks.

## Requirements

- Node.js ≥ 20
- The quarantine CLI binary must be built: `cd cli && make cli-build`
- Three environment variables (see below)

## Environment Variables

| Variable | Description |
|---|---|
| `QUARANTINE_GITHUB_TOKEN` | PAT or fine-grained token with repo read/write access |
| `QUARANTINE_TEST_OWNER` | GitHub org or username that owns the test repository |
| `QUARANTINE_TEST_REPO` | Name of the test repository (e.g. `quarantine-test-fixture`) |
| `QUARANTINE_BIN` | *(optional)* Path to the quarantine binary. Defaults to `../cli/bin/quarantine`. |

Tests skip automatically when the required env vars are absent — running
`npm test` locally without credentials is safe and produces a clean skip.

## Setting Up the Test Repository

The E2E suite needs a real GitHub repository to run `quarantine init` against.
This repository must exist and have at least one commit on its default branch
so that the API can return a SHA for branch creation.

### 1. Create the repository

```bash
gh repo create <your-org>/quarantine-test-fixture --public --add-readme
```

A public repo with a README is sufficient. The E2E test creates and deletes the
`quarantine/state` branch on every run, so the repo never accumulates state.

### 2. Create a Personal Access Token

The token needs write access to repository contents so it can create branches
and files. Two options:

**Classic PAT** (simpler):
- Go to: GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
- Scopes required: `repo` (full control of private repositories — also covers public)

**Fine-grained token** (more restrictive):
- Go to: GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens
- Resource owner: the org or user that owns the test repo
- Repository access: only `quarantine-test-fixture`
- Permissions required: **Contents** (read and write)

### 3. Configure CI secrets and variables

In the quarantine repository (the one that runs CI, not the test fixture),
add the following under **Settings → Secrets and variables → Actions**:

| Type | Name | Value |
|---|---|---|
| Secret | `QUARANTINE_GITHUB_TOKEN` | The PAT from step 2 |
| Variable | `QUARANTINE_TEST_OWNER` | Your GitHub org or username |
| Variable | `QUARANTINE_TEST_REPO` | `quarantine-test-fixture` |

Owner and repo are stored as **variables** (not secrets) because they are
non-sensitive and visible in CI logs.

## Running Locally

```bash
# Build the CLI binary first
cd cli && make cli-build && cd ..

# Set credentials
export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here
export QUARANTINE_TEST_OWNER=your-org
export QUARANTINE_TEST_REPO=quarantine-test-fixture

# Install deps and run
cd e2e
pnpm install
pnpm test
```

## Adding New E2E Tests

Each milestone that exercises real GitHub API flows should have a corresponding
test file here. Name files after the command or feature being tested:

```
e2e/
  init.test.js       # quarantine init (M1)
  run.test.js        # quarantine run full flow (M4/M5)
  ...
```