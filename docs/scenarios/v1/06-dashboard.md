# Dashboard

### Scenario 36: User views org-wide flaky test overview [M7]

**Risk:** Users cannot see the aggregate flaky test problem across their organization, missing systemic quality issues that span multiple repositories.

**Given** the user is viewing the dashboard and the `dashboard.yml`
configuration includes 4 repositories with Quarantine configured, containing a
combined 12 quarantined tests

**When** the user navigates to the org-level overview page

**Then** the dashboard displays a summary showing total quarantined tests across
all repos (12), a breakdown per repository with test counts, the most recently
quarantined tests, and links to drill into each project's details.

---

### Scenario 37: User views single project's flaky test details and trends [M7]

**Risk:** Users cannot determine whether the flaky test situation in a project is improving or worsening, making it impossible to measure the impact of quality initiatives.

**Given** the user selects the repository `acme/payments-service`, which has
3 quarantined tests

**When** the project detail page loads

**Then** the dashboard displays a list of all 3 quarantined tests with their
names, date first quarantined, last flaky occurrence, links to their
corresponding GitHub Issues, and a trend chart showing flaky test count over
time (data derived from ingested GitHub Artifacts history).

---

### Scenario 38: User filters and searches quarantined tests on dashboard [M7]

**Risk:** Users with many quarantined tests cannot find specific ones, making the dashboard unusable for targeted investigation.

**Given** the user is viewing a repository with 15 quarantined tests

**When** the user types `timeout` into the search bar and selects the filter
`Status: Still Failing`

**Then** the dashboard filters the list to show only quarantined tests whose
names contain `timeout` and whose most recent run result was a failure,
updating the displayed count accordingly.

---

### Scenario 39: Dashboard polls artifacts and ingests new results [M6]

**Risk:** The dashboard shows stale data because artifact polling fails to discover new results, undermining trust in the analytics.

**Given** the dashboard is running with `dashboard.yml` configured as:
```yaml
source: manual
repos:
  - owner: mycargus
    repo: my-app
poll_interval: 300
```
and its last successful poll was 5 minutes ago

**When** the background polling interval elapses

**Then** the dashboard:
1. Queries the GitHub Artifacts API for `mycargus/my-app` with
   `If-None-Match` (ETag) header for conditional requests.
2. Filters artifacts by name prefix `quarantine-results`.
3. Downloads any new artifacts (follows 302 redirect to blob storage).
4. Extracts zip, parses JSON, validates against
   `schemas/test-result.schema.json`.
5. Upserts into SQLite (keyed by `run_id` for idempotency).
6. If the ETag matches (304 Not Modified), skips processing — this does not
   count against the rate limit.

---

### Scenario 40: Dashboard circuit breaker pauses polling after failures [M6]

**Risk:** Persistent polling failures exhaust the GitHub API rate limit budget, starving other repositories of polling capacity (ADR-015).

**Given** the dashboard is polling a repository and 3 consecutive GitHub API
calls have failed (e.g., 500 Internal Server Error)

**When** the circuit breaker threshold (3 consecutive failures) is reached

**Then** the dashboard pauses polling for that repository for 30 minutes. After
the pause, the dashboard resumes polling. On the first successful poll, the
circuit breaker resets. Per ADR-015.

---

### Scenario 71: Project listing page shows repo names, run counts, and last sync time [M6]

**Risk:** Users have no way to see which repositories are configured or whether data ingestion is working, making the dashboard appear empty or broken.

**Given** the dashboard is running with `dashboard.yml` configured as:
```yaml
source: manual
repos:
  - owner: mycargus
    repo: my-app
  - owner: acme
    repo: payments-service
```
and SQLite contains 5 ingested test runs for `mycargus/my-app` (last synced
2025-01-15T10:30:00Z) and 0 test runs for `acme/payments-service` (never synced)

**When** the user navigates to the project listing page (`/`)

**Then** the dashboard displays a list of all configured repositories showing:
1. Repository name (`mycargus/my-app`, `acme/payments-service`).
2. Test run count (5 for `mycargus/my-app`, 0 for `acme/payments-service`).
3. Last sync timestamp (`2025-01-15T10:30:00Z` for `mycargus/my-app`, "Never"
   for `acme/payments-service`).

---

### Scenario 72: Malformed artifact JSON is skipped with a warning [M6]

**Risk:** A single malformed artifact crashes the ingestion pipeline, preventing all subsequent valid artifacts from being processed.

**Given** the dashboard is polling `mycargus/my-app` and discovers 3 new
artifacts: `quarantine-results-100` (valid JSON), `quarantine-results-101`
(malformed JSON — missing required `run_id` field), and
`quarantine-results-102` (valid JSON)

**When** the ingestion pipeline processes the 3 artifacts

**Then** the dashboard:
1. Successfully ingests artifact 100 into SQLite.
2. Skips artifact 101 and logs a warning:
   `[ingest] WARNING: skipping artifact quarantine-results-101 for mycargus/my-app: validation failed`.
3. Successfully ingests artifact 102 into SQLite.
4. Does not crash or stop processing remaining artifacts.

---

### Scenario 76: On-demand pull is debounced per-repo (max 1 per 5 minutes) [M6]

**Risk:** Without debounce, every page load hammers the GitHub API, burning through the
1,000 req/hr budget in minutes for a team that keeps a dashboard tab open (NFR-2.3.4).

**Given** the dashboard has two repos configured:
- `mycargus/my-app` last pulled at `2026-03-28T10:00:00Z` (stale)
- `acme/payments-service` last pulled at `2026-03-28T10:04:00Z` (fresh)

**When** the debounce check runs at `2026-03-28T10:06:00Z`

**Then:**
1. `shouldPull` returns `true` for `mycargus/my-app` (6 min > 5 min threshold).
2. `shouldPull` returns `false` for `acme/payments-service` (2 min < 5 min threshold).
3. If `last_pulled_at` is `null` (first view ever), `shouldPull` returns `true`.
4. After a successful pull, `last_pulled_at` is updated to the current time.
5. At exactly 5 minutes since last pull, `shouldPull` returns `false`
   (strict greater-than — not yet stale).

---

### Scenario 86: Dashboard triggers on-demand sync on first page load and displays real data [M7]

**Risk:** The dashboard shows an empty state on first visit because no sync has ever run — the on-demand sync must fire automatically on page load to populate the database and display real data. (FR-1.5.2, FR-1.5.3)

**Given** the dashboard is running with `dashboard.yml` configured as:
```yaml
source: manual
repos:
  - owner: mycargus
    repo: my-app
```
and `QUARANTINE_GITHUB_TOKEN` is set in the environment, and SQLite contains no data for
`mycargus/my-app` (`last_pulled_at` is `null`), and GitHub has 3 artifacts named
`quarantine-results-1`, `quarantine-results-2`, `quarantine-results-3` for `mycargus/my-app`,
each containing valid test result JSON

**When** the user navigates to the home page (`/`)

**Then:**
1. The dashboard detects `last_pulled_at` is `null` → `shouldPull` returns `true`.
2. The dashboard calls the GitHub Artifacts API for `mycargus/my-app`.
3. All 3 artifacts matching the `quarantine-results` prefix are downloaded and ingested into SQLite (both `test_runs` and `quarantined_tests` populated).
4. `last_pulled_at` is updated to the current time after a successful sync.
5. The home page renders with the ingested data (test run counts, last synced time, quarantined test totals).
6. A second page load within 5 minutes does NOT re-trigger a GitHub API call (`shouldPull` returns `false`).
7. If the sync fails (no token, 401, 500, network unreachable), the page still renders with whatever data SQLite has (possibly empty) and logs a warning — sync failure MUST NOT produce an HTTP error page.

---

### Scenario 87: Artifact ingestion populates quarantined_tests from test entries [M7]

**Risk:** Dashboard analytics show zero quarantined tests even when artifacts contain quarantine data, because the ingest pipeline only stores run-level summaries and never extracts the per-test quarantine entries. (FR-1.5.1)

**Given** an artifact for `mycargus/my-app` is being ingested with `run_id: "run-abc123"` and `timestamp: "2026-02-10T14:00:00Z"`, containing these test entries:
- `test_id: "spec/payments_spec.rb::PaymentsService::processes_payment"`, `status: "quarantined"`, `original_status: "failed"`, `issue_number: 42`
- `test_id: "spec/auth_spec.rb::Auth::login"`, `status: "quarantined"`, `original_status: "passed"`, `issue_number: 43`
- `test_id: "spec/cart_spec.rb::Cart::checkout"`, `status: "quarantined"`, `original_status: null` (excluded pre-execution by Jest/Vitest), `issue_number: 44`

**When** the ingestion pipeline processes this artifact

**Then:**
1. All 3 entries are upserted into `quarantined_tests`.
2. `quarantined_at` is set to `"2026-02-10T14:00:00Z"` (the artifact's `timestamp`) for all new entries.
3. The entry with `original_status: "failed"` has `last_run_status: "failing"`.
4. The entry with `original_status: "passed"` has `last_run_status: "passing"`.
5. The entry with `original_status: null` has `last_run_status: null`.
6. `issue_url` is `https://github.com/mycargus/my-app/issues/42` for issue_number 42 (and similarly for 43, 44).
7. The `test_runs` row for `run-abc123` is also inserted (existing M6 behavior is unchanged).

---

### Scenario 88: Subsequent artifact ingestion updates last_run_status but preserves quarantined_at [M7]

**Risk:** Re-ingesting a later artifact overwrites the original quarantine date, destroying the history of when a test was first quarantined and breaking the "date first quarantined" display. (FR-1.5.1)

**Given** `quarantined_tests` already contains an entry for
`spec/payments_spec.rb::PaymentsService::processes_payment` in project `mycargus/my-app`
with `quarantined_at: "2026-01-15T09:00:00Z"`, `last_run_status: "failing"`, and
`flaky_count: 1` (set by earlier artifacts), and a new artifact is ingested with
`timestamp: "2026-03-01T10:00:00Z"` containing the same test entry with
`status: "quarantined"` and `original_status: "passed"` (the test is still quarantined
but now passing in this run)

**When** the ingestion pipeline processes the new artifact

**Then:**
1. `last_run_status` is updated to `"passing"` (reflecting the most recent run).
2. `quarantined_at` remains `"2026-01-15T09:00:00Z"` (the original quarantine date is preserved — not overwritten).
3. `flaky_count` remains `1` (unchanged — this run was not a flaky detection).
4. No duplicate row is created — the existing row is updated in place.

---

### Scenario 89: Flaky detection increments flaky_count and updates last_flaky_at [M7]

**Risk:** The "last flaky occurrence" column always shows "Never" and flaky count is always 0, because the ingest pipeline never processes `status: "flaky"` entries — making it impossible to gauge how often a quarantined test is still misbehaving. (FR-1.5.1)

**Given** an artifact for `mycargus/my-app` is being ingested with
`timestamp: "2026-03-15T14:00:00Z"`, containing these test entries:
- `test_id: "spec/payments_spec.rb::PaymentsService::processes_payment"`, `status: "flaky"`,
  `issue_number: 42` (failed initially, passed on retry — a new flaky detection for an
  already-quarantined test)
- `test_id: "spec/auth_spec.rb::Auth::login"`, `status: "quarantined"`,
  `original_status: "passed"`, `issue_number: 43` (quarantined, running normally — NOT
  a flaky detection)

and `quarantined_tests` already has entries for both tests with `flaky_count: 2` and
`last_flaky_at: "2026-02-20T10:00:00Z"`

**When** the ingestion pipeline processes the artifact

**Then:**
1. The `processes_payment` entry: `flaky_count` is incremented to `3` and `last_flaky_at` is updated to `"2026-03-15T14:00:00Z"`.
2. The `login` entry: `flaky_count` remains `2` and `last_flaky_at` remains `"2026-02-20T10:00:00Z"` — `status: "quarantined"` is NOT a flaky detection.
3. `last_run_status` for `processes_payment` is set to `"passing"` (a flaky test ultimately passed on retry).

---
