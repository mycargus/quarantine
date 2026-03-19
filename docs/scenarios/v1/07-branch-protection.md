# Branch Protection

### Scenario 41: CLI updates quarantine.json on unprotected branch [M4]

**Risk:** The standard write path for quarantine state fails on unprotected branches, blocking the most common deployment configuration.

**Given** the `quarantine/state` branch is not protected, and the CLI has
detected a new flaky test

**When** the CLI writes the updated `quarantine.json` to the `quarantine/state`
branch via the GitHub Contents API

**Then** the write succeeds directly via the Contents API PUT with SHA-based
optimistic concurrency, and `quarantine.json` is updated.

---

### Scenario 42: CLI updates quarantine.json when branch is protected [M4]

**Risk:** Branch protection rules block quarantine state updates with no fallback, causing flaky test detections to be silently lost.

**Given** the `quarantine/state` branch has branch protection rules enabled
(e.g., required reviews, status checks), and the CLI has detected a new flaky
test

**When** the CLI attempts to write `quarantine.json` via the Contents API and
receives a 403 or 422 error indicating the branch is protected

**Then** the CLI falls back to storing the pending quarantine state update in
the GitHub Actions cache (keyed by run ID), logs:
`[quarantine] WARNING: Branch 'quarantine/state' is protected. Quarantine state
saved to Actions cache. A workflow with write access must apply the update.`
The CI build still exits with code 0 (the flaky test is treated as quarantined
for this run based on the pending update).

---
