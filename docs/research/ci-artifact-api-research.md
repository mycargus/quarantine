# CI Platform Artifact API Research

> Research date: 2026-03-17
> Purpose: Evaluate whether per-provider dashboard pollers are viable for v2+ multi-CI support
> Context: The v1 dashboard polls GitHub Actions artifacts. This document assesses Jenkins, GitLab CI, CircleCI, and Buildkite.

---

## 1. Jenkins

### 1.1 Artifact Storage

- Artifacts are stored on the **Jenkins controller's local filesystem** under `$JENKINS_HOME/jobs/<job>/builds/<build-number>/archive/`.
- Jobs archive artifacts via `archiveArtifacts` in Pipeline or the "Archive the artifacts" post-build action in Freestyle jobs.
- **Retention:** Controlled per-job via "Discard old builds" (keep last N builds, or builds from last N days). No global default -- artifacts persist until builds are deleted. Admins can set system-wide defaults via the "Global Build Discarder" configuration.
- Artifacts consume controller disk directly. Large installations often use the [Artifact Manager on S3](https://plugins.jenkins.io/artifact-manager-s3/) or Azure plugin to offload storage.

### 1.2 Artifact API

Jenkins exposes artifacts through its **Remote Access API** (JSON/XML/Python). There is no separate "artifacts API" -- artifacts are accessed as sub-resources of builds.

**Key endpoints:**

| Operation | Endpoint | Notes |
|-----------|----------|-------|
| List builds for a job | `GET /job/<job>/api/json?tree=builds[number,result,timestamp,artifacts[*]]` | Returns build metadata including artifact list |
| List artifacts for a build | `GET /job/<job>/<build>/api/json?tree=artifacts[*]` | Returns `relativePath`, `fileName`, `displayPath` for each artifact |
| Download a specific artifact | `GET /job/<job>/<build>/artifact/<relativePath>` | Direct file download |
| Download all artifacts as ZIP | `GET /job/<job>/<build>/artifact/*zip*/archive.zip` | Entire archive |
| List builds with depth/range | `GET /job/<job>/api/json?tree=builds[number,timestamp]{0,50}` | Range selector `{M,N}` limits results |

- **Filtering by time:** No native `since` parameter. Must list builds and filter client-side by `timestamp` field (epoch millis). The `tree` parameter and range selectors `{0,N}` help limit payload size.
- **Folder/multibranch jobs:** Path becomes `/job/<folder>/job/<job>/...`. Multibranch pipelines: `/job/<multibranch>/job/<branch>/...`.

**Docs:** https://www.jenkins.io/doc/book/using/remote-access-api/

### 1.3 Authentication

| Method | Details |
|--------|---------|
| API Token | Generated per-user in Jenkins UI. Sent via HTTP Basic Auth (`user:token`). **Recommended for automation.** |
| Username + Password | HTTP Basic Auth. Works but discouraged. |
| SSO/LDAP | Depends on auth plugin; API tokens still work after SSO login. |

- **Minimum permission for read-only artifact access:** The `Job/Read` and `Job/ExtendedRead` permissions (or `Overall/Read` + `Job/Read`). With Role-Based Access Control plugin, a custom "artifact-reader" role with only `Job/Read` is possible.
- No OAuth support natively (some plugins add it, but not standard).

### 1.4 Read-Only Access

**Yes.** Create a dedicated Jenkins user with only `Job/Read` permission. With the Role-Based Authorization Strategy plugin, this can be scoped to specific jobs/folders.

### 1.5 Rate Limits

- **No built-in rate limiting** on the API. Jenkins will serve as many requests as the controller can handle.
- Admins may deploy reverse proxies (nginx, HAProxy) with rate limiting in front of Jenkins.
- Practical concern: Jenkins controllers under heavy load can become unresponsive. The dashboard poller should self-limit (e.g., 1-2 requests/second).

### 1.6 Artifact Naming

- Artifacts are identified by `relativePath` (e.g., `reports/junit.xml`). The CI job chooses the path via the `archiveArtifacts` step's `artifacts` pattern (e.g., `**/test-results/*.xml`).
- The Quarantine CLI can upload a specifically-named artifact (e.g., `quarantine-results.json`) and the poller can download it by exact path.

### 1.7 Polling Feasibility

**Moderate.** There is no "artifacts since timestamp X" endpoint. The polling strategy would be:

1. `GET /job/<job>/api/json?tree=builds[number,timestamp,result]{0,20}` -- list recent builds with timestamps.
2. Filter client-side for `timestamp > lastPollTimestamp`.
3. For each new build, `GET /job/<job>/<build>/artifact/quarantine-results.json` -- download the specific artifact.

This is **2-step but efficient** if the poller tracks the last-seen build number per job. The `tree` parameter keeps responses small.

**Challenge:** Jenkins has no concept of "list all jobs across all folders" in a single call. The poller must be configured with explicit job paths, or recursively walk the folder tree via `/api/json?tree=jobs[name,url,jobs[name,url]]`.

### 1.8 Key Differences from GitHub Actions

| Aspect | GitHub Actions | Jenkins |
|--------|---------------|---------|
| Artifact storage | GitHub-managed, ephemeral (90d default) | Controller filesystem or plugin-managed |
| Discovery | List artifacts by repo, filter by name | Must enumerate builds per job |
| Global listing | `GET /repos/:owner/:repo/actions/artifacts` lists all | No equivalent; must know job paths |
| Authentication | PAT with `actions:read` scope | API token + Basic Auth |
| Multi-repo | One API call per repo | One Jenkins instance may host many jobs; no repo concept |
| Retention | Automatic (configurable per-repo) | Admin-managed, varies wildly |

**Dashboard poller changes needed:**
- Configuration must include Jenkins URL + job paths (no auto-discovery).
- HTTP Basic Auth instead of Bearer token.
- Build-number-based watermarking instead of artifact-ID-based.
- Handle self-hosted TLS certificates.
- No ETag/conditional-request support (Jenkins API does not support ETags on JSON endpoints).

### 1.9 Viability Assessment

**VIABLE but with caveats.** Jenkins has a functional artifact API. The main challenges are: (a) no global artifact listing requires explicit job configuration, (b) self-hosted means variable network/TLS/auth configurations, (c) no rate limiting means the poller must be conservative. A Jenkins poller is straightforward to build but requires more user configuration than GitHub.

---

## 2. GitLab CI

### 2.1 Artifact Storage

- Artifacts are stored by GitLab server, either on local disk (default) or object storage (S3, GCS, Azure -- recommended for production).
- Artifacts are associated with **jobs** within **pipelines**.
- **Retention:** Configurable per-job via `expire_in` in `.gitlab-ci.yml` (e.g., `expire_in: 30 days`). Instance-wide default expiration is configurable by admins. On GitLab.com (SaaS), the default is **30 days**. Artifacts marked `expire_in: never` or with the "Keep" button are retained indefinitely.
- Maximum artifact size: 1 GB on GitLab.com (configurable on self-managed).

### 2.2 Artifact API

GitLab has a **dedicated Job Artifacts API** with rich endpoints. All endpoints are under `/api/v4/`.

**Key endpoints (verified from docs at https://docs.gitlab.com/ee/api/job_artifacts.html):**

| Operation | Endpoint | Notes |
|-----------|----------|-------|
| Download artifacts by job ID | `GET /projects/:id/jobs/:job_id/artifacts` | Returns ZIP archive |
| Download artifacts by ref name | `GET /projects/:id/jobs/artifacts/:ref_name/download?job=name` | Latest successful pipeline for that ref |
| Download single file by job ID | `GET /projects/:id/jobs/:job_id/artifacts/*artifact_path` | Stream individual file from archive |
| Download single file by ref | `GET /projects/:id/jobs/artifacts/:ref_name/raw/*artifact_path?job=name` | Individual file from latest successful |
| Keep artifacts | `POST /projects/:id/jobs/:job_id/artifacts/keep` | Prevent expiration |
| Delete job artifacts | `DELETE /projects/:id/jobs/:job_id/artifacts` | Requires Maintainer role |
| Delete project artifacts | `DELETE /projects/:id/artifacts` | Bulk delete, Maintainer role |

**For discovery/polling, use the Pipelines and Jobs APIs:**

| Operation | Endpoint | Key filters |
|-----------|----------|-------------|
| List project pipelines | `GET /projects/:id/pipelines` | `updated_after`, `updated_before`, `created_after`, `created_before`, `status`, `ref`, `scope` |
| List pipeline jobs | `GET /projects/:id/pipelines/:pipeline_id/jobs` | `scope[]` (e.g., `success`, `failed`) |
| List project jobs | `GET /projects/:id/jobs` | `scope[]` filter |

**Docs:**
- Job Artifacts API: https://docs.gitlab.com/ee/api/job_artifacts.html
- Pipelines API: https://docs.gitlab.com/ee/api/pipelines.html
- Jobs API: https://docs.gitlab.com/ee/api/jobs.html

### 2.3 Authentication

| Method | Details |
|--------|---------|
| Personal Access Token (PAT) | `PRIVATE-TOKEN` header or `private_token` query param. Created in User Settings. |
| Project Access Token | Scoped to a single project. Good for dashboard poller. |
| Group Access Token | Scoped to a group of projects. Premium/Ultimate tier. |
| OAuth 2.0 | Full OAuth flow supported. |
| CI/CD Job Token | `CI_JOB_TOKEN` -- only within CI jobs, limited scope. |
| Deploy Token | Read-only, project-scoped. |

- **Minimum scope for read-only artifact access:** PAT with `read_api` scope. This grants read access to all API endpoints without write permissions. Alternatively, a Project Access Token with `Reporter` role and `read_api` scope.

### 2.4 Read-Only Access

**Yes, excellent support.** A PAT or Project Access Token with `read_api` scope provides read-only access. The Reporter role (minimum for project access tokens) cannot delete artifacts or modify pipelines. Deploy Tokens also work for artifact download with zero write access.

### 2.5 Rate Limits

- **GitLab.com (SaaS):** Authenticated API requests are limited to **2,000 requests per minute** per user (as of current docs). Unauthenticated: 500/min per IP.
- **Self-managed:** Configurable by admin. Default is no rate limit, but admins can set per-user and per-IP limits.
- GitLab also has pipeline-specific rate limits (e.g., max pipelines created per minute), but these don't affect read-only artifact polling.

**For dashboard polling context:** 2,000 req/min is generous. Polling 50 projects every 5 minutes requires ~100-150 requests per cycle (list pipelines + download artifacts), well within limits.

### 2.6 Artifact Naming

- Artifacts are defined in `.gitlab-ci.yml` with `artifacts: paths:` (list of file paths/globs) and `artifacts: name:` (custom archive name, default is the job name).
- Individual files within the archive retain their original paths.
- The Quarantine CLI can upload `quarantine-results.json` and the poller can download it via the single-file endpoint: `GET /projects/:id/jobs/:job_id/artifacts/quarantine-results.json`.

### 2.7 Polling Feasibility

**Excellent.** The Pipelines API supports `updated_after` and `created_after` filters, enabling efficient incremental polling:

1. `GET /projects/:id/pipelines?updated_after=<last_poll_iso>&status=success` -- find new/completed pipelines since last poll.
2. `GET /projects/:id/pipelines/:pipeline_id/jobs?scope[]=success` -- find completed jobs in those pipelines.
3. `GET /projects/:id/jobs/:job_id/artifacts/quarantine-results.json` -- download the specific result file.

This is a **clean 3-step process** with native time-based filtering. Pagination is supported via `page` and `per_page` parameters.

### 2.8 Key Differences from GitHub Actions

| Aspect | GitHub Actions | GitLab CI |
|--------|---------------|-----------|
| Artifact model | Artifacts belong to workflow runs | Artifacts belong to jobs within pipelines |
| Time-based filtering | `created` parameter on list artifacts (limited) | `updated_after`/`created_after` on pipelines (excellent) |
| Single file download | Must download full ZIP, extract | Native single-file download endpoint |
| Auth | PAT with `actions:read` | PAT with `read_api` scope |
| Rate limits | 1,000/hr (GITHUB_TOKEN), 5,000/hr (PAT) | 2,000/min on GitLab.com |
| Retention | 90 days default | 30 days default (configurable) |
| Conditional requests | ETags supported | ETags supported |

**Dashboard poller changes needed:**
- Different auth header (`PRIVATE-TOKEN` instead of `Authorization: Bearer`).
- Pipeline-centric discovery instead of workflow-run-centric.
- Project ID-based URLs instead of `owner/repo`.
- Single-file download is actually simpler (no ZIP extraction needed).
- Shorter default retention means the poller should run frequently.

### 2.9 Viability Assessment

**HIGHLY VIABLE.** GitLab has the best artifact API of all platforms evaluated. Native time-based pipeline filtering, single-file artifact download, granular read-only access tokens, and generous rate limits make it an ideal candidate for a dashboard poller. The GitLab poller may actually be simpler than the GitHub one due to the single-file download endpoint.

---

## 3. CircleCI (Brief Assessment)

### 3.1 Artifact Storage & API

- Artifacts are uploaded via `store_artifacts` step in `.circleci/config.yml`. They are stored in CircleCI's cloud storage.
- **Retention:** 30 days on the Free plan. Configurable on paid plans.
- **API v2 endpoints:**

| Operation | Endpoint |
|-----------|----------|
| List artifacts for a job | `GET /api/v2/project/:project-slug/:job-number/artifacts` |
| List artifacts for a workflow | (not directly; must enumerate jobs) |
| Download artifact | Direct URL returned in the list response (`url` field) |
| List pipelines | `GET /api/v2/project/:project-slug/pipeline` |
| Get pipeline workflows | `GET /api/v2/pipeline/:pipeline-id/workflow` |
| Get workflow jobs | `GET /api/v2/workflow/:workflow-id/job` |

- **Docs:** https://circleci.com/docs/api/v2/

### 3.2 Authentication

- **API Token:** Personal API token or Project API token. Sent via `Circle-Token` header.
- Read-only project tokens are available.

### 3.3 Polling Feasibility

**Moderate.** No `since` filter on pipeline listing. Must paginate through pipelines and check timestamps client-side. The multi-step pipeline -> workflow -> job -> artifacts traversal is verbose (4 API calls to reach artifacts).

### 3.4 Rate Limits

- CircleCI API is rate-limited but limits are not precisely documented. Historically ~300 requests/minute for v2. Response headers include rate limit info.

### 3.5 Viability Assessment

**VIABLE but verbose.** CircleCI has a functional API but the 4-level hierarchy (project -> pipeline -> workflow -> job -> artifacts) makes polling chattier than GitHub or GitLab. No time-based filtering on pipeline listing increases polling overhead. The approach works but requires more API calls per polling cycle.

---

## 4. Buildkite (Brief Assessment)

### 4.1 Artifact Storage & API

- Artifacts are uploaded via `buildkite-agent artifact upload` command. Stored in Buildkite-managed storage or customer's own S3/GCS bucket.
- **Retention:** Managed storage retains artifacts for 6 months. Self-hosted storage has no Buildkite-imposed limit.
- **REST API endpoints:**

| Operation | Endpoint |
|-----------|----------|
| List artifacts for a build | `GET /v2/organizations/:org/pipelines/:pipeline/builds/:build/artifacts` |
| List artifacts for a job | `GET /v2/organizations/:org/pipelines/:pipeline/builds/:build/jobs/:job/artifacts` |
| Download artifact | `GET /v2/organizations/:org/pipelines/:pipeline/builds/:build/artifacts/:artifact/download` |
| List builds | `GET /v2/organizations/:org/pipelines/:pipeline/builds` |

- **Docs:** https://buildkite.com/docs/apis/rest-api/artifacts

### 4.2 Authentication

- **API Access Token:** Created in Buildkite UI. Sent via `Authorization: Bearer <token>`.
- Tokens have granular scopes. `read_artifacts` scope provides read-only artifact access. `read_builds` scope needed to list builds.
- GraphQL API also available (may be more efficient for bulk queries).

### 4.3 Polling Feasibility

**Good.** The builds list endpoint supports filtering by `created_from` (ISO 8601 timestamp) and `state` parameters, enabling incremental polling:

1. `GET /v2/organizations/:org/pipelines/:pipeline/builds?created_from=<timestamp>&state=passed`
2. For each new build, list artifacts and download the target file.

Buildkite also has a **GraphQL API** that can fetch builds + artifacts in a single query, reducing round trips.

### 4.4 Rate Limits

- REST API: **200 requests per minute** per token.
- GraphQL API: Separate limits based on query complexity.

### 4.5 Viability Assessment

**VIABLE and clean.** Buildkite's API is well-designed with time-based build filtering and granular token scopes. The `read_artifacts` scope enables clean read-only access. The 200 req/min rate limit is the main constraint but is sufficient for reasonable polling intervals. The GraphQL API is a bonus for efficient bulk queries.

---

## 5. Comparative Summary

| Feature | GitHub Actions | GitLab CI | Jenkins | CircleCI | Buildkite |
|---------|---------------|-----------|---------|----------|-----------|
| **Viability** | v1 (implemented) | Highly viable | Viable (caveats) | Viable (verbose) | Viable (clean) |
| **Artifact API quality** | Good | Excellent | Adequate | Good | Good |
| **Time-based filtering** | Limited | Excellent (`updated_after`) | None (client-side) | None (client-side) | Good (`created_from`) |
| **Single-file download** | No (full ZIP) | Yes (native) | Yes (by path) | Yes (direct URL) | Yes (by artifact ID) |
| **Read-only token** | PAT `actions:read` | PAT `read_api` | User + Job/Read perm | Project token | `read_artifacts` scope |
| **Rate limits** | 1K-5K/hr | 2K/min | None (self-limit) | ~300/min | 200/min |
| **Default retention** | 90 days | 30 days | Unlimited (disk) | 30 days | 6 months |
| **Auth mechanism** | Bearer token | Private-Token header | Basic Auth | Circle-Token header | Bearer token |
| **Self-hosted option** | GHES | GitLab Self-Managed | Always self-hosted | No (CircleCI Server exists) | No (Buildkite Hybrid exists) |
| **Auto-discovery** | List by repo | List by project | No (must configure jobs) | List by project | List by pipeline |
| **API calls per poll cycle** | 2 (list + download) | 3 (pipelines + jobs + download) | 2 (builds + download) | 4+ (pipeline chain) | 2 (builds + download) |

## 6. Recommendation for v2 Strategy

### Priority order for multi-CI support:

1. **GitLab CI (P1):** Best API, largest market share after GitHub, excellent time-based filtering, native single-file download. Easiest to implement after GitHub.

2. **Jenkins (P2):** Huge installed base especially in enterprise. API is adequate. Main cost is the additional configuration burden (job paths, self-hosted TLS). Worth it for market reach.

3. **Buildkite (P3):** Clean API, growing adoption in mid-to-large companies. Low implementation effort. Good candidate if there is customer demand.

4. **CircleCI (P4):** Functional but the verbose API hierarchy increases implementation and maintenance cost relative to the others. Implement if demanded.

### Architecture implications:

The dashboard poller should be designed with a **provider interface**:

```
type ArtifactPoller interface {
    // ListNewRuns returns test runs since the given checkpoint
    ListNewRuns(ctx context.Context, project Project, since Checkpoint) ([]RunReference, error)
    // DownloadResult fetches a single test result artifact
    DownloadResult(ctx context.Context, ref RunReference) ([]byte, error)
}
```

Each CI platform implements this interface. The checkpoint type varies (artifact ID for GitHub, pipeline ID + timestamp for GitLab, build number for Jenkins, etc.). This is consistent with the existing architecture where the dashboard is a non-critical, read-only consumer.

All four platforms support the core requirement: **read-only polling of named test result artifacts from completed builds, authenticated via tokens, without write access to the CI system.**

---

## Sources

- GitLab Job Artifacts API: https://docs.gitlab.com/ee/api/job_artifacts.html (fetched and verified 2026-03-17)
- GitLab Pipelines API: https://docs.gitlab.com/ee/api/pipelines.html (fetched and verified 2026-03-17)
- Jenkins Remote Access API: https://www.jenkins.io/doc/book/using/remote-access-api/
- Buildkite Artifacts API: https://buildkite.com/docs/apis/rest-api/artifacts
- CircleCI API v2: https://circleci.com/docs/api/v2/
- GitLab rate limits: https://docs.gitlab.com/ee/user/gitlab_com/index.html
- Buildkite rate limits: https://buildkite.com/docs/apis/rest-api#rate-limiting
