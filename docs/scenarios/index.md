# User Scenarios

All Given-When-Then scenarios for the Quarantine project, organized by topic.
Each v1 scenario is tagged with the milestone where it first becomes testable.

## v1 Scenarios

| Section | Milestones |
|---------|------------|
| [Initialization](v1/01-initialization.md) | M1 | 
| [Configuration Validation](v1/02-configuration.md) | M1
| [Core Flows](v1/03-core-flows.md) | M2–M5
| [Concurrency](v1/04-concurrency.md) | M4–M5
| [Degraded Mode](v1/05-degraded-mode.md) | M4, M6
| [Dashboard](v1/06-dashboard.md) | M6–M7
| [Branch Protection](v1/07-branch-protection.md) | M4
| [CLI Flags & Configuration](v1/08-cli-flags.md) | M2–M5
| [Test Runner Edge Cases](v1/09-test-runner-edge-cases.md) | M2, M4
| [GitHub API Edge Cases](v1/10-github-api-edge-cases.md) | M4–M5
| [Configuration Edge Cases](v1/11-config-edge-cases.md) | M1–M2

## v2+ Scenarios

See [v2/01-v2-scenarios.md](v2/01-v2-scenarios.md) for post-v1 scenarios including:
GitHub App, OAuth, Jira, Slack notifications, Jenkins CI, `--base-branch` for
non-GHA CI (ADR-023), and adaptive polling.
