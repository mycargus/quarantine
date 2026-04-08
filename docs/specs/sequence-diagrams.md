# Sequence Diagrams for Key Flows

> Last updated: 2026-04-07
>
> Mermaid sequence diagrams for the 12 key flows (8 v1, 4 v2).
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

## 9. Dashboard: JWT → Installation Token Exchange (v2)

The dashboard generates a JWT from the App's private key, exchanges it for
a short-lived installation token, and uses that token for GitHub API calls.
The `InstallationTokenProvider` caches the token and refreshes proactively.

```mermaid
sequenceDiagram
    participant Caller as Dashboard (polling / sync)
    participant TP as InstallationTokenProvider
    participant JWT as generateJWT()
    participant API as GitHub API

    Caller->>TP: getToken(installationId)

    alt Cached token valid (>5 min remaining)
        TP-->>Caller: cached installation token
    else No token or <5 min remaining
        TP->>JWT: generateJWT(clientID, privateKeyPEM, now)
        Note over JWT: Pure function: RS256 JWT<br>iss=clientID, iat=now-60s, exp=now+9min
        JWT-->>TP: signed JWT

        TP->>API: POST /app/installations/{id}/access_tokens<br>Authorization: Bearer {jwt}<br>(no permissions body)
        alt 201 Created
            API-->>TP: { token, expires_at, permissions }
            Note over TP: Cache token + expires_at
            TP-->>Caller: new installation token
        else 401 Unauthorized (bad JWT)
            API-->>TP: 401
            Note over TP: Log warning: JWT rejected
            TP-->>Caller: error (do not throw)
        else Network error / 5xx
            API-->>TP: error
            Note over TP: Log warning: exchange failed
            TP-->>Caller: error (do not throw)
        end
    end

    Caller->>API: GET /repos/{owner}/{repo}/actions/artifacts<br>Authorization: Bearer {installation_token}
    API-->>Caller: artifact list
```

---

## 10. Dashboard: Startup Sync + Discovery Loop (v2)

On startup, the dashboard blocks HTTP traffic until installation discovery
completes. After startup, a 15-minute interval re-syncs periodically.
SIGTERM/SIGINT stops the loop cleanly.

```mermaid
sequenceDiagram
    participant Proc as Dashboard Process
    participant Sync as syncInstallations()
    participant TP as InstallationTokenProvider
    participant API as GitHub API
    participant DB as SQLite
    participant HTTP as HTTP Server

    Note over Proc: Process starts

    Note over Proc: Validate App credentials<br>(CLIENT_ID, PRIVATE_KEY)
    alt Credentials missing or invalid
        Proc-->>Proc: Exit with descriptive error<br>(HTTP server never starts)
    end

    Proc->>Sync: syncInstallations() [blocking]

    Note over Sync: Generate JWT for App-level API call
    Sync->>API: GET /app/installations?per_page=100<br>Authorization: Bearer {jwt}
    API-->>Sync: installations page 1 (with IDs)

    loop Follow Link rel="next" until all pages fetched
        Sync->>API: GET /app/installations?page=N&per_page=100
        API-->>Sync: installations page N
    end

    loop For each installation
        Sync->>TP: getToken(installationId)
        TP->>API: POST /app/installations/{id}/access_tokens<br>Authorization: Bearer {jwt}
        API-->>TP: installation token
        TP-->>Sync: installation token

        Sync->>API: GET /installation/repositories?per_page=100<br>Authorization: Bearer {installation_token}
        API-->>Sync: repos page 1

        loop Follow Link rel="next" until all pages fetched
            Sync->>API: GET /installation/repositories?page=N&per_page=100
            API-->>Sync: repos page N
        end

        Sync->>DB: UPSERT installations (id, suspended_at, ...)
        Sync->>DB: UPSERT projects (owner, repo, installation_id)
    end

    alt Repo removed from installation
        Sync->>DB: UPDATE projects SET installation_id = NULL<br>WHERE repo not in discovered list<br>(preserves test_runs, quarantined_tests)
    end

    Sync-->>Proc: sync complete

    Note over Proc: Start HTTP server
    Proc->>HTTP: listen()

    Note over Proc: Start background loop<br>setInterval(syncInstallations, 15min)

    loop Every 15 minutes
        alt Sync succeeds
            Sync->>DB: UPSERT installations + projects
        else Sync fails (500, network error, etc.)
            Note over Sync: Log error, do NOT throw<br>Existing DB data unchanged
        end
    end

    alt SIGTERM or SIGINT received
        Proc->>Proc: clearInterval(discoveryLoop)
        Note over Proc: No further sync calls
        Proc->>HTTP: close()
    end
```

---

## 11. Dashboard: OAuth Login Flow (v2)

A user logs in via GitHub OAuth. `@remix-run/auth` handles the redirect and
code exchange. The dashboard stores the access token in an encrypted session
cookie (8-hour Max-Age). No server-side session table. No refresh tokens.

```mermaid
sequenceDiagram
    participant User as User (Browser)
    participant Dash as Dashboard
    participant GH as GitHub (github.com)

    User->>Dash: GET / (no session)
    Dash-->>User: 401 Unauthorized

    User->>Dash: GET /auth/login
    Note over Dash: @remix-run/auth builds<br>OAuth URL with PKCE + state param (CSRF)
    Dash-->>User: 302 Redirect → github.com/login/oauth/authorize<br>?client_id={id}&redirect_uri={callback}&state={csrf}

    User->>GH: Authorize Quarantine App
    GH-->>User: 302 Redirect → /auth/github/callback?code={code}&state={csrf}

    User->>Dash: GET /auth/github/callback?code={code}&state={csrf}
    Note over Dash: @remix-run/auth validates state + PKCE,<br>exchanges code for tokens

    Dash->>GH: POST github.com/login/oauth/access_token<br>(client_id, client_secret, code)
    GH-->>Dash: { access_token (ghu_, 8hr) }

    Note over Dash: Store access_token + user profile<br>in encrypted cookie (Max-Age: 28800)
    Dash-->>User: Set-Cookie: session={encrypted}<br>(httpOnly, secure, SameSite=Lax, Max-Age=28800)<br>302 Redirect → /

    User->>Dash: GET / (with session cookie)
    Note over Dash: Decrypt cookie → access_token

    alt Cookie expired (after 8 hours)
        Dash-->>User: 401 (re-authenticate via OAuth)
    else Cookie valid
        Dash-->>User: 200 OK (dashboard page)
    end
```

---

## 12. Dashboard: User Permission Filtering (v2)

After OAuth login, the dashboard filters the project list to only repos the
user can access via their GitHub permissions. Note: `GET /user/installations`
returns installation metadata only (no repos). A separate call to
`GET /user/installations/{id}/repositories` per installation is required to
list repos the user can access.

```mermaid
sequenceDiagram
    participant User as User (Browser)
    participant Dash as Dashboard
    participant API as GitHub API
    participant DB as SQLite

    User->>Dash: GET / (authenticated, session cookie)
    Note over Dash: session() middleware decrypts cookie<br>→ access_token + user profile

    Dash->>API: GET /user/installations?per_page=100<br>Authorization: Bearer {user_access_token}
    API-->>Dash: installations page 1<br>(metadata only, no repo lists)

    loop Follow Link rel="next" until all pages fetched
        Dash->>API: GET /user/installations?page=N&per_page=100
        API-->>Dash: installations page N
    end

    loop For each installation
        Dash->>API: GET /user/installations/{id}/repositories?per_page=100<br>Authorization: Bearer {user_access_token}
        API-->>Dash: repos page 1

        loop Follow Link rel="next" until all pages fetched
            Dash->>API: GET /user/installations/{id}/repositories?page=N&per_page=100
            API-->>Dash: repos page N
        end
    end

    Dash->>DB: SELECT * FROM projects<br>WHERE installation_id IS NOT NULL
    DB-->>Dash: all App-discovered projects

    Note over Dash: Intersect: projects in DB<br>∩ repos user can access<br>(pure function)

    alt User has access to some repos
        Dash-->>User: 200 OK — filtered project list
    else User has access to zero repos
        Dash-->>User: 200 OK — empty project list (not an error)
    end
```

---

## Participant Reference

| Participant | Description |
|---|---|
| User/Developer | Human running CLI locally or CI triggering it |
| CLI | The `quarantine` Go binary |
| Test Runner | The wrapped test framework (Jest, RSpec, Vitest) |
| GitHub API | GitHub REST API (Contents, Search, Issues, Artifacts, App) |
| GitHub Branch | The `quarantine/state` branch storing `quarantine.json` |
| GitHub Issues | GitHub Issues used to track flaky tests |
| GitHub Actions Cache | Fallback cache for `quarantine.json` in degraded mode |
| Dashboard | Remix 3 analytics application |
| SQLite | Dashboard's local database (WAL mode) |
| Poll Timer | Dashboard's scheduled polling worker |
| generateJWT() | Pure function: produces RS256 JWT for App auth (v2) |
| InstallationTokenProvider | Token cache + exchange: JWT → installation token (v2) |
| syncInstallations() | Discovery function: lists installations + repos, upserts DB (v2) |
| GitHub (github.com) | OAuth endpoints on github.com (not api.github.com) (v2) |
