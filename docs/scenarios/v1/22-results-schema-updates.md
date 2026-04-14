# Results Schema Updates

### Scenario 140: Dashboard ingests results artifact containing unresolved test entries [M10]

**Risk:** The `"unresolved"` test status (ADR-031 section 4) is new to the results schema.
If the dashboard's ingestion pipeline does not handle it, artifacts from runs
with rerun failures or timeouts fail validation and are silently dropped —
losing visibility into infrastructure problems that affect test reliability
data.

**Given** an artifact for `mycargus/my-app` is being ingested with
`suite_name: "backend"`, `run_id: "run-xyz789"`, and
`timestamp: "2026-04-10T14:00:00Z"`, containing these test entries:
- `test_id: "spec/models/user_spec.rb::User::validates email"`,
  `status: "flaky"`, `issue_number: 42` (failed initially, passed on retry)
- `test_id: "spec/models/order_spec.rb::Order::ships on time"`,
  `status: "unresolved"`,
  `error: "rerun timed out after 5m"`,
  `rerun_exit_code: null` (rerun was killed before exiting)
- `test_id: "spec/services/payment_spec.rb::Payment::charges card"`,
  `status: "unresolved"`,
  `error: "rerun command failed: exec: 'bundle': executable file not found in $PATH"`,
  `rerun_exit_code: 127`

**And** the summary includes: `"flaky_detected": 1`, `"unresolved": 2`

**When** the ingestion pipeline processes the artifact

**Then:**
1. The artifact passes JSON Schema validation against
   `schemas/test-result.schema.json`.
2. The flaky test (`validates email`) is ingested normally — `flaky_count`
   incremented, `last_failure_at` updated (existing M7 behavior, scenario 89).
3. Both unresolved test entries (`ships on time`, `charges card`) are stored
   in the test run results with their `status`, `error`, and `rerun_exit_code`
   values preserved.
4. Unresolved tests are **not** added to `quarantined_tests` — they are not
   confirmed flaky and should not appear as quarantined in the dashboard.
5. Unresolved tests are **not** counted as flaky detections —
   `summary.flaky_detected` reflects only the 1 flaky test.
6. The `summary.unresolved` count (2) is stored with the test run record.
7. The artifact is **not** skipped or rejected due to the `"unresolved"` status
   or the presence of `error`/`rerun_exit_code` fields.
