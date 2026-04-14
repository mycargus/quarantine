# Dashboard State Enumeration

### Scenario 137: Dashboard reads per-suite state files from the state branch [M10]

**Risk:** Without reading the authoritative state files from the state branch,
the dashboard can only infer quarantine state from historical artifacts — which
may be stale, expired (90-day retention), or incomplete. Users see inaccurate
quarantine counts or stale "last failed" dates because the dashboard has no
direct connection to the CLI's ground truth.

**Given** the dashboard is configured with `mycargus/my-app`

**And** the `quarantine/state` branch has:
- `.quarantine/backend/state.json` containing 3 quarantined tests:
  - `spec/models/user_spec.rb::User::validates email` (issue #42, quarantined
    45 days ago)
  - `spec/models/order_spec.rb::Order::calculates total` (issue #51,
    quarantined 30 days ago)
  - `spec/services/payment_spec.rb::Payment::charges card` (issue #63,
    quarantined 3 days ago)
- `.quarantine/frontend/state.json` containing 1 quarantined test:
  - `src/components/Login.test.tsx::Login::handles timeout` (issue #70,
    quarantined 7 days ago)

**When** the dashboard syncs state for `mycargus/my-app`

**Then** the dashboard:
1. Lists the `.quarantine/` directory on the `quarantine/state` branch (1 API
   call via the GitHub Contents API) — discovers `backend/` and `frontend/`
   subdirectories.
2. Reads `.quarantine/backend/state.json` (1 API call) — parses 3 quarantined
   test entries with their `test_id`, `issue_number`, `quarantined_at`, and
   `last_failure_at` fields.
3. Reads `.quarantine/frontend/state.json` (1 API call) — parses 1 quarantined
   test entry.
4. Stores the per-suite quarantine state, keyed by repository and suite name.
5. Total API calls for this sync: 3 (1 directory listing + 2 state file reads).

---

### Scenario 138: Dashboard handles missing .quarantine/ directory on the state branch [M10]

**Risk:** When no suites have ever run `quarantine run`, the `.quarantine/`
directory does not exist on the state branch (only the initial `README.md` from
`quarantine init`). The dashboard crashes or displays an error page, making the
tool appear broken to users who have just set up quarantine but haven't run CI
yet.

**Given** the dashboard is configured with `mycargus/my-app`

**And** the `quarantine/state` branch exists (created by `quarantine init`)
but contains only a `README.md` — no `.quarantine/` directory exists because
no suite has been run yet

**When** the dashboard syncs state for `mycargus/my-app`

**Then** the dashboard:
1. Requests the `.quarantine/` directory listing — receives **404 Not Found**
   from the GitHub Contents API.
2. Treats the 404 as "no quarantine state data available" — **not** as an error.
3. Continues to display artifact-based data if any exists (from previous
   ingestion cycles).
4. Does **not** log an error or trigger the circuit breaker — a missing
   `.quarantine/` directory is an expected state for newly initialized repos.
5. On the next sync cycle, retries the directory listing normally (no backoff
   penalty).

---

### Scenario 139: Dashboard handles partial state when some suites have not run [M10]

**Risk:** A repository has multiple configured suites but only some have
completed their first `quarantine run`. Missing state files for un-run suites
cause the dashboard to abort state enumeration entirely, losing data for suites
that do have state files.

**Given** the dashboard is configured with `mycargus/my-app`

**And** the `quarantine/state` branch has:
- `.quarantine/backend/state.json` containing 2 quarantined tests
- No `.quarantine/frontend/state.json` (the frontend suite is configured in
  `.quarantine/config.yml` but has never run `quarantine run frontend`)

**When** the dashboard syncs state for `mycargus/my-app`

**Then** the dashboard:
1. Lists `.quarantine/` — discovers `backend/` subdirectory (and possibly
   `frontend/` if the directory was created without a state file).
2. Reads `.quarantine/backend/state.json` — successfully parses 2 quarantined
   test entries.
3. Attempts to read `.quarantine/frontend/state.json` — receives 404.
4. Stores the backend suite's quarantine state normally.
5. Treats the frontend suite as having no quarantine state — shows zero
   quarantined tests for that suite, **not** an error.
6. Does **not** skip or discard the backend suite's data because of the
   frontend suite's missing file — each suite's state is read independently.
