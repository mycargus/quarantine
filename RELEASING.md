# Release Process

## Overview

Releases follow a two-phase flow: **release candidate (rc)** then **final release**. The rc phase validates the built binary in a real CI environment before promoting to a stable release.

## Prerequisites

- Clean working tree on `main`
- CHANGELOG.md entry for the version (e.g., `## [0.1.0]`)
- Push access to the repository

## Phase 1: Release Candidate

```bash
make release VERSION=v0.1.0-rc1
```

The local `release.sh` script:
1. Validates version format and CHANGELOG entry
2. Verifies clean working tree and no duplicate tag
3. Runs `make check` (all linters + TypeScript typecheck) and CLI, dashboard, and contract tests
4. Verifies `go mod tidy` produces no changes
5. Prompts for confirmation, then creates and pushes the annotated tag

E2E tests are not run locally — they run in the release workflow against the real fixture repo.

The GitHub Actions release workflow (`.github/workflows/release.yml`):
1. Runs CLI, dashboard, and contract test jobs in parallel
2. GoReleaser publishes a **prerelease** (binaries visible, excluded from `latest`)
3. **Pre-release E2E**: runs E2E tests against existing fixture repo artifacts (validates tests still pass)
4. **Trigger fixture repo**: dispatches `workflow_dispatch` on `mycargus/quarantine-test-fixture` with the rc version — fixture installs the rc binary and runs its CI
5. **Post-release E2E**: runs E2E tests against the fixture repo's fresh output (smoke test of the real installed binary)

If the post-release E2E fails, the rc stays as a prerelease. Fix the issue and cut a new rc (e.g., `v0.1.0-rc2`).

## Phase 2: Final Release

After the rc workflow completes successfully (monitor at **Actions → Release**):

```bash
make release VERSION=v0.1.0
```

Local pre-flight checks are skipped (the rc already validated the code). The release workflow runs CLI, dashboard, and contract tests, then GoReleaser publishes a **full release** (since the tag has no prerelease suffix, `prerelease: auto` marks it as stable). The `latest` API endpoint now returns this version.

E2E jobs are skipped for final tags — the rc already validated end-to-end behavior.

## GoReleaser

Configuration: `.goreleaser.yml`

- `prerelease: auto` — tags with a prerelease suffix (e.g., `-rc1`) produce prereleases; clean tags produce full releases
- Builds cross-compiled binaries for linux/darwin (amd64/arm64)
- Produces `checksums.txt` for verification
- Release notes extracted from CHANGELOG.md

## Install Script

`scripts/install.sh` resolves the `latest` stable release by default. Users can pin a specific version:

```bash
VERSION=v0.1.0 bash install.sh
```

The `latest` endpoint excludes prereleases, so rc binaries are only installed when explicitly requested.

## Fixture Repo

`mycargus/quarantine-test-fixture` runs `quarantine run jest-tests` with deliberately flaky tests. Its `upload-test-artifact.yml` workflow accepts a `version` input that controls which quarantine binary to install. The release workflow passes the rc tag (e.g., `v0.1.0-rc1`) so the fixture repo tests the exact prerelease binary.

## One-time Setup

### `release` environment

Configure the `release` environment in the repository settings before the first release:

1. **Settings → Environments → New environment** → name it `release`
2. Under **Deployment branches and tags**, select **Selected branches and tags**
3. **Add deployment branch or tag rule** → enter `v*` as a **tag** pattern
4. Save

No secrets are required — the release job uses the automatic `GITHUB_TOKEN`.

### `CI` environment (E2E tests)

The release workflow runs E2E tests that require GitHub credentials. These are
configured under the `CI` environment (shared with the regular CI workflow):

1. **Settings → Environments** → select or create `CI`
2. **Environment secrets:**
   - `QUARANTINE_GITHUB_TOKEN` — fine-grained PAT scoped to the fixture repo (`QUARANTINE_TEST_OWNER/QUARANTINE_TEST_REPO`) with these permissions:
     - **Actions**: Read and write (trigger fixture CI, watch run status)
     - **Contents**: Read (read quarantine state branch)
     - **Issues**: Read (observe quarantine issues)
     - **Metadata**: Read (required by all fine-grained PATs)
3. **Environment variables:**
   - `QUARANTINE_TEST_OWNER` — GitHub owner for the E2E test repository
   - `QUARANTINE_TEST_REPO` — GitHub repository name for E2E tests

If using a classic PAT, the `repo` scope covers all of the above.

See `test/e2e/README.md` for full E2E setup instructions.

### Signed commits

Require commit signature verification on `main`:

1. **Settings → Branches → Branch protection rules** → edit the `main` rule
2. Enable **Require signed commits**

Or via the CLI:

```bash
gh api repos/mycargus/quarantine/branches/main/protection/required_signatures \
  --method POST
```

All committers must configure GPG or SSH commit signing. See
[GitHub docs on signing commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).

## Installing a Release

### Install script (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash
```

Pin to a specific version:

```bash
curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | VERSION=v0.1.0 bash
```

Install to a custom directory:

```bash
curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | INSTALL_DIR=./bin bash
```

The install script detects OS and architecture, downloads the binary, verifies the
SHA-256 checksum, and installs it.

### Manual download

```bash
VERSION="0.1.0"

# Linux amd64
curl -sL "https://github.com/mycargus/quarantine/releases/download/v${VERSION}/quarantine_${VERSION}_linux_amd64" \
  -o quarantine

# macOS arm64
curl -sL "https://github.com/mycargus/quarantine/releases/download/v${VERSION}/quarantine_${VERSION}_darwin_arm64" \
  -o quarantine

chmod +x quarantine
sudo mv quarantine /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/mycargus/quarantine.git
cd quarantine
go build -o quarantine ./cli/cmd/quarantine
```

## Verify Checksums

Each release includes `checksums.txt` with SHA-256 hashes:

```bash
VERSION="0.1.0"
curl -sL "https://github.com/mycargus/quarantine/releases/download/v${VERSION}/checksums.txt" \
  -o checksums.txt

# Linux
sha256sum --check --ignore-missing checksums.txt

# macOS
shasum -a 256 --check checksums.txt
```

## Rollback

### Patch release (preferred)

Fix forward with a new patch release:

1. Fix the issue on `main`
2. Update `CHANGELOG.md` with a new version entry
3. `make release VERSION=vX.Y.Z` (incrementing the patch version)

### Mark as pre-release (emergency)

If a release needs to be pulled immediately while a fix is in progress:

```bash
gh release edit vX.Y.Z --prerelease --repo mycargus/quarantine
```

This removes it from the "latest" release endpoint. Users with a pinned version
are unaffected. Users relying on `latest` will get the previous stable release.
Revert with `gh release edit vX.Y.Z --latest` after publishing the fix.

### Delete release and tag (last resort)

Only use this if the release was created in error (wrong tag, wrong branch):

```bash
gh release delete vX.Y.Z --yes --repo mycargus/quarantine
git push origin --delete vX.Y.Z
git tag -d vX.Y.Z
```

Warning: this breaks any install scripts or CI configs that reference the
deleted version. Prefer a patch release or pre-release marking instead.
