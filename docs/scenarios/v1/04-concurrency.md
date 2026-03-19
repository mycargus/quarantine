# Concurrency

### Scenario 27: Concurrent CI builds detect the same flaky test simultaneously [M5]

**Risk:** Concurrent builds each create a separate GitHub Issue for the same flaky test, flooding the repository with duplicates (ADR-009, ADR-012).

**Given** the CLI is configured in CI, `quarantine.json` has no entry for
`CacheService > should handle eviction`, and two CI builds (Build A and Build B)
are running in parallel on different PRs

**When** both builds detect `should handle eviction` as flaky and both attempt
to create a GitHub Issue for it

**Then** the first build to reach GitHub creates the Issue titled
`[Quarantine] CacheService > should handle eviction` with labels `["quarantine",
"quarantine:{test_hash}"]`. The second build uses check-before-create (searches
for an existing open issue with matching deterministic label
`quarantine:{test_hash}`) and finds the issue already exists, so it skips issue
creation. Both builds succeed without duplicate issues.

**Note:** There is a small race window between the dedup search returning
"no issue" and the POST to create one. In rare cases, both builds may create
issues. This is accepted per ADR-012 — a human closes the duplicate, and the
next build finds the first issue.

---

### Scenario 28: Concurrent CI builds update quarantine.json simultaneously (CAS conflict) [M4]

**Risk:** A concurrent write overwrites another build's quarantine state changes, silently losing flaky test detections (ADR-006, ADR-012).

**Given** two CI builds (Build A and Build B) are running in parallel. Both have
fetched `quarantine.json` at SHA `abc123` from the `quarantine/state` branch.

**When** Build A writes its update to `quarantine.json` first (new SHA
`def456`), and then Build B attempts to write its update using the stale SHA
`abc123`

**Then** Build B's write fails with a 409 Conflict from the GitHub Contents API
because the SHA no longer matches (ADR-006, ADR-012). Build B:
1. Re-fetches `quarantine.json` at the new SHA `def456`.
2. Merges its changes with Build A's changes using the union strategy
   (quarantine wins — per ADR-012).
3. Retries the write with the updated SHA.
4. The resulting `quarantine.json` contains both builds' updates without data
   loss.

If all 3 retry attempts fail (409 each time), Build B logs a warning and skips
the write. The flaky test will be re-detected on the next CI run. This is safe
because re-detection is cheap.

---

### Scenario 29: Concurrent quarantine and unquarantine race [M4]

**Risk:** A race condition between quarantine and unquarantine permanently re-enables a flaky test that concurrent builds detected as still flaky (ADR-012).

**Given** `quarantine.json` contains an entry for
`CacheService > should handle eviction` with an open GitHub Issue. Two CI builds
(Build A and Build B) are running in parallel. Build A detects a new flaky test
`ApiService > should handle timeout` while Build B finds that the issue for
`should handle eviction` has been closed and removes it from its local copy of
`quarantine.json`.

**When** Build A writes first (adding `should handle timeout`, keeping
`should handle eviction`), and Build B attempts to write (removing
`should handle eviction`) using a stale SHA, triggering a 409 Conflict

**Then** Build B re-reads `quarantine.json`, merges using the quarantine-wins
(union) strategy per ADR-012, and the resulting `quarantine.json` contains both
`should handle eviction` and `should handle timeout`. Build B logs:
`[quarantine] WARNING: Test 'CacheService > should handle eviction' was
unquarantined (issue closed) but re-quarantined due to concurrent update. It
will be unquarantined on the next build.`

On the very next CI run, the CLI checks the issue status, finds it closed, and
removes `should handle eviction` from `quarantine.json`. The impact is one extra
build cycle where the test remains quarantined — excluded from execution
(Jest/Vitest) or failure-suppressed (RSpec) rather than running and potentially
failing. This is the safe default.

---
