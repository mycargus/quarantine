# Non-Functional Requirements

## 2.1 Performance

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.1.1 | CLI overhead must be less than 5 seconds added to a test run, excluding retry time. | [v1] |
| NFR-2.1.2 | Dashboard page load must be under 2 seconds. | [v1] |
| NFR-2.1.3 | System must support 50+ concurrent CI builds without conflicts. | [v1] |

## 2.2 Reliability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.2.1 | CLI must never break a build due to its own failure. | [v1] |
| NFR-2.2.2 | Graceful degradation when GitHub API or dashboard is unreachable. | [v1] |
| NFR-2.2.3 | Result ingestion must be idempotent (duplicate artifact processing is safe). | [v1] |

## 2.3 Security

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.3.1 | Dashboard is internal-only (deployed behind corporate network). | [v1] |
| NFR-2.3.2 | Reverse proxy with rate limiting at 60 requests/min/IP. | [v1] |
| NFR-2.3.3 | Unauthenticated rate limit: 20 req/min/IP; authenticated rate limit: 300 req/min/user. | [v2+] |
| NFR-2.3.4 | Artifact polling is debounced (max 1 pull per repo per 5 minutes). | [v1] |
| NFR-2.3.5 | GitHub API circuit breaker triggers on consecutive failures. | [v1] |

## 2.4 Scalability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.4.1 | SQLite WAL mode for dashboard concurrent reads. | [v1] |
| NFR-2.4.2 | Staggered artifact polling across repos to avoid thundering herd. | [v1] |
| NFR-2.4.3 | Adaptive polling: inactive repos are polled less frequently. | [v2+] |

## 2.5 Deployment

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.5.1 | CLI is a single Go binary, cross-compiled for linux/darwin/windows on amd64 and arm64. | [v1] |
| NFR-2.5.2 | CLI distributed via GitHub Releases as a direct download. | [v1] |
| NFR-2.5.3 | CLI Docker image published with usage instructions. | [v1] |
| NFR-2.5.4 | Dashboard ships as a single Docker container (`docker run quarantine-dash`) as a convenience; Docker is not required. | [v1] |

## 2.6 Compatibility

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.6.1 | CI-provider agnostic: works in any CI environment that can run a binary. | [v1] |
| NFR-2.6.2 | Native GitHub Actions integration for artifacts and cache. | [v1] |
| NFR-2.6.3 | Supports JUnit XML output from any test framework. | [v1] |
