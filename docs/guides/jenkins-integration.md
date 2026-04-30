# Jenkins Integration

This guide covers running Quarantine on Jenkins or any non-GitHub-Actions CI
runner against a GitHub repository (or GitHub mirror of a Gerrit/GitLab/
Bitbucket project).

**Who is this for?** Teams whose code lives outside GitHub Actions but who
want to track flaky tests against a GitHub repo (state branch, Issues,
optional PR comments on a GitHub mirror).

**Prerequisites:** A GitHub repository where Quarantine state, Issues, and
optional PR comments will live. The working tree's git origin does NOT need
to be the same GitHub repo — it can be Gerrit, GitLab, Bitbucket, or any
other host (per ADR-037).

---

## What works on Jenkins

Per ADR-037's amended scope boundary:

> **Full features** (PR comments on GitHub PRs, dashboard artifact ingestion)
> **require GitHub Actions.**
>
> **Core CLI features** (quarantine state branch CAS, Issue create/dedup,
> JUnit XML parsing, retry logic, results.json output) **work on any CI
> runner** that has `QUARANTINE_GITHUB_TOKEN` set and network access to
> GitHub.

| Feature | Jenkins | GitHub Actions |
|---------|---------|----------------|
| Quarantine state on `quarantine/state` branch | ✅ | ✅ |
| GitHub Issue creation for new flaky tests | ✅ | ✅ |
| JUnit XML parsing + retry logic | ✅ | ✅ |
| `results.json` written to disk | ✅ | ✅ |
| PR comment posting (with `--pr N` flag) | ✅ (manual PR number) | ✅ (auto from `GITHUB_EVENT_PATH`) |
| Dashboard ingestion of artifacts | ❌ (separate ADR) | ✅ |
| `GITHUB_TOKEN` auto-provisioned | ❌ (use a PAT) | ✅ |

---

## Token setup

Jenkins does not auto-provision `GITHUB_TOKEN`. Create a GitHub Personal
Access Token (classic) with the `repo` scope and store it as a Jenkins
credential:

1. Generate a PAT at <https://github.com/settings/tokens> with `repo` scope.
2. In Jenkins → Manage Jenkins → Credentials → System → Global credentials,
   add a new "Secret text" credential. Recommended ID: `quarantine-github-token`.
3. Reference it in your Jenkinsfile via `withCredentials` and export it as
   `QUARANTINE_GITHUB_TOKEN` (preferred) or `GITHUB_TOKEN`.

---

## Two-phase init (config-driven setup)

Per ADR-037, `quarantine init` is a two-phase flow. Run phase 1 locally:

```bash
quarantine init
```

Phase 1 detects test frameworks, scans `git remote -v` for github.com
candidates (best-effort), and writes `.quarantine/config.yml` with empty
`github.owner` and `github.repo` fields. It exits 2 with hand-edit
instructions:

```
Error [config]: github.owner and github.repo are required.
.quarantine/config.yml has been created. Edit it to set:

  github:
    owner: <your-github-org-or-user>
    repo:  <your-github-repo-name>

Then re-run 'quarantine init' to complete setup.
```

If your working tree has a github.com remote (e.g., a GitHub mirror as
`upstream`), phase 1 will append a hint comment block:

```yaml
github:
  owner: # set to your GitHub organization or user
  repo:  # set to your GitHub repository name
  # detected GitHub remotes (review before using):
  #   origin   -> mhargiss/quarantine
  #   upstream -> mycargus/quarantine
```

Hints are advisory only. Resolution reads the explicit `github.owner` and
`github.repo` fields — never the hints.

Edit the config to fill in the GitHub repo where state and Issues should
land:

```yaml
github:
  owner: my-org
  repo: my-project
```

Commit the config to your repo, then either:

- **Run phase 2 locally** (`quarantine init` re-run): validates the token,
  creates the `quarantine/state` branch, and exits 0 with a "setup complete"
  summary. Useful when you have repo-write access from your laptop.

- **Skip phase 2 — let CI bootstrap the state branch** (per ADR-038): the
  first `quarantine run` in CI will create the `quarantine/state` branch
  itself if it doesn't exist. Useful when CI has a writable token but
  developers don't.

Either path is supported. CI bootstrap is convenient for Jenkins: developers
hand-edit the config, commit it, and the first CI build does the rest.

---

## Jenkinsfile snippet

```groovy
pipeline {
  agent any

  stages {
    stage('Test') {
      steps {
        withCredentials([string(
          credentialsId: 'quarantine-github-token',
          variable: 'QUARANTINE_GITHUB_TOKEN'
        )]) {
          sh '''
            quarantine run backend
          '''
        }
      }
    }
  }

  post {
    always {
      archiveArtifacts artifacts: '.quarantine/**/results.json',
                       allowEmptyArchive: true
    }
  }
}
```

Replace `backend` with the suite name configured in `.quarantine/config.yml`.

If your build is associated with a GitHub PR (e.g., a Gerrit change with a
GitHub mirror PR), pass the PR number explicitly:

```groovy
sh "quarantine run backend --pr ${env.GITHUB_PR_NUMBER}"
```

When `--pr N` is provided, Quarantine posts a PR comment to PR #N on
`github.com/{owner}/{repo}` with the run summary. When neither
`GITHUB_EVENT_PATH` nor `--pr` is available, PR comment posting is silently
skipped — the build is not affected.

---

## Pre-flight verification

Before the first CI build, you can verify the setup from your laptop:

```bash
QUARANTINE_GITHUB_TOKEN=ghp_xxx quarantine doctor
```

`quarantine doctor` reads `github.owner`/`github.repo` from
`.quarantine/config.yml`, makes a single `GET /repos/{owner}/{repo}` call,
and prints:

```
github.owner:  my-org (from config)
github.repo:   my-project (from config)
token:         authenticated
target:        reachable
```

On 4xx response (typo in owner/repo, or token can't see the repo), doctor
exits 2 with `Error: Cannot reach my-org/my-project on GitHub (NNN)` and a
remediation hint. Doctor never inspects the working tree's git origin.

---

## Limitations

- **No dashboard ingestion of Jenkins artifacts.** The Quarantine dashboard
  ingests artifacts from GitHub Actions only. Jenkins users see state and
  Issues on GitHub but no dashboard analytics until the project migrates to
  GitHub Actions.

- **No auto-provisioned token.** Jenkins must supply
  `QUARANTINE_GITHUB_TOKEN` (a PAT). PATs are long-lived and must be
  rotated manually (pre-existing v1 limitation from ADR-008).

- **SHA provenance caveat.** The CLI records `git rev-parse HEAD` from the
  working tree as the run's commit SHA. If your working tree is a Gerrit
  clone whose change SHAs differ from the GitHub mirror's commit SHAs (Gerrit
  rebases tend to alter SHAs), the SHA recorded in `results.json` may not
  exist on the GitHub side. This is acceptable for flakiness tracking — the
  test_id (`<file>::<classname>::<name>`) is the dedup key, not the SHA — but
  worth knowing if you build tooling that joins on the commit SHA.

- **PR comments require an explicit `--pr` flag.** Outside GitHub Actions
  there is no `GITHUB_EVENT_PATH` to read. If your Jenkins job knows the
  PR number (e.g., via a parameter or Gerrit-to-GitHub mirror metadata),
  pass it via `--pr N`. Otherwise, PR comments are skipped silently.

---

## Troubleshooting

**`Error [config]: github.owner and github.repo are required ...`**

Phase 1 of init wrote the partial config but you haven't filled in the
GitHub fields yet, or the config has empty values. Hand-edit
`.quarantine/config.yml` to set `github.owner` and `github.repo`.

**`Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.`**

The Jenkins job is not exporting the credential. Verify the
`withCredentials` block sets `QUARANTINE_GITHUB_TOKEN` and the credential
ID matches.

**`Error: Cannot reach my-org/my-project on GitHub (404). ...`**

Either the repository does not exist, the owner/repo in config has a typo,
or the token does not have access. Run `quarantine doctor` to see GitHub's
own status code, then verify the GitHub URL exists and the token has
`repo` scope on it.

**`[quarantine] WARNING: Cannot create state branch 'quarantine/state': ...`**

The first run could not create the branch (token lacks write scope, network
error, etc.). The build continues in degraded mode. Either run
`quarantine init` phase 2 locally with a writable token, or fix the CI
token's scope.
