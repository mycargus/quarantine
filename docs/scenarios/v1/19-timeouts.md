# Timeout Enforcement

### Scenario 131: test command timeout sends SIGTERM then SIGKILL, processes partial XML [M10]

**Risk:** A hanging test runner blocks the CI pipeline indefinitely. Without
quarantine-level timeouts, the CI job waits for its own (often 6h) timeout,
wasting machine capacity and delaying feedback.

**Given** `.quarantine/config.yml` with a `backend` suite:
```yaml
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
    timeout: 1m
```

**And** the `bundle exec rspec` command runs but hangs after writing partial
`rspec.xml` (80 tests recorded, then the process stalls — e.g., waiting on a
deadlocked database connection)

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Starts the `bundle exec rspec` process.
2. After **1 minute** (the configured `timeout`), sends `SIGTERM` to the process.
3. Waits up to **5 seconds** for the process to exit gracefully.
4. If still running after 5 seconds, sends `SIGKILL`.
5. Detects that `rspec.xml` exists and contains partial results (80 tests).
6. Processes the partial XML as if it were a complete run — any failures in the
   partial results are retried and classified normally.
7. Prints to stderr:
   ```
   Error [timeout]: test command timed out after 1m.
   Partial results processed: 80 tests from rspec.xml.
   ```
8. Exits with code **2** (quarantine infrastructure error).

---

### Scenario 132: test command timeout with no partial XML exits 2 with crash-style diagnostic [M10]

**Risk:** When a test command times out before producing any XML output, quarantine
silently exits 0 or gives no information about what happened, leaving the developer
with no diagnosis.

**Given** `.quarantine/config.yml` with a `backend` suite with `timeout: 30s`

**And** the `bundle exec rspec` command hangs immediately without writing any
`rspec.xml` (e.g., fails at startup, waiting on a test fixture that never loads)

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Sends SIGTERM after 30 seconds, then SIGKILL after 5 seconds.
2. Finds no `rspec.xml` matching the `junitxml` glob.
3. Prints to stderr:
   ```
   Error [timeout]: test command timed out after 30s and produced no JUnit XML at 'rspec.xml'.
   Check that your test runner can start successfully outside of quarantine.
   ```
4. Exits with code **2**.

No state update. No GitHub Issue. No PR comment.

---

### Scenario 133: rerun timeout kills hanging rerun and classifies test as unresolved [M10]

**Risk:** A single hanging rerun command blocks the entire CI job indefinitely,
even though other failed tests could be retried and classified in the meantime.

**Given** `.quarantine/config.yml` with a `backend` suite:
```yaml
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
    rerun_timeout: 30s
```

**And** two tests fail the initial run:
- `User::validates email` — successfully reruns in 2 seconds, passes → **flaky**
- `Order::ships on time` — when rerun, hangs indefinitely (blocking on an
  external service that isn't available)

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Successfully reruns `User::validates email` in 2 seconds, classifies as flaky.
2. Starts the rerun for `Order::ships on time`.
3. After **30 seconds** (the configured `rerun_timeout`), sends SIGKILL to the
   hanging rerun process.
4. Classifies `Order::ships on time` as **unresolved** with
   `error: "rerun timed out after 30s"` and continues — does NOT abort the run.
5. Prints:
   ```
   Error [rerun]: rerun timed out after 30s for 'ships on time'
   1 test could not be retried — rerun command timed out.
   ```
6. Exits with code **2** (unresolved infrastructure error; no genuine failures).

Results JSON includes both the flaky classification for `validates email` and
the unresolved classification for `ships on time`.

---

### Scenario 134: --timeout CLI flag overrides suite config timeout for that invocation [M10]

**Risk:** When a CI job needs a temporary longer timeout (e.g., the test
environment is slow on a given day), the user must edit `.quarantine/config.yml`
and commit a change rather than passing a flag — making one-off overrides
unnecessarily disruptive.

**Given** `.quarantine/config.yml` with a `backend` suite configured with
`timeout: 10m`

**When** the developer runs `quarantine run backend --timeout 30m`

**Then** quarantine uses a **30-minute** timeout for this invocation, overriding
the `10m` value from config. The config file is not modified.

The override applies only to this invocation. The next `quarantine run backend`
without `--timeout` uses the `10m` value from config.
