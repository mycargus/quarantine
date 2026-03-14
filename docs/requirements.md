# Quarantine - Project Requirements

## Product Summary

Quarantine automatically detects, disables (quarantines), and tracks flaky
(non-deterministic) tests in CI pipelines. It wraps the user's test command,
detects flaky tests by re-running failures, quarantines them so builds pass,
creates GitHub issues, and provides a dashboard for visibility.

---

## 1. Functional Requirements

### 1.1 Flaky Test Detection

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.1.1 | CLI wraps the user's test command via `quarantine run -- <test command>`. | [v1] |
| FR-1.1.2 | Parses JUnit XML output to identify test failures. Supports a glob pattern for multiple XML files (e.g., `--junitxml="results/*.xml"`); the CLI merges all matching files before processing. | [v1] |
| FR-1.1.3 | Re-runs individual failing tests N times (configurable, default 3). | [v1] |
| FR-1.1.4 | If a test passes on any retry, it is flagged as flaky. | [v1] |
| FR-1.1.5 | Auto-detects the test framework from JUnit XML structure. | [v1] |
| FR-1.1.6 | Ships with baked-in rerun command signatures for: RSpec, Jest, and Vitest [v1]. Adds pytest, go test (`gotestsum`), JUnit/Maven, PHPUnit, NUnit, and others in [v2+]. | [v1] |
| FR-1.1.7 | User can override the rerun command in `quarantine.yml` config. | [v1] |

### 1.2 Quarantine Management

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.2.1 | Maintains `quarantine.json` on a dedicated GitHub branch (`quarantine/state`). | [v1] |
| FR-1.2.2 | Quarantined test failures are suppressed by converting them to skips in modified JUnit XML output. | [v1] |
| FR-1.2.3 | Build exits 0 if only quarantined tests failed; exits 1 for real (non-quarantined) failures. | [v1] |
| FR-1.2.4 | On each run, checks if the GitHub issue for a quarantined test is closed; if so, unquarantines the test. | [v1] |
| FR-1.2.5 | Uses optimistic concurrency for `quarantine.json` updates (SHA-based compare-and-swap via GitHub API, with retry on conflict). | [v1] |

### 1.3 GitHub Integration

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.3.1 | Creates GitHub issues for newly detected flaky tests, with appropriate labels. | [v1] |
| FR-1.3.2 | Posts PR comments summarizing flaky test results. | [v1] |
| FR-1.3.3 | Performs check-before-create for issues to avoid duplicates (searches by label + test ID). | [v1] |
| FR-1.3.4 | Uses the GitHub Contents API for `quarantine.json` state management. | [v1] |

### 1.4 Configuration

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.4.1 | Configuration is stored in `quarantine.yml` in the repository root. | [v1] |
| FR-1.4.2 | Configurable options include: retries, framework (auto-detected if omitted), and github owner/repo (auto-detected from git remote). | [v1] |
| FR-1.4.3 | `quarantine validate` command checks configuration for correctness. | [v1] |

### 1.5 Dashboard

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.5.1 | Web UI displays quarantined tests per project, trends over time, and test stability metrics. | [v1] |
| FR-1.5.2 | Pulls test results from GitHub Artifacts (not pushed by the CLI). | [v1] |
| FR-1.5.3 | Uses hybrid polling: background sync every 5 minutes plus on-demand sync when a user views a page. | [v1] |
| FR-1.5.4 | Uses SQLite for historical data storage. | [v1] |
| FR-1.5.5 | Provides cross-repo visibility (one dashboard instance for an entire org). | [v1] |
| FR-1.5.6 | Built with React Router v7 (framework mode), TypeScript, and Tailwind CSS. | [v1] |

### 1.6 Degraded Mode

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.6.1 | CLI operates normally if the dashboard is unreachable; results are stored in GitHub Artifacts. | [v1] |
| FR-1.6.2 | CLI operates normally if the GitHub API is unreachable; falls back to cached `quarantine.json`. | [v1] |
| FR-1.6.3 | CLI must never fail the build due to Quarantine infrastructure issues. | [v1] |

### 1.7 Code Sync Adapter

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.7.1 | Automated PRs to add framework-specific skip markers in source code for quarantined tests. | [v2+] |
| FR-1.7.2 | Automated PRs to remove skip markers when the corresponding issue is closed. | [v2+] |
| FR-1.7.3 | Changes are batched: one PR per day with all quarantine changes, not one PR per test. | [v2+] |
| FR-1.7.4 | Optional feature, toggled per project in configuration. | [v2+] |

### 1.8 Monorepo Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.8.1 | v1 assumes one `quarantine.yml` and one `quarantine.json` per repository. | [v1] |
| FR-1.8.2 | Add a `scope` or `project` field in `quarantine.yml` to namespace test IDs for monorepos. | [v2+] |
| FR-1.8.3 | The `quarantine.json` key structure uses full test IDs (including file path), enabling future scope prefixes without breaking existing entries. | [v1] |

### 1.9 Expanded CI Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.9.1 | Support Jenkins, GitLab CI, and Bitbucket Pipelines. | [v2+] |
| FR-1.9.2 | CI-provider detection for appropriate artifact/cache storage backend. | [v2+] |

### 1.10 Expanded Ticket Tracker Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.10.1 | Jira integration for issue creation and lifecycle management. | [v2+] |
| FR-1.10.2 | Tracker is configurable in `quarantine.yml`. | [v2+] |

### 1.11 Authentication

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.11.1 | CLI authenticates via GitHub PAT for contents, issues, and PR comment operations. | [v1] |
| FR-1.11.2 | Dashboard uses a GitHub PAT to pull artifacts. | [v1] |
| FR-1.11.3 | No API keys required in v1 (CLI talks directly to GitHub, not the dashboard). | [v1] |
| FR-1.11.4 | GitHub App installed on org with short-lived tokens (1-hour expiry, client-side refresh) and fine-grained permissions. | [v2+] |
| FR-1.11.5 | GitHub OAuth for dashboard login via remix-auth. | [v2+] |
| FR-1.11.6 | Org-level integration: install once, all repos covered. | [v2+] |

### 1.12 Notifications

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.12.1 | PR comments posted on the triggering PR/commit. | [v1] |
| FR-1.12.2 | Slack integration for flaky test notifications. | [v2+] |
| FR-1.12.3 | Email notifications for flaky test events. | [v2+] |
| FR-1.12.4 | Configurable threshold alerts (e.g., "notify when >N tests quarantined"). | [v2+] |
| FR-1.12.5 | Periodic flaky test summary reports. | [v2+] |

---

## 2. Non-Functional Requirements

### 2.1 Performance

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.1.1 | CLI overhead must be less than 5 seconds added to a test run, excluding retry time. | [v1] |
| NFR-2.1.2 | Dashboard page load must be under 2 seconds. | [v1] |
| NFR-2.1.3 | System must support 50+ concurrent CI builds without conflicts. | [v1] |

### 2.2 Reliability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.2.1 | CLI must never break a build due to its own failure. | [v1] |
| NFR-2.2.2 | Graceful degradation when GitHub API or dashboard is unreachable. | [v1] |
| NFR-2.2.3 | Result ingestion must be idempotent (duplicate artifact processing is safe). | [v1] |

### 2.3 Security

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.3.1 | Dashboard is internal-only (deployed behind corporate network). | [v1] |
| NFR-2.3.2 | Reverse proxy with rate limiting at 60 requests/min/IP. | [v1] |
| NFR-2.3.3 | Unauthenticated rate limit: 20 req/min/IP; authenticated rate limit: 300 req/min/user. | [v2+] |
| NFR-2.3.4 | Artifact polling is debounced (max 1 pull per repo per 5 minutes). | [v1] |
| NFR-2.3.5 | GitHub API circuit breaker triggers on consecutive failures. | [v1] |

### 2.4 Scalability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.4.1 | SQLite WAL mode for dashboard concurrent reads. | [v1] |
| NFR-2.4.2 | Staggered artifact polling across repos to avoid thundering herd. | [v1] |
| NFR-2.4.3 | Adaptive polling: inactive repos are polled less frequently. | [v2+] |

### 2.5 Deployment

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.5.1 | CLI is a single Go binary, cross-compiled for linux/darwin/windows on amd64 and arm64. | [v1] |
| NFR-2.5.2 | CLI distributed via GitHub Releases as a direct download. | [v1] |
| NFR-2.5.3 | CLI Docker image published with usage instructions. | [v1] |
| NFR-2.5.4 | Dashboard ships as a single Docker container (`docker run quarantine-dash`) as a convenience; Docker is not required. | [v1] |

### 2.6 Compatibility

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.6.1 | CI-provider agnostic: works in any CI environment that can run a binary. | [v1] |
| NFR-2.6.2 | Native GitHub Actions integration for artifacts and cache. | [v1] |
| NFR-2.6.3 | Supports JUnit XML output from any test framework. | [v1] |

---

## 3. Constraints

| ID | Constraint |
|----|-----------|
| C-1 | v1 target customer is the user's employer (enterprise environment). |
| C-2 | v1 CI support: GitHub Actions only for artifact/cache features; the CLI itself runs in any CI. |
| C-3 | v1 ticket tracker: GitHub Issues only. |
| C-4 | Primary project goal: learning and resume portfolio. |
| C-5 | Secondary project goal: potential monetization. |
