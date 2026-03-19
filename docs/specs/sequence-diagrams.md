# Sequence Diagrams for Key Flows

> Last updated: 2026-03-17
>
> Mermaid sequence diagrams for the 8 key flows.
> Each diagram is self-contained and renderable in GitHub.

---

## 1. Happy Path: Flaky Test Detected and Quarantined

A test fails on the first run, passes on retry, and is quarantined. The CLI
creates a GitHub Issue, posts a PR comment, writes results to disk, and exits 0.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Branch as GitHub Branch<br>(quarantine/state)
    participant Issues as GitHub Issues
    participant Runner as Test Runner

    Dev->>CLI: quarantine run -- <test command>
    CLI->>API: GET /repos/{owner}/{repo}/contents/quarantine.json<br>(from quarantine/state)
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA
    API-->>CLI: quarantine.json (with existing quarantined tests)

    Note over CLI: Batch check issue status for quarantined tests
    CLI->>API: GET /search/issues?q=repo:{owner}/{repo}+label:quarantine+is:closed
    API-->>CLI: closed issues list (if any)

    alt Some issues are closed
        Note over CLI: Remove unquarantined tests from quarantine list
    end

    Note over CLI: Build test command with framework-specific<br>exclusion flags for quarantined tests
    CLI->>Runner: Execute test command (quarantined tests excluded)
    Runner-->>CLI: Exit code + JUnit XML output

    Note over CLI: Parse JUnit XML
    Note over CLI: Test "testFoo" failed

    CLI->>Runner: Re-run "testFoo" (retry 1 of N)
    Runner-->>CLI: Exit code + JUnit XML output

    alt Passes on retry
        Note over CLI: Classify "testFoo" as flaky
    else Fails all retries
        Note over CLI: Classify as genuine failure
    end

    Note over CLI: Update quarantine.json with new flaky test

    CLI->>API: PUT /repos/{owner}/{repo}/contents/quarantine.json<br>(SHA-based CAS)
    API->>Branch: write file
    alt CAS success
        Branch-->>API: new SHA
        API-->>CLI: 200 OK
    else CAS conflict (409)
        API-->>CLI: 409 Conflict
        Note over CLI: Re-read, merge, retry (max 3)
    end

    Note over CLI: Check for existing issue (dedup)
    CLI->>API: GET /search/issues?q=repo:{owner}/{repo}+label:quarantine+"testFoo"+is:open
    API-->>CLI: no matching issue

    CLI->>API: POST /repos/{owner}/{repo}/issues<br>(title, body, labels: [quarantine])
    API->>Issues: create issue
    Issues-->>API: issue #42
    API-->>CLI: issue URL + number

    Note over CLI: Post or update PR comment
    CLI->>API: GET /repos/{owner}/{repo}/issues/{pr}/comments
    API-->>CLI: existing comments

    alt No existing quarantine comment
        CLI->>API: POST /repos/{owner}/{repo}/issues/{pr}/comments<br>(with <!-- quarantine-bot --> marker)
    else Existing quarantine comment found
        CLI->>API: PATCH /repos/{owner}/{repo}/issues/comments/{id}
    end

    Note over CLI: Write results JSON to .quarantine/results.json
    CLI-->>Dev: Exit 0 (flaky test quarantined, not a real failure)
```

---

## 2. Happy Path: Quarantined Test Excluded from Execution

All quarantined tests have open issues. They are excluded from the test
command. All remaining tests pass.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Branch as GitHub Branch<br>(quarantine/state)
    participant Runner as Test Runner

    Dev->>CLI: quarantine run -- <test command>
    CLI->>API: GET /repos/{owner}/{repo}/contents/quarantine.json<br>(from quarantine/state)
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA
    API-->>CLI: quarantine.json (3 quarantined tests)

    Note over CLI: Batch check issue status
    CLI->>API: GET /search/issues?q=repo:{owner}/{repo}+label:quarantine+is:closed
    API-->>CLI: no closed issues

    Note over CLI: All 3 issues still open —<br>all quarantined tests remain excluded

    Note over CLI: Build test command with framework-specific<br>exclusion flags (e.g., --testPathIgnorePatterns,<br>--exclude, --exclude for rspec/jest/vitest)

    CLI->>Runner: Execute test command (3 quarantined tests excluded)
    Runner-->>CLI: Exit 0 + JUnit XML output

    Note over CLI: Parse JUnit XML — all ran tests passed

    Note over CLI: No new flaky tests detected,<br>no state changes needed

    Note over CLI: Write results JSON to .quarantine/results.json
    CLI-->>Dev: Exit 0
```

---

## 3. Quarantined Test's Issue Is Closed (Unquarantine)

A developer closes a quarantine issue. On the next CLI run, the test is
removed from the quarantine list and runs normally again.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Branch as GitHub Branch<br>(quarantine/state)
    participant Runner as Test Runner

    Dev->>CLI: quarantine run -- <test command>
    CLI->>API: GET /repos/{owner}/{repo}/contents/quarantine.json<br>(from quarantine/state)
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA (includes "testFoo")
    API-->>CLI: quarantine.json

    Note over CLI: Batch check issue status
    CLI->>API: GET /search/issues?q=repo:{owner}/{repo}+label:quarantine+is:closed
    API-->>CLI: issue #42 for "testFoo" is closed

    Note over CLI: Remove "testFoo" from quarantine list

    CLI->>API: PUT /repos/{owner}/{repo}/contents/quarantine.json<br>(SHA-based CAS, "testFoo" removed)
    API->>Branch: write updated file
    Branch-->>API: new SHA
    API-->>CLI: 200 OK

    Note over CLI: "testFoo" is no longer excluded —<br>build test command without excluding it

    CLI->>Runner: Execute test command ("testFoo" now included)
    Runner-->>CLI: Exit code + JUnit XML output

    Note over CLI: Parse JUnit XML

    alt "testFoo" passes
        Note over CLI: Test is healthy again
        CLI-->>Dev: Exit 0
    else "testFoo" fails
        Note over CLI: "testFoo" is a real failure now —<br>it fails the build like any other test
        CLI-->>Dev: Exit 1
    end
```

---

## 4. Concurrent Builds: CAS Conflict on quarantine.json

Two CI builds detect flaky tests simultaneously and both try to update
quarantine.json. The second build hits a 409 conflict and retries.

```mermaid
sequenceDiagram
    participant A as Build A (CLI)
    participant API as GitHub API
    participant Branch as GitHub Branch<br>(quarantine/state)
    participant B as Build B (CLI)

    Note over A,B: Both builds start around the same time

    A->>API: GET quarantine.json
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA1
    API-->>A: quarantine.json (SHA1)

    B->>API: GET quarantine.json
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA1
    API-->>B: quarantine.json (SHA1)

    Note over A: Build A detects flaky test "testAlpha"<br>and adds it to quarantine.json
    Note over B: Build B detects flaky test "testBeta"<br>and adds it to quarantine.json

    A->>API: PUT quarantine.json (SHA1 → new content)
    API->>Branch: CAS check: SHA1 matches current
    Branch-->>API: write OK → SHA2
    API-->>A: 200 OK (SHA2)

    B->>API: PUT quarantine.json (SHA1 → new content)
    API->>Branch: CAS check: SHA1 does NOT match (now SHA2)
    Branch-->>API: conflict
    API-->>B: 409 Conflict

    Note over B: Retry 1 of 3: re-read and merge

    B->>API: GET quarantine.json
    API->>Branch: read file
    Branch-->>API: quarantine.json + SHA2 (includes "testAlpha")
    API-->>B: quarantine.json (SHA2)

    Note over B: Merge: keep "testAlpha" (from A),<br>add "testBeta" (from B)

    B->>API: PUT quarantine.json (SHA2 → merged content)
    API->>Branch: CAS check: SHA2 matches current
    Branch-->>API: write OK → SHA3
    API-->>B: 200 OK (SHA3)

    Note over A,B: Final state: quarantine.json (SHA3)<br>contains both "testAlpha" and "testBeta"
```

---

## 5. Degraded Mode: GitHub API Unreachable (Cache Hit)

The GitHub API is unreachable, but the CLI falls back to cached
quarantine.json from the GitHub Actions cache.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Cache as GitHub Actions Cache
    participant Runner as Test Runner

    Dev->>CLI: quarantine run -- <test command>

    CLI->>API: GET /repos/{owner}/{repo}/contents/quarantine.json
    Note over API: Network timeout / 5xx error
    API-->>CLI: timeout / error

    Note over CLI: GitHub API unreachable —<br>attempt Actions cache fallback

    CLI->>Cache: Restore cache (quarantine-state key)
    Cache-->>CLI: cache hit — cached quarantine.json

    Note over CLI: ⚠ Running in degraded mode<br>stderr: [quarantine] WARNING: running in degraded mode<br>(GitHub API unreachable, using cached state)

    Note over CLI: Exclude quarantined tests using cached data
    CLI->>Runner: Execute test command (quarantined tests excluded)
    Runner-->>CLI: Exit code + JUnit XML output

    Note over CLI: Parse JUnit XML

    alt New flaky test detected via retry
        Note over CLI: Cannot update quarantine.json —<br>API unreachable. Log warning.
        Note over CLI: Cannot create GitHub Issue —<br>API unreachable. Log warning.
        Note over CLI: Cannot post PR comment —<br>API unreachable. Log warning.
    end

    alt Running in GitHub Actions
        Note over CLI: Emit ::warning annotation:<br>"Quarantine running in degraded mode"
    end

    Note over CLI: Write results JSON to .quarantine/results.json
    CLI-->>Dev: Exit 0 (based on test results only)
```

---

## 6. Degraded Mode: No Cache, No API

Both the GitHub API and Actions cache are unavailable. The CLI runs all tests
with no exclusions but still detects flaky tests via retry.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Cache as GitHub Actions Cache
    participant Runner as Test Runner

    Dev->>CLI: quarantine run -- <test command>

    CLI->>API: GET /repos/{owner}/{repo}/contents/quarantine.json
    Note over API: timeout / error
    API-->>CLI: timeout / error

    CLI->>Cache: Restore cache (quarantine-state key)
    Cache-->>CLI: cache miss — no cached data

    Note over CLI: ⚠ Running in degraded mode (no state available)<br>stderr: [quarantine] WARNING: running in degraded mode<br>(no quarantine state available, running all tests)

    Note over CLI: No quarantine state — run all tests with no exclusions
    CLI->>Runner: Execute test command (all tests run)
    Runner-->>CLI: Exit code + JUnit XML output

    Note over CLI: Parse JUnit XML

    alt Test failures detected
        CLI->>Runner: Re-run failing tests (retry 1 of N)
        Runner-->>CLI: Exit code + JUnit XML output

        alt Passes on retry (flaky)
            Note over CLI: Classify as flaky — forgive failure<br>(cannot quarantine without API, but retry still works)
            Note over CLI: Log warning: flaky test detected<br>but cannot update quarantine state
        else Fails all retries (genuine failure)
            Note over CLI: Classify as real failure
        end
    end

    alt Running in GitHub Actions
        Note over CLI: Emit ::warning annotation
    end

    Note over CLI: Write results JSON to .quarantine/results.json
    CLI-->>Dev: Exit based on test results<br>(0 if all pass or only flaky, 1 if real failures)
```

---

## 7. Dashboard: Artifact Ingestion

The dashboard polls GitHub Artifacts on a schedule, downloads new results,
and upserts them into SQLite.

```mermaid
sequenceDiagram
    participant Timer as Poll Timer
    participant Dash as Dashboard
    participant API as GitHub API
    participant DB as SQLite

    Timer->>Dash: Poll cycle triggered (every 5 min, staggered per repo)

    Dash->>DB: SELECT last_synced, last_etag FROM projects<br>WHERE id = {project_id}
    DB-->>Dash: last_synced timestamp, last_etag

    Dash->>API: GET /repos/{owner}/{repo}/actions/artifacts<br>?name=quarantine-results<br>If-None-Match: {last_etag}

    alt 304 Not Modified
        API-->>Dash: 304 (no new artifacts)
        Note over Dash: Nothing to do — skip this cycle
    else 200 OK with new artifacts
        API-->>Dash: artifact list + new ETag

        loop For each new artifact
            Dash->>API: GET /repos/{owner}/{repo}/actions/artifacts/{id}/zip
            API-->>Dash: artifact ZIP (contains results JSON)

            Note over Dash: Extract and parse results JSON

            alt Valid JSON matching schema
                Dash->>DB: BEGIN TRANSACTION
                Dash->>DB: UPSERT INTO test_runs (...)
                Dash->>DB: UPSERT INTO tests (...)
                Dash->>DB: INSERT INTO test_results (...)
                Dash->>DB: INSERT INTO quarantine_events (...)<br>(if quarantine/unquarantine detected)
                Dash->>DB: COMMIT
            else Malformed JSON
                Note over Dash: Log warning, skip this artifact
            end
        end

        Dash->>DB: UPDATE projects<br>SET last_synced = now(), last_etag = {new_etag}<br>WHERE id = {project_id}
    end
```

---

## 8. quarantine init

A developer initializes Quarantine for their repository. The CLI walks them
through interactive prompts, writes configuration, validates access, and
creates the state branch.

```mermaid
sequenceDiagram
    participant Dev as User/Developer
    participant CLI
    participant API as GitHub API
    participant Branch as GitHub Branch<br>(quarantine/state)

    Dev->>CLI: quarantine init

    alt quarantine.yml already exists
        CLI-->>Dev: Prompt: quarantine.yml already exists. Overwrite? [y/N]
        alt Developer enters n or presses enter
            Note over CLI: Print "Aborted. Existing quarantine.yml preserved." and exit 0
        end
        Note over CLI: Developer entered y — proceed with init
    end

    CLI-->>Dev: Prompt: Select test framework<br>(rspec / jest / vitest)
    Dev->>CLI: jest

    CLI-->>Dev: Prompt: Number of retries (default: 3)
    Dev->>CLI: 3

    CLI-->>Dev: Prompt: JUnit XML output path<br>(default: framework-specific)
    Dev->>CLI: results/junit.xml

    Note over CLI: Auto-detect owner/repo from git remote

    Note over CLI: Write quarantine.yml to repo root
    Note over CLI: quarantine.yml contains:<br>version: 1, framework: jest,<br>retries: 3, junitxml: results/junit.xml

    Note over CLI: Validate GitHub token
    alt QUARANTINE_GITHUB_TOKEN set
        Note over CLI: Use QUARANTINE_GITHUB_TOKEN
    else GITHUB_TOKEN set
        Note over CLI: Fall back to GITHUB_TOKEN
    else No token found
        CLI-->>Dev: Error: No GitHub token found.<br>Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.
        Note over CLI: Exit 2
    end

    CLI->>API: GET /repos/{owner}/{repo}
    alt 401 / 403
        CLI-->>Dev: Error: Token lacks required permissions.<br>Needs 'repo' scope.
        Note over CLI: Exit 2
    else 404
        CLI-->>Dev: Error: Repository not found.<br>Check owner/repo in git remote.
        Note over CLI: Exit 2
    else 200 OK
        API-->>CLI: repo metadata
        Note over CLI: Repo access confirmed
    end

    CLI->>API: GET /repos/{owner}/{repo}/git/ref/heads/quarantine/state

    alt Branch already exists
        API-->>CLI: 200 OK (branch ref)
        Note over CLI: Branch exists — skip creation
    else Branch does not exist (404)
        API-->>CLI: 404

        Note over CLI: Create quarantine/state branch

        CLI->>API: GET /repos/{owner}/{repo}/git/ref/heads/main
        API-->>CLI: main branch SHA

        CLI->>API: POST /repos/{owner}/{repo}/git/refs<br>(ref: refs/heads/quarantine/state, sha: main SHA)
        API-->>CLI: 201 Created

        Note over CLI: Write empty quarantine.json to new branch
        CLI->>API: PUT /repos/{owner}/{repo}/contents/quarantine.json<br>(branch: quarantine/state,<br>content: {"version":1,"updated_at":"...","tests":{}})
        API->>Branch: create file
        Branch-->>API: file SHA
        API-->>CLI: 201 Created
    end

    CLI-->>Dev: ✓ quarantine.yml written<br>✓ GitHub access verified<br>✓ quarantine/state branch ready<br><br>Next steps:<br>1. Add quarantine.yml to version control<br>2. Add to CI: quarantine run -- <your test command><br>3. Set QUARANTINE_GITHUB_TOKEN in CI secrets
```

---

## Participant Reference

| Participant | Description |
|---|---|
| User/Developer | Human running CLI locally or CI triggering it |
| CLI | The `quarantine` Go binary |
| Test Runner | The wrapped test framework (Jest, RSpec, Vitest) |
| GitHub API | GitHub REST API (Contents, Search, Issues, Artifacts) |
| GitHub Branch | The `quarantine/state` branch storing `quarantine.json` |
| GitHub Issues | GitHub Issues used to track flaky tests |
| GitHub Actions Cache | Fallback cache for `quarantine.json` in degraded mode |
| Dashboard | React Router v7 analytics application |
| SQLite | Dashboard's local database (WAL mode) |
| Poll Timer | Dashboard's scheduled polling worker |
