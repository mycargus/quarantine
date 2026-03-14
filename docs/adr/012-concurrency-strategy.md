# ADR-012: Concurrency Strategy

**Status:** Accepted
**Date:** 2026-03-14

## Context

An org may have dozens of CI builds running simultaneously. These builds may concurrently update quarantine.json, detect the same flaky test, or create GitHub issues. The system must handle concurrent operations safely without data loss or excessive duplication.

## Decision

Three-pronged concurrency strategy:

**1. quarantine.json updates: Optimistic concurrency via GitHub Contents API.**

Read returns content + SHA. Write includes the original SHA. GitHub returns 409 Conflict if another write happened since the read. On conflict: re-read, merge changes (union of quarantined tests — quarantine wins), retry. Max 3 retries. If all retries fail, log a warning and continue -- the next build will pick up the state.

**Quarantine-wins merge policy:** On merge conflict, if Build A quarantines a test and Build B unquarantines it (issue closed), the quarantine entry is preserved (union strategy). The CLI logs: "Test '{test_name}' was unquarantined (issue closed) but re-quarantined due to concurrent detection. It will be unquarantined on the next build." The test is unquarantined on the very next clean build when the CLI re-checks the issue status and finds it closed, so the impact is one extra build cycle at most.

**2. GitHub issue creation: Check-before-create with deterministic labels.**

Before creating an issue for a flaky test, search for open issues with label `quarantine` and label `quarantine:{test_hash}`. If found, link to the existing issue. If not found, create a new issue with both labels. A small race window between check and create may produce rare duplicates -- acceptable, as a human closes the duplicate in seconds. The quarantine.json entry also records the issue URL, providing a secondary deduplication mechanism.

**3. Dashboard writes: SQLite in WAL (Write-Ahead Log) mode.**

Concurrent reads are fast and non-blocking. Writes are serialized but WAL mode handles hundreds of writes per second -- far beyond what CI generates. Result ingestion is idempotent (keyed by run ID; duplicate processing is a no-op).

## Alternatives Considered

- **Distributed locking (e.g., advisory locks via API):** Complex, fragile, adds latency. Optimistic concurrency is simpler and sufficient.
- **Queue-based writes:** Adds infrastructure (message queue). Overkill for this throughput.
- **Last-write-wins:** Simple but causes data loss (one build's quarantine entry overwrites another's). Rejected.

## Consequences

**Positive:**
- No external locking infrastructure needed.
- Conflict resolution is automatic and transparent.
- Rare duplicates are harmless, not data loss.
- SQLite WAL handles realistic write throughput easily.

**Negative:**
- Optimistic concurrency retries add latency during high-contention periods (many builds finishing simultaneously).
- Merge logic must be carefully implemented (union semantics, not overwrite).
- Rare duplicate issues require manual cleanup.
