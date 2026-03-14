# ADR-014: Direct Binary Downloads for CLI Distribution

**Status:** Accepted
**Date:** 2026-03-14

## Context

The Go CLI needs to be distributed to CI environments and developer machines. Options evaluated include Homebrew, direct binary downloads, Docker images, and language-ecosystem package managers.

## Decision

Direct binary downloads via GitHub Releases for v1. Each release publishes pre-compiled binaries for linux/darwin/windows + amd64/arm64. Installation in CI is a curl + chmod:

```yaml
- run: |
    curl -sL https://github.com/org/quarantine/releases/latest/download/quarantine-linux-amd64 -o /usr/local/bin/quarantine
    chmod +x /usr/local/bin/quarantine
```

Additionally provide Docker usage instructions for CI environments that prefer containerized tools, and consider a Docker image of the CLI for v1.

v2 additions: Homebrew tap, GitHub Action (`uses: org/quarantine-action@v1` with auto-download), possibly npm/pip wrappers for ecosystem familiarity.

## Alternatives Considered

- **Homebrew:** Useful for developer machines, not CI environments (CI does not use Homebrew). Low value for v1 target (CI integration). Deferred.
- **Docker image only:** Adds docker pull overhead, more verbose invocation (`docker run --rm -v $(pwd):/work quarantine run ...`). Offered as an option but not primary.
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
