# ADR-014: Direct Binary Downloads for CLI Distribution

**Status:** Accepted
**Date:** 2026-03-14

## Context

The Go CLI needs to be distributed to CI environments and developer machines. Options evaluated include Homebrew, direct binary downloads, and language-ecosystem package managers.

## Decision

Direct binary downloads via GitHub Releases for v1. Each release publishes pre-compiled binaries for linux/darwin + amd64/arm64. An install script handles OS/arch detection, download, and checksum verification:

```yaml
- name: Install quarantine
  run: curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash
  env:
    VERSION: v0.1.0  # pin to a specific version
```

Or download directly with checksum verification:

```yaml
- name: Install quarantine
  run: |
    VERSION="0.1.0"
    curl -sL "https://github.com/mycargus/quarantine/releases/download/v${VERSION}/quarantine_${VERSION}_linux_amd64" \
      -o /usr/local/bin/quarantine
    curl -sL "https://github.com/mycargus/quarantine/releases/download/v${VERSION}/checksums.txt" \
      -o /tmp/checksums.txt
    cd /usr/local/bin && grep "quarantine_${VERSION}_linux_amd64" /tmp/checksums.txt | sha256sum --check
    chmod +x /usr/local/bin/quarantine
```

v2 additions: Homebrew tap, GitHub Action (`uses: org/quarantine-action@v1` with auto-download), possibly npm/pip wrappers for ecosystem familiarity.

## Alternatives Considered

- **Homebrew:** Useful for developer machines, not CI environments (CI does not use Homebrew). Low value for v1 target (CI integration). Deferred.
- **Docker image:** The CLI wraps test runners and needs access to the user's test suite, dependencies, and framework binaries. A Docker image would require mounting the entire project workspace and test infrastructure, negating the simplicity of a single binary. Not viable.
- **GitHub Action:** Convenient for GitHub Actions users, but limits portability. Good v2 addition.

## Consequences

**Positive:**
- Simplest possible distribution -- one curl command.
- No package manager dependencies.
- Works in any CI environment with curl/wget.
- GitHub Releases is free and reliable.

**Negative:**
- No auto-updates -- user must manually update version in CI config. Mitigated by v2 GitHub Action which can pin to `@v1` for auto-minor-updates.
- The curl-pipe-sh pattern has security concerns. Mitigated by publishing checksums and supporting checksum verification.
