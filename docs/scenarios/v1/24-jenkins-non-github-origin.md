# Jenkins / Non-GitHub-Origin Support

Scenarios covering ADR-037: `.quarantine/config.yml` is the sole source of
truth for `github.owner` and `github.repo`. The working tree's git origin is
no longer required to be a github.com URL. `quarantine init` becomes a
two-phase flow.

> **Supersedes Scenario 9** (`docs/scenarios/v1/01-initialization.md`).
> Scenario 9 specifies that `quarantine init` exits 2 with a "remote is not a
> GitHub URL" error. ADR-037 replaces that with a config-driven flow:
> init never reads the origin URL host. Scenario 9 should be treated as
> superseded once M20 is implemented.

---

### Scenario 174: quarantine init — first run writes partial config, exits 2 [M20]

**Risk:** Init silently produces a config that appears complete when the user
has provided no GitHub target, leading the user to assume CI will work and
hit confusing runtime failures.

**Given** a developer is working in a git repository with no
`.quarantine/config.yml`
**And** the repository's `origin` may be any host (github.com, Gerrit,
GitLab — irrelevant to init's behavior)
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Detects test frameworks and prepares suite stubs (existing M9 behavior).
2. Best-effort runs `git remote -v` to find github.com candidates (see
   Scenario 184 for the fork case).
3. Writes `.quarantine/config.yml` containing all detected suite entries
   plus a `github` block with empty values. If no remotes are github.com
   URLs, no hint comments are emitted:
   ```yaml
   github:
     owner: # set to your GitHub organization or user
     repo:  # set to your GitHub repository name
   ```
4. Does NOT attempt to create the `quarantine/state` branch (owner/repo
   unknown).
5. Prints:
   ```
   Error [config]: github.owner and github.repo are required.
   .quarantine/config.yml has been created. Edit it to set:

     github:
       owner: <your-github-org-or-user>
       repo:  <your-github-repo-name>

   Then re-run 'quarantine init' to complete setup.
   ```
6. Exits with code 2.

---

### Scenario 175: quarantine init — re-run after hand-edit completes setup [M20]

**Risk:** A re-run of init after the user adds owner/repo to config is treated
as a duplicate-init error rather than the expected "complete setup" path,
leaving the user stuck.

**Given** a `.quarantine/config.yml` exists from a prior `quarantine init`
**And** the user has hand-edited the config to add valid `github.owner` and
`github.repo` values
**And** `QUARANTINE_GITHUB_TOKEN` is set with a valid PAT
**And** the `quarantine/state` branch does not yet exist

**When** the developer runs `quarantine init` again

**Then** the CLI:
1. Reads the existing `.quarantine/config.yml` and validates that
   `github.owner` and `github.repo` are present and non-empty.
2. Validates the GitHub token via `GET /repos/{owner}/{repo}`.
3. Creates the `quarantine/state` branch with an initial empty
   `.quarantine/<suite>/state.json` for each configured suite.
4. Prints a setup-complete summary including:
   ```
   github.owner:  <owner> (from config)
   github.repo:   <repo> (from config)
   Branch:        quarantine/state (created)
   ```
5. Exits with code 0.

If the `quarantine/state` branch already exists (e.g., the user ran init
multiple times), step 3 is skipped per NFR-2.2.4 idempotency; init still
exits 0.

---

### Scenario 176: quarantine run — Jenkins with config-provided owner/repo [M20]

**Risk:** `quarantine run` fails to resolve owner/repo when the working tree's
origin is a Gerrit URL, even when the user has correctly set
`github.owner` and `github.repo` in `.quarantine/config.yml`.

**Given** `.quarantine/config.yml` contains:
```yaml
github:
  owner: my-org
  repo:  my-project
test_suites:
  - name: backend
    command: bundle exec rspec
    junitxml: rspec.xml
    rerun_command: bundle exec rspec {file} -e "{name}"
```
**And** the git repository's `origin` remote points to a Gerrit URL
**And** `QUARANTINE_GITHUB_TOKEN` is set to a valid PAT
**And** the `quarantine/state` branch exists on `github.com/my-org/my-project`
**And** no `GITHUB_EVENT_PATH` or `GITHUB_ACTIONS` env vars are set

**When** Jenkins runs `quarantine run backend`

**Then** the CLI:
1. Reads `github.owner: my-org` and `github.repo: my-project` from config.
   Does NOT inspect the git origin URL.
2. Reads quarantine state from `github.com/my-org/my-project` on the state
   branch via GitHub Contents API.
3. Executes the `backend` suite command, parses JUnit XML, performs flaky
   detection and retries.
4. Writes per-suite state updates via CAS.
5. Creates GitHub Issues for newly detected flaky tests on `my-org/my-project`.
6. Skips PR comment posting (no PR number — no `GITHUB_EVENT_PATH`,
   no `--pr` flag).
7. Writes `.quarantine/backend/results.json` to disk.
8. Exits with the appropriate code (0 = pass, 1 = genuine failures, 2 = quarantine error).

---

### Scenario 177: quarantine run — config missing github.owner/github.repo [M20]

**Risk:** `quarantine run` produces a cryptic 404 from a GitHub API call deep
in the run instead of failing fast at startup with a clear config error.

**Given** `.quarantine/config.yml` exists but `github.owner` or `github.repo`
is missing or empty
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine run backend`

**Then** the CLI:
1. Detects the missing field at config-load time, before executing the test
   command or making any GitHub API call.
2. Prints:
   ```
   Error [config]: github.owner and github.repo are required in .quarantine/config.yml.
   Run 'quarantine init' or edit the config to add them.
   ```
3. Exits with code 2. The test command is NOT executed.

---

### Scenario 178: quarantine run — Jenkins with no GitHub token [M20]

**Risk:** A Jenkins pipeline that omits `QUARANTINE_GITHUB_TOKEN` silently
proceeds until the first GitHub API call, producing a cryptic 401 error deep
in a CI run instead of a clear startup diagnostic.

**Given** `.quarantine/config.yml` exists with valid `github.owner` and
`github.repo` set
**And** neither `QUARANTINE_GITHUB_TOKEN` nor `GITHUB_TOKEN` is set in the
environment (Jenkins does not auto-provision `GITHUB_TOKEN`)

**When** Jenkins runs `quarantine run backend`

**Then** the CLI prints:
```
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.
```
and exits with code 2. The test command is NOT executed.

---

### Scenario 179: quarantine doctor — Gerrit origin, target reachable [M20]

**Risk:** `quarantine doctor` reports an error about the origin URL being
non-GitHub even when the config fully specifies a valid GitHub target,
blocking the user from verifying their setup before running in Jenkins.

**Given** a developer's git repository has `origin` pointing to a Gerrit URL
**And** `.quarantine/config.yml` contains valid `github.owner: my-org` and
`github.repo: my-project`
**And** `QUARANTINE_GITHUB_TOKEN` is set
**And** `GET /repos/my-org/my-project` returns 200

**When** the developer runs `quarantine doctor`

**Then** the CLI:
1. Reads owner/repo from config. Does NOT inspect the origin URL.
2. Calls `GET /repos/my-org/my-project` once to verify reachability.
3. On 200, prints a success summary including:
   ```
   github.owner:  my-org (from config)
   github.repo:   my-project (from config)
   token:         authenticated
   target:        reachable
   ```
4. Does NOT inspect `permissions` or `has_issues` in the response — token
   scope checks and feature-flag checks are not doctor's responsibility.
5. Exits with code 0. No warning about the origin URL is emitted.

---

### Scenario 180: quarantine doctor — config missing owner/repo [M20]

**Risk:** `quarantine doctor` reports success when `github.owner` and
`github.repo` are absent from config, giving false confidence before a run
that will immediately fail.

**Given** `.quarantine/config.yml` exists but does NOT contain `github.owner`
or `github.repo` (e.g., the user ran `quarantine init` once but never
hand-edited the config)
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine doctor`

**Then** the CLI:
1. Detects the missing field at config-load time.
2. Does NOT make any GitHub API call.
3. Prints:
   ```
   Error [config]: github.owner and github.repo are required in .quarantine/config.yml.
   Run 'quarantine init' or edit the config to add them.
   ```
4. Exits with code 2.

---

### Scenario 181: quarantine doctor — target unreachable (4xx from GitHub) [M20]

**Risk:** A typo in `github.owner` or `github.repo`, or a token without
access to the target repo, is not caught by doctor — surfacing only at run
time deep in a CI log.

**Given** `.quarantine/config.yml` contains `github.owner: my-org` and
`github.repo: my-project`
**And** `QUARANTINE_GITHUB_TOKEN` is set
**And** `GET /repos/my-org/my-project` returns 404 (either the repo does not
exist or the token cannot see it — GitHub does not distinguish)

**When** the developer runs `quarantine doctor`

**Then** the CLI:
1. Reads owner/repo from config and calls `GET /repos/my-org/my-project`.
2. Receives a 404 response.
3. Prints (using GitHub's response body where useful):
   ```
   Error: Cannot reach my-org/my-project on GitHub (404).
   Either the repository does not exist or the configured token does not
   have access to it. Verify github.owner and github.repo in
   .quarantine/config.yml, and confirm QUARANTINE_GITHUB_TOKEN has access
   to the repository.
   ```
4. Exits with code 2.

The same flow applies for 401 (token invalid or expired) and 403 (token
present but cannot read this repo) — doctor surfaces the GitHub status code
and message; it does not introspect token scopes.

---

### Scenario 182: quarantine run — Jenkins with explicit --pr flag [M20]

**Risk:** PR comment posting is unavailable in Jenkins even when a GitHub PR
exists for the change (mirrored from Gerrit), depriving the team of PR-level
flaky test visibility during the migration period.

**Given** a Jenkins pipeline is running against a change that has a
corresponding GitHub PR `#42` on the GitHub mirror
**And** `.quarantine/config.yml` has valid `github.owner` and `github.repo`
**And** `QUARANTINE_GITHUB_TOKEN` is set
**And** `GITHUB_EVENT_PATH` is not set
**And** the `backend` suite detects a flaky test during this run

**When** Jenkins runs `quarantine run backend --pr 42`

**Then** the CLI:
1. Reads the PR number from the `--pr 42` flag (no `GITHUB_EVENT_PATH` needed).
2. Completes flaky detection, state update, and issue creation normally.
3. Posts a PR comment to PR #42 on `github.com/{owner}/{repo}` with the
   quarantine summary.
4. Exits with code 0.

---

### Scenario 183: quarantine run — Jenkins, no PR number available [M20]

**Risk:** Missing PR context causes `quarantine run` to exit 2, breaking the
Jenkins build when PR comment posting is not possible.

**Given** a Jenkins pipeline is running a branch build (not associated with a
GitHub PR)
**And** `.quarantine/config.yml` has valid `github.owner` and `github.repo`
**And** `QUARANTINE_GITHUB_TOKEN` is set
**And** neither `GITHUB_EVENT_PATH` nor `--pr` provides a PR number

**When** Jenkins runs `quarantine run backend`

**Then** the CLI:
1. Completes flaky detection, state update, and issue creation normally.
2. Silently skips PR comment posting (no PR number available; this is not an
   error condition).
3. Writes `.quarantine/backend/results.json` to disk.
4. Exits with the appropriate code (0 = pass, 1 = genuine failures, 2 = quarantine error).

No warning or error is emitted about the missing PR number.

---

### Scenario 184: quarantine init — phase 1 hints surface fork and upstream remotes [M20]

**Risk:** A developer working from a fork clone (where `origin` points to
their personal fork and `upstream` points to the team repo) is silently
funneled toward the fork by an auto-detection that picks the first github.com
remote, causing quarantine state and issues to land in the wrong repository.

**Given** a developer is working in a git repository with no
`.quarantine/config.yml`
**And** `git remote -v` returns:
```
origin    git@github.com:mhargiss/quarantine.git (fetch)
origin    git@github.com:mhargiss/quarantine.git (push)
upstream  https://github.com/mycargus/quarantine.git (fetch)
upstream  https://github.com/mycargus/quarantine.git (push)
```
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Writes `.quarantine/config.yml` with empty `github.owner` and
   `github.repo` fields, and appends both detected github.com remotes as
   advisory YAML comments under the `github` block:
   ```yaml
   github:
     owner: # set to your GitHub organization or user
     repo:  # set to your GitHub repository name
     # detected GitHub remotes (review before using):
     #   origin   -> mhargiss/quarantine
     #   upstream -> mycargus/quarantine
   ```
2. Does NOT pre-fill `github.owner` or `github.repo` with either candidate.
3. Exits with code 2 and the same hand-edit instructions as Scenario 174.

A subsequent `quarantine init` re-run reads only the explicit
`github.owner` / `github.repo` fields (Scenario 175); the comment block is
ignored. If the user fills in `mhargiss/quarantine`, quarantine writes to
the fork; if they fill in `mycargus/quarantine`, it writes to upstream. The
choice is explicit and version-controlled.

If `git remote -v` fails (not a git repo, command unavailable, etc.) or no
remotes are github.com URLs, init proceeds with the empty `github` block
and no hint comments — Scenario 174's behavior.
