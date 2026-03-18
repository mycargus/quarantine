# Configuration Edge Cases

### Scenario 64: Config resolution order [M2]

**Given** `quarantine.yml` has `retries: 5` and `junitxml: "custom/*.xml"`.
The `origin` git remote points to `github.com/my-org/my-project`.

**When** the developer runs
`quarantine run --retries 2 --junitxml "override.xml" -- jest --ci`

**Then** the CLI applies config resolution in priority order:
1. CLI flags: `retries: 2`, `junitxml: "override.xml"` (highest priority)
2. Config file: `retries: 5`, `junitxml: "custom/*.xml"` (overridden by flags)
3. Auto-detected: `github.owner: my-org`, `github.repo: my-project`
4. Defaults: `storage.branch: quarantine/state`, etc. (lowest priority)

Result: retries=2, junitxml="override.xml", github.owner="my-org".

---

### Scenario 65: Minimal valid config [M1]

**Given** `quarantine.yml` contains only:
```yaml
version: 1
framework: jest
```

**When** the developer runs `quarantine doctor`

**Then** the CLI applies all defaults:
- `retries: 3`
- `junitxml: junit.xml` (Jest framework default)
- `issue_tracker: github`
- `labels: [quarantine]`
- `notifications.github_pr_comment: true`
- `storage.branch: quarantine/state`
- `github.owner` and `github.repo` auto-detected from git remote

Prints the resolved configuration. Exits with code 0.

---

### Scenario 66: Unsupported config version [M1]

**Given** `quarantine.yml` contains `version: 2`

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Unsupported config version: 2. This version of the CLI supports version 1.
```
Exits with code 2.
