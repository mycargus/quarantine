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

1. Verifies the `CHANGELOG.md` entry
2. Runs CLI lint and tests
3. Runs contract tests
4. Builds cross-compiled binaries (linux/darwin × amd64/arm64)
5. Creates a GitHub Release with archives and `checksums.txt`

## One-time setup

Configure the `release` environment in the repository settings before the first release:

1. **Settings → Environments → New environment** → name it `release`
2. Under **Deployment branches and tags**, select **Selected branches and tags**
3. **Add deployment branch or tag rule** → enter `v*` as a **tag** pattern
4. Save

## Installing a release

### Pre-compiled binary (recommended)

```bash
# Linux amd64
gh release download --repo mycargus/quarantine \
  --pattern 'quarantine_*_linux_amd64.tar.gz' --output quarantine.tar.gz
tar xzf quarantine.tar.gz
sudo mv quarantine /usr/local/bin/quarantine

# macOS arm64
gh release download --repo mycargus/quarantine \
  --pattern 'quarantine_*_darwin_arm64.tar.gz' --output quarantine.tar.gz
tar xzf quarantine.tar.gz
sudo mv quarantine /usr/local/bin/quarantine
```

Or download directly from the [Releases page](https://github.com/mycargus/quarantine/releases).

### Build from source

```bash
git clone https://github.com/mycargus/quarantine.git
cd quarantine
go build -o quarantine ./cli/cmd/quarantine
```

## Verify checksums

Each release includes `checksums.txt` with SHA256 hashes:

```bash
shasum -a 256 --check checksums.txt
```
