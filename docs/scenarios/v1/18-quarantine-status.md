# Quarantine Status

### Scenario 128: quarantine status shows quarantined count and duration estimate for a suite [M10]

**Risk:** `quarantine status` either crashes when no artifacts are available or
omits the duration estimate without explanation, giving the user no visibility
into the CI time cost of quarantined tests.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` on the state branch contains 3
quarantined tests:
- `spec/models/user_spec.rb::User::validates email` (issue #42, quarantined 45 days ago,
  last failed 2 days ago)
- `spec/models/order_spec.rb::Order::calculates total` (issue #51, quarantined 30 days ago,
  last failed 29 days ago)
- `spec/services/payment_spec.rb::Payment::retries charge` (issue #63, quarantined 3 days ago,
  last failed 3 days ago)

**And** recent artifacts for the `backend` suite contain `duration_ms` data
averaging 4,200ms per quarantined test over the last 10 runs

**When** the developer runs `quarantine status backend`

**Then** the CLI prints:
```
Suite: backend
Quarantined tests: 3
Avg quarantined test duration: 4.2s (from last 10 runs)
Estimated CI time per run on quarantined tests: ~12.6s

Oldest quarantined (consider closing if fixed):
  spec/models/user_spec.rb::validates email         (#42, 45 days, last failed 2 days ago)
  spec/models/order_spec.rb::calculates total       (#51, 30 days, last failed 29 days ago)
  spec/services/payment_spec.rb::retries charge     (#63, 3 days, last failed 3 days ago)
```
Exits with code 0.

---

### Scenario 129: quarantine status with no artifacts available omits duration line [M10]

**Risk:** `quarantine status` crashes or prints a misleading "0s" duration when
no recent artifacts are available (e.g., on a new repo or after artifact expiry).

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` on the state branch contains 2
quarantined tests

**And** no GitHub Artifacts are available for the `backend` suite (either the
repo is newly set up or all artifacts have expired after 90 days)

**When** the developer runs `quarantine status backend`

**Then** the CLI prints (without the duration line):
```
Suite: backend
Quarantined tests: 2

Oldest quarantined (consider closing if fixed):
  spec/models/user_spec.rb::validates email         (#42, 12 days, last failed 11 days ago)
  spec/models/order_spec.rb::calculates total       (#51, 5 days, last failed 4 days ago)
```
Exits with code 0.

The duration estimate line is **omitted** — no "0s" placeholder, no error.

---

### Scenario 130: quarantine status with no argument shows summary for all suites [M10]

**Risk:** `quarantine status` with no argument selects an arbitrary suite or
errors unhelpfully when multiple suites are configured, leaving the developer
without an overview of the repository's full quarantine state.

**Given** `.quarantine/config.yml` with `backend` and `frontend` suites

**And** state files on the state branch:
- `.quarantine/backend/state.json`: 5 quarantined tests
- `.quarantine/frontend/state.json`: 2 quarantined tests

**When** the developer runs `quarantine status` (no suite name)

**Then** the CLI prints a summary for all suites:
```
SUITE      QUARANTINED
backend    5
frontend   2
Total      7

Run `quarantine status <suite-name>` for details including duration estimates
and oldest quarantined tests.
```
Exits with code 0.
