# Configuration Validation

### Scenario 13: quarantine doctor — valid configuration [M1]

**Risk:** The doctor command reports a configuration as valid when it contains errors, giving the user false confidence that CI will work correctly.

**Given** a `quarantine.yml` file exists in the repo root with the following
content:
```yaml
version: 1
framework: jest
retries: 3
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
```

**When** the developer runs `quarantine doctor` from the repo root

**Then** the CLI reads `quarantine.yml`, validates all fields against the schema
(including `version`, `framework`, `issue_tracker`, `labels`, and
`notifications`), resolves auto-detected values (`github.owner` and
`github.repo` from the `origin` git remote, framework-specific `junitxml`
default), and prints the resolved configuration:
```
quarantine.yml is valid.

  Resolved configuration:
    version:         1
    framework:       jest
    retries:         3
    junitxml:        junit.xml (default)
    github.owner:    my-org (auto-detected)
    github.repo:     my-project (auto-detected)
    issue_tracker:   github
    labels:          [quarantine]
    notifications:   github_pr_comment: true
    storage.branch:  quarantine/state (default)
```
If `QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN` is not set, appends a warning:
`Warning: No GitHub token found in environment. 'quarantine run' will fail
unless QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN is set.`

Exits with code 0.

---

### Scenario 14: quarantine doctor — missing config file [M1]

**Risk:** A missing config file produces an opaque error instead of directing the user to run `quarantine init`.

**Given** no `quarantine.yml` file exists in the current directory

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: quarantine.yml not found in the current directory.
Run 'quarantine init' to create one.
```
Exits with code 2.

---

### Scenario 15: quarantine doctor — invalid field values [M1]

**Risk:** Invalid configuration values (e.g., negative retries) are not caught until `quarantine run` fails in CI.

**Given** a `quarantine.yml` file exists with `retries: -1`

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Invalid retries value: -1. Must be between 1 and 10.
```
Exits with code 2. Each invalid field produces a separate error line. All errors
are reported (not short-circuited on first error).

---

### Scenario 16: quarantine doctor — forward-compatible config value [M1]

**Risk:** Users set `issue_tracker: jira` believing it works, but no tickets are created -- the feature silently does nothing (ADR-021).

**Given** a `quarantine.yml` file exists with:
```yaml
version: 1
framework: jest
retries: 3
issue_tracker: jira
```

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Unsupported issue_tracker 'jira'. This version supports: github.
Jira support is planned for a future release.
```
Exits with code 2. Forward-compatible fields (`issue_tracker`, `labels`,
`notifications`) have restricted allowed values in v1 that expand in v2 without
schema changes (ADR-021).

---

### Scenario 17: quarantine doctor — unknown fields [M1]

**Risk:** A typo in a field name goes unnoticed, or a user configures `notifications.slack: true` expecting Slack alerts that never fire.

**Given** a `quarantine.yml` file exists with:
```yaml
version: 1
framework: jest
custom_field: something
notifications:
  github_pr_comment: true
  slack: true
```

**When** the developer runs `quarantine doctor`

**Then** the CLI reports:
- Warning: `Unknown field 'custom_field' in quarantine.yml will be ignored.`
  (unknown top-level keys produce warnings, not errors)
- Error: `Unknown notification channel 'slack'. This version supports:
  github_pr_comment. Slack and email notifications are planned for a future
  release.` (unknown keys under `notifications` are errors because they may
  indicate a user expecting functionality that doesn't exist)

Exits with code 2 (errors present). Warnings alone would exit 0.

---

### Scenario 18: quarantine doctor — custom config path [M1]

**Risk:** The `--config` flag is ignored, forcing all projects into the default `quarantine.yml` path and breaking non-standard project layouts.

**Given** a valid configuration file exists at `config/quarantine.yml` (not the
default location)

**When** the developer runs `quarantine doctor --config config/quarantine.yml`

**Then** the CLI reads from the specified path, validates, and prints the
resolved configuration. Exits with code 0. The `--config` flag works the same
way for `quarantine run`.

---
