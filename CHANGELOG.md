# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-03-31

### Added

- `quarantine run` wraps test commands, parses JUnit XML output, retries failures, and manages quarantine state on GitHub
- `quarantine init` creates the `quarantine/state` branch and validates GitHub token permissions
- `quarantine doctor` validates `quarantine.yml` configuration
- `quarantine version` prints the CLI version
- Quarantine state managed via GitHub Contents API with SHA-based compare-and-swap
- GitHub Issues creation for newly quarantined tests with deterministic labels
- PR comments with quarantine status and quarantined test list
- Artifact upload for test results (`.quarantine/results.json`)
- Support for Jest, RSpec, and Vitest test frameworks via JUnit XML
- Degraded mode: falls back to cached `quarantine.json` on GitHub API errors without breaking the build
- Cross-compiled binaries for linux/darwin (amd64/arm64)

[Unreleased]: https://github.com/mycargus/quarantine/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mycargus/quarantine/releases/tag/v0.1.0
