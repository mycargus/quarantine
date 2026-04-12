# Multi-Suite Initialization

### Scenario 110: quarantine init detects jest and rspec, pre-fills two suite entries [M9]

**Risk:** `quarantine init` fails to detect multiple test frameworks or writes
a config that is missing required fields, causing the user to start from a blank
template rather than a working starting point.

**Given** a repository with both a `package.json` containing
`"jest": "^29.0.0"` in `devDependencies` and a `Gemfile` containing
`gem 'rspec'`

**And** `QUARANTINE_GITHUB_TOKEN` is set to a valid token with repo access

**And** no `.quarantine/` directory exists and no `quarantine/state` branch
exists

**When** the developer runs `quarantine init` from the repo root

**Then** the CLI:
1. Detects jest from `package.json` and rspec from `Gemfile`.
2. Creates `.quarantine/config.yml` with two pre-filled suite entries:
   ```yaml
   version: 1

   github:
     owner: <detected-owner>
     repo: <detected-repo>

   issue_tracker: github
   labels:
     - quarantine
   notifications:
     github_pr_comment: true
   storage:
     branch: quarantine/state

   test_suites:
     - name: jest
       command: ["npx", "jest", "--ci"]
       junitxml: "junit.xml"
       rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
       retries: 3
     - name: rspec
       command: ["bundle", "exec", "rspec"]
       junitxml: "rspec.xml"
       rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
       retries: 3
   ```
3. Creates `.quarantine/.gitignore` with:
   ```gitignore
   # Ignore all runtime files. Only config.yml is source-controlled.
   *
   !.gitignore
   !config.yml
   ```
4. Creates the `quarantine/state` branch with an initial commit containing
   a `README.md` that explains the branch's purpose.
5. Validates the GitHub token.
6. Prints:
   ```
   Detected test frameworks: jest, rspec
   Pre-filled 2 suite entries in .quarantine/config.yml

   Quarantine initialized.
     Config:   .quarantine/config.yml (created)
     Branch:   quarantine/state (created)

   Next step: review .quarantine/config.yml, adjust suite names and commands,
   then run `quarantine doctor` to validate.
   ```
7. Exits with code 0.

---

### Scenario 111: quarantine init with no framework detected writes commented example suite [M9]

**Risk:** When no framework is detected, `quarantine init` creates a config
with an empty `test_suites` array, causing `quarantine run` to immediately
error with no guidance on how to add a suite.

**Given** a repository with no `package.json` and no `Gemfile`

**And** `QUARANTINE_GITHUB_TOKEN` is set to a valid token

**And** no `.quarantine/` directory exists

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Detects no test frameworks.
2. Creates `.quarantine/config.yml` with a commented example suite entry
   instead of a populated `test_suites` array:
   ```yaml
   version: 1

   github:
     owner: <detected-owner>
     repo: <detected-repo>

   issue_tracker: github
   labels:
     - quarantine
   notifications:
     github_pr_comment: true
   storage:
     branch: quarantine/state

   test_suites:
     # Add your test suites here. Example:
     # - name: unit
     #   command: ["npm", "test"]
     #   junitxml: "junit.xml"
     #   rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
     #   retries: 3
   ```
3. Creates `.quarantine/.gitignore` and the state branch with `README.md`.
4. Prints:
   ```
   No test frameworks detected.

   Quarantine initialized.
     Config:   .quarantine/config.yml (created)
     Branch:   quarantine/state (created)

   Next step: edit .quarantine/config.yml to add your test suites,
   then run `quarantine doctor` to validate.
   ```
5. Exits with code 0.

---

### Scenario 112: quarantine init is idempotent — re-running skips existing artifacts [M9]

**Risk:** Re-running `quarantine init` silently overwrites the user's edited
`.quarantine/config.yml` (containing their customized suite definitions),
destroying their configuration.

**Given** a repository where `quarantine init` was previously run successfully

**And** `.quarantine/config.yml` exists with user-customized suite entries

**And** the `quarantine/state` branch exists with quarantine state

**When** the developer runs `quarantine init` again (e.g., after a new
team member follows the setup guide without checking existing state)

**Then** the CLI checks each artifact independently and skips existing ones:
1. Detects `.quarantine/config.yml` exists → skips without overwriting.
2. Detects `.quarantine/.gitignore` exists → skips.
3. Detects `quarantine/state` branch exists → skips without recreating.
4. Validates the GitHub token.
5. Prints:
   ```
   .quarantine/config.yml already exists — skipping.
   .quarantine/.gitignore already exists — skipping.
   quarantine/state branch already exists — skipping.
   GitHub token validated.

   Quarantine is already initialized. Edit .quarantine/config.yml to add test suites.
   ```
6. Exits with code 0.

The user's `.quarantine/config.yml` is **unchanged**.

---

### Scenario 113: quarantine init recreates missing state branch when config exists [M9]

**Risk:** If the state branch is accidentally deleted, the user has no recovery
path short of deleting the config and re-running init from scratch, losing
their suite definitions.

**Given** `.quarantine/config.yml` exists with valid suite entries

**And** the `quarantine/state` branch has been deleted (e.g., accidentally
by a repository admin)

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Detects `.quarantine/config.yml` exists → skips.
2. Detects `.quarantine/.gitignore` exists → skips.
3. Detects `quarantine/state` branch does NOT exist → recreates it with
   an initial commit containing a `README.md`.
4. Prints:
   ```
   .quarantine/config.yml already exists — skipping.
   .quarantine/.gitignore already exists — skipping.
   quarantine/state branch not found — creating.
   GitHub token validated.

   Quarantine recovered. The state branch has been recreated.
   Previous quarantine state was on the deleted branch and is not recoverable.
   ```
5. Exits with code 0.

The user's `.quarantine/config.yml` is **unchanged**.

---

### Scenario 114: quarantine doctor validates test_suites array and rejects invalid configs [M9]

**Risk:** An invalid config reaches `quarantine run` at CI time and produces a
cryptic error that stops the build. `quarantine doctor` should catch all
structural issues before the user commits the config.

**Given** `.quarantine/config.yml` exists with the following invalid content:
```yaml
version: 1
test_suites:
  - name: "My Backend!"
    command: "bundle exec rspec"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
```

**When** the developer runs `quarantine doctor`

**Then** the CLI reports three separate errors:
1. `suite "My Backend!": name must match [a-z0-9][a-z0-9-]* (max 30 chars)`
   — the name contains uppercase letters, spaces, and a special character.
2. `suite "My Backend!": command must be a YAML array, not a string.
   Use: command: ["bundle", "exec", "rspec"]`
   — `command` is a string, not a YAML sequence.
3. `suite "My Backend!": junitxml is required`
   — the `junitxml` field is missing.

And reports no errors about `rerun_command` (it is valid).

Exits with code 2.

---

### Scenario 115: quarantine doctor warns on detected jest retryTimes but does not error [M9]

**Risk:** A user enables Jest's `retryTimes` without knowing it hides failures
from JUnit XML, silently defeating quarantine's flaky detection. The doctor
command should warn without blocking the CI run.

**Given** `.quarantine/config.yml` exists and is valid

**And** `jest.config.js` in the repo root contains:
```js
module.exports = {
  retryTimes: 2,
  ...
}
```

**When** the developer runs `quarantine doctor`

**Then** the CLI:
1. Validates the config successfully (no errors).
2. Prints the resolved config.
3. Prints a **warning** (not an error):
   ```
   Warning: jest.config.js contains 'retryTimes'. Framework-level retries hide
   failures from JUnit XML, preventing quarantine from detecting flaky tests.
   Remove retryTimes before using quarantine.
   ```
4. Exits with code **0** (not 2 — the warning does not make the config invalid).

The warning does NOT appear during `quarantine run` — only when the user
explicitly runs `quarantine doctor`.

---

### Scenario 116: quarantine doctor does not warn when retryTimes is set to 0 [M9]

**Risk:** A user who disabled Jest retries by setting `retryTimes(0)` gets a
false-positive warning from `quarantine doctor`, causing confusion and eroding
trust in the tool's diagnostics.

**Given** `.quarantine/config.yml` exists and is valid

**And** a test file in the repo contains `jest.retryTimes(0)` (an explicit
no-op disabling retries)

**When** the developer runs `quarantine doctor`

**Then** the CLI:
1. Validates the config successfully.
2. Prints the resolved config.
3. Does **NOT** print any warning about `retryTimes` (the regex
   `/retryTimes\(\s*[1-9]/` does not match `retryTimes(0)`).
4. Exits with code 0.
