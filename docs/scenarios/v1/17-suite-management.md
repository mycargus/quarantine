# Suite Management

### Scenario 126: quarantine suite list prints all configured suites [M10]

**Risk:** `quarantine suite list` omits suites or prints incorrect command/junitxml
values, leading a developer to misconfigure CI by referencing the wrong suite names
or paths.

**Given** `.quarantine/config.yml` with three configured suites:
```yaml
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
  - name: e2e
    command: ["npx", "playwright", "test"]
    junitxml: "test-results/junit.xml"
    rerun_command: ["npx", "playwright", "test", "{file}"]
    retries: 2
```

**When** the developer runs `quarantine suite list`

**Then** the CLI prints:
```
SUITE      COMMAND                         JUNITXML
backend    bundle exec rspec               rspec.xml
frontend   npx jest --ci                   junit.xml
e2e        npx playwright test             test-results/junit.xml
```
Exits with code 0.

---

### Scenario 127: quarantine suite remove asks for confirmation and preserves state file [M10]

**Risk:** `quarantine suite remove` silently deletes the suite's state file on the
state branch, losing quarantine history, or removes the suite without warning the
user about the impact on existing CI workflows.

**Given** `.quarantine/config.yml` with `backend` and `frontend` suites

**And** `.quarantine/backend/state.json` exists on the `quarantine/state` branch
with 3 quarantined tests

**And** open GitHub Issues with labels `quarantine:backend:<hash>` for each
quarantined test

**When** the developer runs `quarantine suite remove backend`

**Then** the CLI prints the ramifications and asks for confirmation:
```
Removing suite 'backend':
  - The 'backend' entry will be removed from .quarantine/config.yml
  - The state file (.quarantine/backend/state.json) on the quarantine/state
    branch will NOT be deleted — quarantined tests remain quarantined
  - GitHub issues for this suite's flaky tests will remain open but will no
    longer be updated by quarantine
  - If CI still runs `quarantine run backend`, it will error because the suite
    no longer exists in config — update your CI workflow first

Are you sure? [y/N]
```

**And when** the developer types `y` and presses enter

**Then** the CLI:
1. Removes the `backend` entry from `.quarantine/config.yml` while preserving
   the `frontend` entry and all shared settings.
2. Does **NOT** delete `.quarantine/backend/state.json` on the state branch.
3. Does **NOT** close or modify any GitHub Issues associated with `backend`.
4. Prints: `Suite 'backend' removed from .quarantine/config.yml.`
5. Exits with code 0.

**And when** the developer instead types `n` (or presses enter, accepting
the `[N]` default)

**Then** the CLI prints: `Aborted. No changes made.` and exits with code 0.
The config file is **unchanged**.
