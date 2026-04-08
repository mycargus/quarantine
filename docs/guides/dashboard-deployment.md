# Deploying the Dashboard

This guide covers configuring and deploying the Quarantine dashboard for
production use.

**Who is this for?** Operators deploying the dashboard for their team or
organization.

**Prerequisites:** The Quarantine CLI is already integrated into CI. You want
a web dashboard for test stability trends and cross-repo analytics.

---

## Overview

The dashboard is a Remix 3 (TypeScript) application backed by SQLite. It
discovers test results by polling GitHub Artifacts and stores historical data
locally. The dashboard is read-only -- it never writes to GitHub.

**Two operating modes:**

| Mode | Config | Auth | Repo discovery |
|------|--------|------|----------------|
| `source: manual` (v1) | List repos in `dashboard.yml` | PAT via env var | Manual |
| `source: github-app` (v2) | Install the App | OAuth + installation tokens | Automatic |

---

## 1. Configuration

### dashboard.yml

The dashboard reads `dashboard.yml` from the working directory on startup.

**Manual mode (v1):**

```yaml
source: manual
repos:
  - owner: my-org
    repo: my-project
  - owner: my-org
    repo: another-project
```

**App mode (v2):**

```yaml
source: github-app
```

In App mode, repos are discovered automatically from the App's installations.
The `repos` array is silently ignored if present.

### Environment variables

**v1 (manual mode):**

| Env Var | Required | Description |
|---------|----------|-------------|
| `QUARANTINE_GITHUB_TOKEN` | Yes | PAT with `repo` + `actions:read` scopes |
| `PORT` | No | HTTP listen port (default: `3000`) |

**v2 (App mode) -- additional variables:**

| Env Var | Required | Description |
|---------|----------|-------------|
| `QUARANTINE_APP_CLIENT_ID` | Yes | App client ID (JWT `iss` + OAuth client ID) |
| `QUARANTINE_APP_CLIENT_SECRET` | Yes | App client secret (OAuth token exchange) |
| `QUARANTINE_APP_PRIVATE_KEY_PATH` | Yes* | Path to PEM file |
| `QUARANTINE_APP_PRIVATE_KEY` | Yes* | PEM contents as env var value |
| `QUARANTINE_APP_ORIGIN` | Yes | Dashboard origin for OAuth redirect URI |

\* One of `QUARANTINE_APP_PRIVATE_KEY_PATH` or `QUARANTINE_APP_PRIVATE_KEY`
is required.

See the [GitHub App Setup Guide](github-app-setup.md) for App registration
and credential management.

---

## 2. Startup Behavior

### Manual mode

1. Parse `dashboard.yml`, validate config
2. Start HTTP server
3. Begin artifact polling (every 5 min per repo, staggered)

### App mode

1. Parse `dashboard.yml`, validate config
2. Validate App credentials -- missing/invalid credentials cause immediate exit
   with a descriptive error
3. **Startup sync (blocking):** call `GET /app/installations` and
   `GET /installation/repositories` per installation (paginated), populate
   `installations` and `projects` tables
4. Start HTTP server (traffic is NOT served until sync completes)
5. Begin artifact polling using installation tokens
6. Start background discovery loop (re-syncs every 15 min)

**Startup sync timeout:** If the initial sync does not complete within 60
seconds (e.g., GitHub API unresponsive), the dashboard logs a timeout error
and exits with a non-zero code. The HTTP server does not start with
incomplete data.

### Shutdown

On `SIGTERM` or `SIGINT`:

1. Background discovery loop is stopped (no further sync calls)
2. HTTP server closes gracefully
3. SQLite connections are released

---

## 3. Deployment Options

### Node.js server (recommended)

```sh
# Install dependencies
cd dashboard && pnpm install

# Start (dev)
pnpm dev

# Start (production)
node --import tsx/esm app/server.ts
```

Deploy behind a reverse proxy (nginx, Caddy, or cloud load balancer) for TLS
termination and rate limiting.

### Docker

```dockerfile
FROM node:22-slim
WORKDIR /app
COPY dashboard/ .
RUN corepack enable && pnpm install --frozen-lockfile
EXPOSE 3000
CMD ["node", "--import", "tsx/esm", "app/server.ts"]
```

Mount a volume for the SQLite database:

```sh
docker run -v quarantine-data:/app/data -p 3000:3000 \
  -e QUARANTINE_GITHUB_TOKEN=ghp_... \
  quarantine-dashboard
```

---

## 4. Data Storage

The dashboard uses SQLite in WAL mode for concurrent reads.

**Tables:**

| Table | Purpose |
|-------|---------|
| `projects` | Tracked repos (manual or App-discovered) |
| `test_runs` | One row per CLI run (from artifact ingestion) |
| `quarantined_tests` | Current and historical quarantine state |
| `installations` (v2) | GitHub App installations |

**Persistence:** The SQLite database file is the only stateful artifact. Back
it up if you need historical data. Losing it means the dashboard re-ingests
from GitHub Artifacts on next poll (artifacts expire after 90 days).

**Migrations:** Schema migrations run automatically on server startup in
`initDb()`. No manual migration step is needed.

---

## 5. Monitoring

### Health endpoint

`GET /health` returns `200 OK` without authentication. Use it for load
balancer health checks and monitoring probes.

In App mode (v2), the health endpoint is accessible even when the user is not
authenticated -- it is explicitly excluded from route protection.

### Artifact polling

The dashboard logs each poll cycle:

- Repos polled and artifacts found
- Conditional request results (304 Not Modified = no new data)
- Ingestion errors (malformed JSON, schema validation failures)
- Rate limit warnings (when `X-RateLimit-Remaining` < 20% of limit)

### Installation discovery (v2)

The dashboard logs discovery results every 15 minutes:

- Installations found
- Repos added/removed
- Suspended or removed installations
- Sync errors

### Rate limiting (v2)

Two layers of rate limiting protect the dashboard:

| Layer | Scope | Limit | When |
|-------|-------|-------|------|
| IP-based | Per source IP | 20 req/min | Before auth resolution |
| User-based | Per authenticated user | 300 req/min | After auth resolution |

Both use fixed-window counters (60-second windows). Rate-limited responses
return HTTP 429 with a `Retry-After` header (seconds until window reset).

---

## 6. Security

### v1 (internal only)

The dashboard is designed for deployment behind a corporate network. No
authentication is built in -- access control is handled by the network layer.

### v2 (public with OAuth)

GitHub OAuth login via `@remix-run/auth`. All routes except `/auth/login`,
`/auth/github/callback`, `/auth/logout`, and `/health` return 401 when
unauthenticated.

Session state is stored entirely in an encrypted cookie (`httpOnly`, `secure`,
`SameSite=Lax`, `Max-Age: 28800`). No server-side session table. When the
cookie expires (8 hours), the user re-authenticates via OAuth.

Users see only repos they have access to via GitHub permissions (filtered by
`GET /user/installations` and `GET /user/installations/{id}/repositories`).

---

*References: [architecture.md](../specs/architecture.md) (system design),
[GitHub App Setup Guide](github-app-setup.md) (App configuration),
[error-handling.md](../specs/error-handling.md) (degradation strategy).*
