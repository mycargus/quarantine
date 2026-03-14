# ADR-004: Go for CLI Language

**Status:** Accepted
**Date:** 2026-03-14

## Context

Need to choose a language for the CLI binary. Requirements: single binary with no runtime dependencies, cross-compilation support, and a good ecosystem for XML parsing, HTTP requests, and subprocess management.

## Decision

Go.

## Alternatives Considered

- **Rust:** Better raw performance (not needed here -- the CLI parses XML, makes HTTP calls, and spawns subprocesses). Slower compile times, smaller hiring pool, and steeper learning curve.
- **Python:** No single-binary story. Requires a runtime, and packaging/distribution adds significant complexity.
- **Node/TypeScript:** No single-binary story. Requires a runtime, same distribution problems as Python.

## Consequences

**Positive:**
- Single binary, approximately 10-15MB, no runtime dependencies.
- Trivial cross-compilation via GOOS/GOARCH environment variables.
- Batteries-included standard library: encoding/xml, net/http, os/exec.
- cobra for CLI framework is mature and well-supported.
- Fast compile times for developer velocity.
- Larger contributor pool than Rust.

**Negative:**
- Larger binary than Rust (approximately 10-15MB vs 5-8MB). Negligible concern for a CLI tool.
- GC pauses are theoretically possible but irrelevant for a CLI that runs for seconds.
