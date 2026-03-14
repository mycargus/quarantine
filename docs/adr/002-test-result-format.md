# ADR-002: JUnit XML as Test Result Format

**Status:** Accepted
**Date:** 2026-03-14

## Context

The CLI needs to parse test results from any test framework. A universal format is required to avoid building and maintaining framework-specific parsers.

## Decision

JUnit XML. The CLI parses JUnit XML output to identify failures, extract test identifiers, and produce modified output.

## Alternatives Considered

- **Framework-specific parsers (one per framework):** More accurate but creates an exponential maintenance burden. Each new framework requires a dedicated parser.
- **TAP (Test Anything Protocol):** Less widely supported than JUnit XML across the ecosystem.
- **Custom output format:** Requires test framework plugins, introducing high friction for adoption.

## Consequences

**Positive:**
- Widely-adopted de facto format with no official schema. The JUnit project itself has an open issue (#2625) requesting an official XSD. Cross-framework variations exist.
- Broad support across frameworks, though some require third-party tools:
  - v1 supported frameworks:
    - **RSpec:** requires the third-party `rspec_junit_formatter` gem.
    - **Jest:** requires the third-party `jest-junit` package.
    - **Vitest:** has built-in JUnit XML support via `--reporter=junit` (no third-party package needed).
  - v2+ supported frameworks:
    - **Go:** requires the third-party `gotestsum` tool.
    - **NUnit:** uses its own XML format, NOT JUnit XML; requires an XSLT transform to convert.
    - pytest, PHPUnit, Maven, and Gradle export JUnit XML natively.
- Single parser implementation covers the supported framework matrix (after format normalization).

**Negative:**
- JUnit XML schema varies slightly across frameworks -- the parser must handle variations. Mitigated by framework fingerprinting from XML structure.
- Some metadata may be lost in translation (framework-specific fields).
