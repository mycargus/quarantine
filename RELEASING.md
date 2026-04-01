# Releasing

This document describes the release process for the quarantine CLI.

## Prerequisites

- Clean working tree on `main`
- `CHANGELOG.md` updated with release notes under `## [X.Y.Z]`
- Push access to the repository

## Process

```
make release VERSION=v0.1.0
```

`make release` runs `scripts/release.sh`, which:

1. Validates the version format (`vX.Y.Z`)
2. Verifies `CHANGELOG.md` has an entry for the version
3. Verifies the working tree is clean
4. Verifies the tag does not already exist
5. Runs `make check` (lint + typecheck)
6. Runs `make cli-test`
7. Runs `make contract-test`
8. Verifies `go mod tidy` produces no changes
9. Prints the release notes and prompts for confirmation
10. Creates an annotated tag and pushes it

GitHub Actions takes over on tag push:

1. Runs CLI build, test, and lint (parallel)
2. Runs dashboard lint, typecheck, and test (parallel)
3. Runs contract lint and tests (parallel)
4. Runs E2E lint and tests (parallel)
5. After all jobs pass: verifies CHANGELOG entry, builds cross-compiled
   binaries (linux/darwin × amd64/arm64), creates GitHub Release with
   binaries and `checksums.txt`

## One-time setup

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
   - `QUARANTINE_GITHUB_TOKEN` — PAT with `repo` scope for the test repository
3. **Environment variables:**
   - `QUARANTINE_TEST_OWNER` — GitHub owner for the E2E test repository
   - `QUARANTINE_TEST_REPO` — GitHub repository name for E2E tests

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

## Installing a release

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

## Verify checksums

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
