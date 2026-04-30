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

---

### Scenario 185: quarantine init — phase 1 with single github.com remote (one hint) [M20]

**Risk:** The one-hint case (origin is github.com, no upstream) is the most
common developer path. If the hint logic silently produces zero hints or two
hints when there is exactly one github.com remote, the user gets incorrect
guidance at first setup.

**Given** a developer is working in a git repository with no
`.quarantine/config.yml`
**And** `git remote -v` returns exactly one github.com remote:
```
origin    https://github.com/my-org/my-project.git (fetch)
origin    https://github.com/my-org/my-project.git (push)
```
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Writes `.quarantine/config.yml` with empty `github.owner` and `github.repo`
   and exactly one hint comment for the detected remote:
   ```yaml
   github:
     owner: # set to your GitHub organization or user
     repo:  # set to your GitHub repository name
     # detected GitHub remotes (review before using):
     #   origin -> my-org/my-project
   ```
2. Does NOT pre-fill `github.owner` or `github.repo`.
3. Exits with code 2 and the same hand-edit instructions as Scenario 174.

---

### Scenario 186: quarantine init — phase 1 with no GitHub token set [M20]

**Risk:** If phase 1 requires a token (as init previously did), a developer
running `quarantine init` for the first time in a Jenkins environment — before
they have set up `QUARANTINE_GITHUB_TOKEN` — gets a "no token" error and no
partial config is written. The user has two problems to fix but gets no config
scaffold to work from.

**Given** a developer is working in a git repository with no
`.quarantine/config.yml`
**And** neither `QUARANTINE_GITHUB_TOKEN` nor `GITHUB_TOKEN` is set

**When** the developer runs `quarantine init`

**Then** the CLI:
1. Detects test frameworks and scans `git remote -v` for github.com hints
   (best-effort, same as Scenario 174).
2. Writes `.quarantine/config.yml` with the empty `github` block and any
   detected hints (same as Scenario 174) — the missing token does NOT prevent
   the partial config from being written.
3. Prints the hand-edit instructions (same as Scenario 174) **plus** a note
   that a GitHub token will be required on the next run:
   ```
   Error [config]: github.owner and github.repo are required.
   .quarantine/config.yml has been created. Edit it to set:

     github:
       owner: <your-github-org-or-user>
       repo:  <your-github-repo-name>

   Then re-run 'quarantine init' to complete setup.

   Note: You will also need a GitHub token. Set QUARANTINE_GITHUB_TOKEN or
   GITHUB_TOKEN before re-running init (required scope: repo).
   ```
4. Exits with code 2.

Phase 1 never talks to the GitHub API, so no token is needed to write the
partial config. Token validation is deferred to phase 2.

---

### Scenario 187: quarantine init — re-run on partial config with owner/repo still empty [M20]

**Risk:** A developer who ran `quarantine init` (phase 1) but has not yet
hand-edited the config re-runs init accidentally. Init should not overwrite the
partial config (which may have been partially edited), and should give the same
clear hand-edit instructions rather than silently no-op or create a confusing
error.

**Given** `.quarantine/config.yml` exists from a prior `quarantine init` but
`github.owner` and `github.repo` are still empty (the user has not yet
hand-edited the file)
**And** `QUARANTINE_GITHUB_TOKEN` is set

**When** the developer runs `quarantine init` again

**Then** the CLI:
1. Reads the existing `.quarantine/config.yml`.
2. Detects that `github.owner` and `github.repo` are missing or empty.
3. Does NOT overwrite the existing config file.
4. Prints the same hand-edit instructions as Scenario 174:
   ```
   Error [config]: github.owner and github.repo are required.
   .quarantine/config.yml has been created. Edit it to set:

     github:
       owner: <your-github-org-or-user>
       repo:  <your-github-repo-name>

   Then re-run 'quarantine init' to complete setup.
   ```
5. Exits with code 2.

The existing config is preserved intact. Any partial edits the user has made
are not lost.

---

### Scenario 188: quarantine run — creates state branch on first invocation when missing [M20]

**Risk:** A developer who has completed phase 1 of init (config written,
`github.owner`/`github.repo` set) but skipped phase 2 cannot run their first
CI build — `quarantine run` fails with "Quarantine is not initialized" and
there is no obvious unblocking step short of running `quarantine init` again.
This is especially painful in Jenkins environments where the developer may not
have repo write access on their laptop.

**Given** `.quarantine/config.yml` has valid `github.owner: my-org` and
`github.repo: my-project`
**And** `QUARANTINE_GITHUB_TOKEN` is set
**And** the `quarantine/state` branch does NOT exist on `my-org/my-project`

**When** Jenkins runs `quarantine run backend`

**Then** the CLI:
1. Reads owner/repo from config, detects the missing branch.
2. Fetches the default branch HEAD SHA via
   `GET /repos/my-org/my-project/git/ref/heads/{default_branch}`.
3. Creates the `quarantine/state` branch via
   `POST /repos/my-org/my-project/git/refs`.
4. Prints to stderr: `[quarantine] State branch 'quarantine/state' created.`
5. Continues the run normally — loads empty quarantine state, executes the
   suite, parses JUnit XML, writes state, creates issues, writes results.json.
6. Exits with the appropriate code (0 = pass, 1 = genuine failures,
   2 = quarantine error).

**Concurrent-run robustness:** if two CI shards run simultaneously before the
branch exists, the second attempt receives a 422 from GitHub. `run` treats 422
as "branch already exists" and continues normally. Neither shard is blocked.

**Branch creation failure (degraded mode):** if branch creation fails (403
token lacks write scope, 5xx, network error), `run` emits
`[quarantine] WARNING: Cannot create state branch 'quarantine/state': <reason>. Continuing in degraded mode.`
and proceeds without quarantine awareness. The build is never broken by branch
creation failure.
