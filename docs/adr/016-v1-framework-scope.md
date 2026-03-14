# ADR-016: v1 Framework Scope (RSpec, Jest, Vitest)

**Status:** Accepted
**Date:** 2026-03-14

## Context

The CLI needs to support specific test frameworks for v1. Framework support means: parsing JUnit XML output (which varies by framework), constructing rerun commands for individual failing tests, and auto-detecting the framework from XML structure. Supporting a framework requires understanding its JUnit XML format, its single-test invocation syntax, and any third-party dependencies needed for JUnit XML output.

## Decision

v1 supports RSpec, Jest, and Vitest. All other frameworks (pytest, Go/gotestsum, JUnit/Maven, PHPUnit, NUnit) are deferred to v2+.

Rationale:

- The target v1 customer (the developer's employer) primarily uses these frameworks.
- **RSpec:** The Ruby flaky test ecosystem is dead. rspec-retry was archived in July 2025 by NoRedInk. The Flexport quarantine gem (this project's inspiration) depends on rspec-retry and is effectively unmaintained. There is a clear gap in the Ruby/RSpec community for a modern flaky test solution.
- **Jest:** The most widely used JavaScript test framework. Requires jest-junit third-party reporter for JUnit XML output.
- **Vitest:** Growing rapidly as the modern Vite-native test framework. Has built-in JUnit XML support (no third-party dependency). Many teams migrating from Jest to Vitest.
- **Python (pytest) deferred:** Not a priority for the employer's stack. Can be added in v2 without architectural changes since the JUnit XML parsing layer is framework-agnostic.

## Alternatives Considered

- **pytest first (most popular in OSS, largest user base):** Rejected because the employer does not prioritize it and the framework-agnostic JUnit XML layer means adding it later is low-effort.
- **Support all frameworks from v1:** Rejected to reduce scope and ensure quality of the v1 implementations.
- **Framework-agnostic only (no rerun, just detection from XML):** Rejected because rerun is the core detection mechanism. Without framework-specific rerun commands, the CLI can only detect flakiness at the suite level, not the individual test level.

## Consequences

- (+) Focused v1 scope with high-quality support for three frameworks.
- (+) RSpec support fills a market gap left by archived tools.
- (+) Vitest support targets a growing ecosystem early.
- (+) Adding frameworks in v2+ requires only: rerun command template, JUnit XML parsing quirks, and optional auto-detection heuristics.
- (-) Python/Go/Java teams cannot use the full feature set in v1 (though basic JUnit XML parsing still works for detection if the user provides a rerun_command override in config).
