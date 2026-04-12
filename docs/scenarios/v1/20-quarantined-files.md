# Quarantined Files List

### Scenario 135: quarantine run writes quarantined-files.txt before executing the test command [M10]

**Risk:** If quarantined-files.txt is written after the test command runs, or not
written at all, users who opted into file-level exclusion via a shell wrapper
see their quarantined tests run during the current job — defeating the performance
mitigation.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` on the state branch contains 3
quarantined tests across 2 files:
- `spec/models/user_spec.rb::User::validates email` (quarantined)
- `spec/models/user_spec.rb::User::validates password` (quarantined — same file)
- `spec/services/payment_spec.rb::Payment::charges card` (quarantined)

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Reads the state file and builds the quarantined file list from in-memory data.
2. **Before executing the test command**, writes
   `.quarantine/backend/quarantined-files.txt` containing:
   ```
   spec/models/user_spec.rb
   spec/services/payment_spec.rb
   ```
   (2 entries — deduplicated; `user_spec.rb` appears once even though 2 tests
   from it are quarantined)
3. **Then** executes the suite's `command` via `exec.Command`.

The file is a newline-delimited list with no trailing newline required. Paths
match the `file_path` component from the state file entries.

---

### Scenario 136: quarantined-files.txt is written as an empty file when no tests are quarantined [M10]

**Risk:** When no tests are quarantined, quarantine skips writing
quarantined-files.txt. A user's shell wrapper that references the file with
`cat .quarantine/backend/quarantined-files.txt` fails with "file not found,"
breaking their CI workflow even when everything is healthy.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` on the state branch contains an
empty `"tests": {}` map (no quarantined tests)

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Reads the state file — zero quarantined tests.
2. **Before executing the test command**, writes
   `.quarantine/backend/quarantined-files.txt` as an **empty file** (zero bytes).
   The file exists; it is not skipped.
3. Executes the suite's `command`.

A user's shell script that reads the file with `cat` or `wc -l` works
correctly — the file is present and its output (empty / 0 lines) accurately
reflects the quarantine state.
