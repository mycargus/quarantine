# Initialization

### Scenario 1: First-time setup with Jest [M1]

**Given** a developer has a project with a Jest test suite and a GitHub Actions
CI pipeline, and Quarantine CLI is installed but not yet initialized

**When** the developer runs `quarantine init` from the repo root

**Then** the CLI interactively prompts for:
1. Framework: `Which test framework? [rspec/jest/vitest]` — developer enters
   `jest`
2. Retries: `How many retries for failing tests? [3]` — developer presses
   enter (accepts default)
3. JUnit XML path: `Path/glob for JUnit XML output? [junit.xml]` — developer
   presses enter (accepts default)

The CLI then:
- Creates `quarantine.yml` in the current directory:
  ```yaml
  version: 1
  framework: jest
  ```
  (Fields matching defaults are omitted except `framework`, which is always
  written.)
- Validates the GitHub token (`QUARANTINE_GITHUB_TOKEN`, falls back to
  `GITHUB_TOKEN`). Makes an authenticated API call to verify the token is valid.
- Auto-detects `github.owner` and `github.repo` from the `origin` git remote.
- Verifies the token has sufficient permissions by reading repository metadata
  via `GET /repos/{owner}/{repo}`.
- Creates the `quarantine/state` branch by reading the default branch HEAD SHA
  via `GET /repos/{owner}/{repo}/git/ref/heads/{default_branch}`, then creating
  the branch via `POST /repos/{owner}/{repo}/git/refs`.
- Writes an empty `quarantine.json` to the new branch via
  `PUT /repos/{owner}/{repo}/contents/quarantine.json`:
  ```json
  {
    "version": 1,
    "updated_at": "2026-03-18T12:00:00Z",
    "tests": {}
  }
  ```
- Prints recommended `jest-junit` configuration:
  ```
  Recommended jest-junit configuration (in jest.config.js or package.json):

    "jest-junit": {
      "classNameTemplate": "{classname}",
      "titleTemplate": "{title}",
      "ancestorSeparator": " > ",
      "addFileAttribute": "true"
    }

  This produces well-structured JUnit XML for quarantine's test identification.
  ```
- Prints a summary:
  ```
  Quarantine initialized successfully.

    Config:     quarantine.yml (created)
    Framework:  jest
    Retries:    3
    JUnit XML:  junit.xml
    Branch:     quarantine/state (created)

  Next steps:
    1. Add quarantine to your CI workflow:

       - name: Run tests
         run: quarantine run -- jest --ci --reporters=default --reporters=jest-junit
         env:
           QUARANTINE_GITHUB_TOKEN: ${{ secrets.QUARANTINE_GITHUB_TOKEN }}

       - name: Upload quarantine results
         if: always()
         uses: actions/upload-artifact@v4
         with:
           name: quarantine-results-${{ github.run_id }}
           path: .quarantine/results.json

    2. Run `quarantine doctor` to verify your configuration.
  ```
- Exits with code 0.

---

### Scenario 2: quarantine init with RSpec [M1]

**Given** a developer has a project with an RSpec test suite

**When** the developer runs `quarantine init` and selects `rspec` as the
framework, accepting defaults for retries (3) and JUnit XML path (`rspec.xml`)

**Then** the CLI creates `quarantine.yml` with `framework: rspec`, validates the
token, creates the branch, and prints the summary. No framework-specific
recommendation is printed (unlike Jest's `jest-junit` guidance). The JUnit XML
default is `rspec.xml`. The workflow snippet uses:
```
run: quarantine run -- rspec --format RspecJunitFormatter --out rspec.xml
```
Exits with code 0.

---

### Scenario 3: quarantine init with Vitest [M1]

**Given** a developer has a project with a Vitest test suite

**When** the developer runs `quarantine init` and selects `vitest` as the
framework, accepting defaults for retries (3) and JUnit XML path
(`junit-report.xml`)

**Then** the CLI creates `quarantine.yml` with `framework: vitest`, validates
the token, creates the branch, and prints the summary. The workflow snippet
uses:
```
run: quarantine run -- vitest run --reporter=junit
```
Exits with code 0.

---

### Scenario 4: quarantine init when quarantine.yml already exists [M1]

**Given** a developer has already run `quarantine init` and a `quarantine.yml`
file exists in the repo root

**When** the developer runs `quarantine init` again

**Then** the CLI detects the existing `quarantine.yml` and prompts:
`quarantine.yml already exists. Overwrite? [y/N]`

If the developer enters `y`: the CLI proceeds with the interactive prompts and
overwrites the existing file.

If the developer enters `n` or presses enter (default): the CLI prints
`Aborted. Existing quarantine.yml preserved.` and exits with code 0.

---

### Scenario 5: quarantine init when quarantine/state branch already exists [M1]

**Given** a developer has already run `quarantine init` and the
`quarantine/state` branch exists in the GitHub repository with a
`quarantine.json` file

**When** the developer runs `quarantine init` again (e.g., after recreating
`quarantine.yml`)

**Then** the CLI detects that the branch already exists (via
`GET /repos/{owner}/{repo}/git/ref/heads/quarantine/state` returning 200),
prints a warning: `Branch 'quarantine/state' already exists. Skipping branch
creation.`, and does NOT overwrite the existing `quarantine.json`. The rest of
the init flow (config creation, token validation, summary) proceeds normally.
Exits with code 0.

---

### Scenario 6: quarantine init with no GitHub token [M1]

**Given** a developer has Quarantine CLI installed, but neither
`QUARANTINE_GITHUB_TOKEN` nor `GITHUB_TOKEN` is set in the environment

**When** the developer runs `quarantine init`

**Then** the CLI completes the interactive prompts and creates `quarantine.yml`
locally, but when it attempts to validate the GitHub token, it prints:
```
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.

  export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here

Required token scope: repo (read/write contents, create issues, post PR comments)
```
Exits with code 2. The `quarantine.yml` file has already been created (partial
init — config created, GitHub setup not completed).

---

### Scenario 7: quarantine init with insufficient token permissions [M1]

**Given** a developer has `GITHUB_TOKEN` set but the token lacks the `repo`
scope (e.g., it has only `read:org` scope)

**When** the developer runs `quarantine init` and the CLI attempts to verify
repository access via `GET /repos/{owner}/{repo}`, receiving a 403 Forbidden

**Then** the CLI prints:
```
Error: GitHub token lacks permission to access repository 'my-org/my-project'.
Required scope: repo. Check your token permissions at https://github.com/settings/tokens
```
Exits with code 2. Init failures are always fatal with diagnostics — no
degraded mode (per milestones.md).

---

### Scenario 8: quarantine init when not a git repository [M1]

**Given** a developer runs `quarantine init` in a directory that is not a git
repository (no `.git` directory)

**When** the CLI attempts to auto-detect `github.owner` and `github.repo` from
the git remote

**Then** the CLI prints:
```
Error: Not a git repository. Run 'quarantine init' from the root of a git repository.
```
Exits with code 2.

---

### Scenario 9: quarantine init with non-GitHub remote [M1]

**Given** a developer's git repository has its `origin` remote set to a
non-GitHub URL (e.g., `https://gitlab.com/my-org/my-project.git`)

**When** the developer runs `quarantine init` and the CLI attempts to parse the
`origin` remote URL

**Then** the CLI prints:
```
Error: Remote 'origin' is not a GitHub URL: https://gitlab.com/my-org/my-project.git
Quarantine v1 supports GitHub repositories only. Set github.owner and github.repo
in quarantine.yml manually if using a non-standard remote.
```
Exits with code 2.

---

### Scenario 10: quarantine init with invalid framework input [M1]

**Given** a developer runs `quarantine init`

**When** the CLI prompts `Which test framework? [rspec/jest/vitest]` and the
developer enters `pytest`

**Then** the CLI prints: `Invalid framework 'pytest'. Supported: rspec, jest,
vitest.` and re-prompts: `Which test framework? [rspec/jest/vitest]`. The prompt
repeats until a valid value is entered.

---

### Scenario 11: quarantine init with GitHub API unreachable [M1]

**Given** a developer has a valid GitHub token but the GitHub API is unreachable
(network failure, DNS resolution failure, or GitHub outage)

**When** the developer runs `quarantine init` and the CLI attempts to verify
repository access

**Then** the CLI retries once after 2 seconds. If the retry also fails, prints:
```
Error: Unable to reach GitHub API: connection timed out.
Check your network connection and try again.
```
Exits with code 2. Init does NOT use degraded mode — failures are fatal with
diagnostics.

---

### Scenario 12: quarantine run without prior init [M2]

**Given** a developer has the CLI installed but has not run `quarantine init`
(no `quarantine.yml` in the repo root, no `quarantine/state` branch)

**When** the developer runs `quarantine run -- jest --ci`

**Then** the CLI checks for `quarantine.yml` and the `quarantine/state` branch.
Finding both absent, it prints:
```
Quarantine is not initialized for this repository. Run 'quarantine init' first.
```
Exits with code 2. The test command is NOT executed.

---
