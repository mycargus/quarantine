# User Scenarios

All Given-When-Then scenarios for the Quarantine project, organized by topic.
Each v1 scenario is tagged with the milestone where it first becomes testable.

## v1 Scenarios

| Section | Milestones |
|---------|------------|
| [Initialization](v1/01-initialization.md) | M1 |
| [Configuration Validation](v1/02-configuration.md) | M1 |
| [Core Flows](v1/03-core-flows.md) | M2–M8 |
| [Concurrency](v1/04-concurrency.md) | M4–M5 |
| [Degraded Mode](v1/05-degraded-mode.md) | M4, M6 |
| [Dashboard](v1/06-dashboard.md) | M6–M7 |
| [Branch Protection](v1/07-branch-protection.md) | M4 |
| [CLI Flags & Configuration](v1/08-cli-flags.md) | M2–M8 |
| [Test Runner Edge Cases](v1/09-test-runner-edge-cases.md) | M2–M8 |
| [GitHub API Edge Cases](v1/10-github-api-edge-cases.md) | M4–M8 |
| [Configuration Edge Cases](v1/11-config-edge-cases.md) | M1–M2 |
| [Schema Validation](v1/12-schema-validation.md) | M8 |
| [Parameterized Tests](v1/13-parameterized-tests.md) | M8 |
| [Multi-Suite Initialization](v1/14-multi-suite-init.md) | M9 |
| [Multi-Suite Run & Errors](v1/15-multi-suite-run.md) | M9 (incl. post-execution reclassification 148–152) |
| [Suite Management](v1/17-suite-management.md) | M10 |
| [Quarantine Status](v1/18-quarantine-status.md) | M10 |
| [Timeout Enforcement](v1/19-timeouts.md) | M10 |
| [Quarantined Files List](v1/20-quarantined-files.md) | M10 |
| [Jenkins / Non-GitHub-Origin Support](v1/24-jenkins-non-github-origin.md) | M20 |

## v2+ Scenarios

| File | Topics | Milestones |
|------|--------|------------|
| [v2/01-v2-scenarios.md](v2/01-v2-scenarios.md) | GitHub App install, OAuth, Jira, Slack, Jenkins, `--base-branch`, adaptive polling | v2+ future |
| [v2/02-github-app-auth.md](v2/02-github-app-auth.md) | JWT, installation tokens, OAuth login, installation discovery, rate limiting | M12–M16 |
