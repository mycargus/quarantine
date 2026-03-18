# User Scenarios

All Given-When-Then scenarios for the Quarantine project, organized by topic.
Each v1 scenario is tagged with the milestone where it first becomes testable.

## v1 Scenarios

### By section

| Section | Scenarios | Milestones | File |
|---------|-----------|------------|------|
| [Initialization](v1/01-initialization.md) | 1–12 | M1 | `v1/01-initialization.md` |
| [Configuration Validation](v1/02-configuration.md) | 13–18 | M1 | `v1/02-configuration.md` |
| [Core Flows](v1/03-core-flows.md) | 19–26 | M2–M4 | `v1/03-core-flows.md` |
| [Concurrency](v1/04-concurrency.md) | 27–29 | M4–M5 | `v1/04-concurrency.md` |
| [Degraded Mode](v1/05-degraded-mode.md) | 30–35 | M4, M6 | `v1/05-degraded-mode.md` |
| [Dashboard](v1/06-dashboard.md) | 36–40 | M6–M7 | `v1/06-dashboard.md` |
| [Branch Protection](v1/07-branch-protection.md) | 41–42 | M4 | `v1/07-branch-protection.md` |
| [CLI Flags & Configuration](v1/08-cli-flags.md) | 43–51 | M2–M5 | `v1/08-cli-flags.md` |
| [Test Runner Edge Cases](v1/09-test-runner-edge-cases.md) | 52–58 | M2, M4 | `v1/09-test-runner-edge-cases.md` |
| [GitHub API Edge Cases](v1/10-github-api-edge-cases.md) | 59–63 | M4–M5 | `v1/10-github-api-edge-cases.md` |
| [Configuration Edge Cases](v1/11-config-edge-cases.md) | 64–66 | M1–M2 | `v1/11-config-edge-cases.md` |

### All scenarios

| # | Title | Milestone | Section |
|---|-------|-----------|---------|
| [1](v1/01-initialization.md#scenario-1-first-time-setup-with-jest-m1) | First-time setup with Jest | M1 | Initialization |
| [2](v1/01-initialization.md#scenario-2-quarantine-init-with-rspec-m1) | quarantine init with RSpec | M1 | Initialization |
| [3](v1/01-initialization.md#scenario-3-quarantine-init-with-vitest-m1) | quarantine init with Vitest | M1 | Initialization |
| [4](v1/01-initialization.md#scenario-4-quarantine-init-when-quarantineyml-already-exists-m1) | quarantine init when quarantine.yml already exists | M1 | Initialization |
| [5](v1/01-initialization.md#scenario-5-quarantine-init-when-quarantinestate-branch-already-exists-m1) | quarantine init when quarantine/state branch already exists | M1 | Initialization |
| [6](v1/01-initialization.md#scenario-6-quarantine-init-with-no-github-token-m1) | quarantine init with no GitHub token | M1 | Initialization |
| [7](v1/01-initialization.md#scenario-7-quarantine-init-with-insufficient-token-permissions-m1) | quarantine init with insufficient token permissions | M1 | Initialization |
| [8](v1/01-initialization.md#scenario-8-quarantine-init-when-not-a-git-repository-m1) | quarantine init when not a git repository | M1 | Initialization |
| [9](v1/01-initialization.md#scenario-9-quarantine-init-with-non-github-remote-m1) | quarantine init with non-GitHub remote | M1 | Initialization |
| [10](v1/01-initialization.md#scenario-10-quarantine-init-with-invalid-framework-input-m1) | quarantine init with invalid framework input | M1 | Initialization |
| [11](v1/01-initialization.md#scenario-11-quarantine-init-with-github-api-unreachable-m1) | quarantine init with GitHub API unreachable | M1 | Initialization |
| [12](v1/01-initialization.md#scenario-12-quarantine-run-without-prior-init-m1) | quarantine run without prior init | M1 | Initialization |
| [13](v1/02-configuration.md#scenario-13-quarantine-doctor--valid-configuration-m1) | quarantine doctor — valid configuration | M1 | Configuration Validation |
| [14](v1/02-configuration.md#scenario-14-quarantine-doctor--missing-config-file-m1) | quarantine doctor — missing config file | M1 | Configuration Validation |
| [15](v1/02-configuration.md#scenario-15-quarantine-doctor--invalid-field-values-m1) | quarantine doctor — invalid field values | M1 | Configuration Validation |
| [16](v1/02-configuration.md#scenario-16-quarantine-doctor--forward-compatible-config-value-m1) | quarantine doctor — forward-compatible config value | M1 | Configuration Validation |
| [17](v1/02-configuration.md#scenario-17-quarantine-doctor--unknown-fields-m1) | quarantine doctor — unknown fields | M1 | Configuration Validation |
| [18](v1/02-configuration.md#scenario-18-quarantine-doctor--custom-config-path-m1) | quarantine doctor — custom config path | M1 | Configuration Validation |
| [19](v1/03-core-flows.md#scenario-19-normal-ci-run-with-no-flaky-tests-m2) | Normal CI run with no flaky tests | M2 | Core Flows |
| [20](v1/03-core-flows.md#scenario-20-ci-run-detects-a-new-flaky-test-m3) | CI run detects a new flaky test | M3 | Core Flows |
| [21](v1/03-core-flows.md#scenario-21-ci-run-with-a-previously-quarantined-test--jest-or-vitest-pre-execution-exclusion-m4) | CI run with a previously quarantined test — Jest or Vitest | M4 | Core Flows |
| [22](v1/03-core-flows.md#scenario-22-ci-run-with-a-previously-quarantined-test--rspec-post-execution-filtering-m4) | CI run with a previously quarantined test — RSpec | M4 | Core Flows |
| [23](v1/03-core-flows.md#scenario-23-ci-run-with-a-real-failure-m3) | CI run with a real failure | M3 | Core Flows |
| [24](v1/03-core-flows.md#scenario-24-multiple-flaky-tests-detected-in-a-single-run-m3) | Multiple flaky tests detected in a single run | M3 | Core Flows |
| [25](v1/03-core-flows.md#scenario-25-quarantined-tests-github-issue-is-closed-unquarantine-m4) | Quarantined test's GitHub issue is closed (unquarantine) | M4 | Core Flows |
| [26](v1/03-core-flows.md#scenario-26-ci-run-with-mixed-results--flaky-quarantined-real-failures-and-passes-m4) | CI run with mixed results — flaky, quarantined, real failures, and passes | M4 | Core Flows |
| [27](v1/04-concurrency.md#scenario-27-concurrent-ci-builds-detect-the-same-flaky-test-simultaneously-m5) | Concurrent CI builds detect the same flaky test simultaneously | M5 | Concurrency |
| [28](v1/04-concurrency.md#scenario-28-concurrent-ci-builds-update-quarantinejson-simultaneously-cas-conflict-m4) | Concurrent CI builds update quarantine.json simultaneously (CAS conflict) | M4 | Concurrency |
| [29](v1/04-concurrency.md#scenario-29-concurrent-quarantine-and-unquarantine-race-m4) | Concurrent quarantine and unquarantine race | M4 | Concurrency |
| [30](v1/05-degraded-mode.md#scenario-30-ci-run-when-github-api-is-unreachable-m4) | CI run when GitHub API is unreachable | M4 | Degraded Mode |
| [31](v1/05-degraded-mode.md#scenario-31-ci-run-when-dashboard-is-unreachable-m4) | CI run when dashboard is unreachable | M4 | Degraded Mode |
| [32](v1/05-degraded-mode.md#scenario-32-dashboard-reconnects-and-syncs-missed-results-from-artifacts-m6) | Dashboard reconnects and syncs missed results from artifacts | M6 | Degraded Mode |
| [33](v1/05-degraded-mode.md#scenario-33-ci-run-with-no-api-access-and-empty-cache-m4) | CI run with no API access and empty cache | M4 | Degraded Mode |
| [34](v1/05-degraded-mode.md#scenario-34-degraded-mode-with---strict-m4) | Degraded mode with --strict | M4 | Degraded Mode |
| [35](v1/05-degraded-mode.md#scenario-35-ci-run-with-no-github-token-set-m4) | CI run with no GitHub token set | M4 | Degraded Mode |
| [36](v1/06-dashboard.md#scenario-36-user-views-org-wide-flaky-test-overview-m7) | User views org-wide flaky test overview | M7 | Dashboard |
| [37](v1/06-dashboard.md#scenario-37-user-views-single-projects-flaky-test-details-and-trends-m7) | User views single project's flaky test details and trends | M7 | Dashboard |
| [38](v1/06-dashboard.md#scenario-38-user-filters-and-searches-quarantined-tests-on-dashboard-m7) | User filters and searches quarantined tests on dashboard | M7 | Dashboard |
| [39](v1/06-dashboard.md#scenario-39-dashboard-polls-artifacts-and-ingests-new-results-m6) | Dashboard polls artifacts and ingests new results | M6 | Dashboard |
| [40](v1/06-dashboard.md#scenario-40-dashboard-circuit-breaker-pauses-polling-after-failures-m6) | Dashboard circuit breaker pauses polling after failures | M6 | Dashboard |
| [41](v1/07-branch-protection.md#scenario-41-cli-updates-quarantinejson-on-unprotected-branch-m4) | CLI updates quarantine.json on unprotected branch | M4 | Branch Protection |
| [42](v1/07-branch-protection.md#scenario-42-cli-updates-quarantinejson-when-branch-is-protected-m4) | CLI updates quarantine.json when branch is protected | M4 | Branch Protection |
| [43](v1/08-cli-flags.md#scenario-43-user-overrides-framework-in-quarantineyml-m2) | User overrides framework in quarantine.yml | M2 | CLI Flags & Configuration |
| [44](v1/08-cli-flags.md#scenario-44-user-customizes-retry-count-m3) | User customizes retry count | M3 | CLI Flags & Configuration |
| [45](v1/08-cli-flags.md#scenario-45---dry-run-flag-m4) | --dry-run flag | M4 | CLI Flags & Configuration |
| [46](v1/08-cli-flags.md#scenario-46---exclude-patterns-m4) | --exclude patterns | M4 | CLI Flags & Configuration |
| [47](v1/08-cli-flags.md#scenario-47---pr-flag-override-and-auto-detection-m5) | --pr flag override and auto-detection | M5 | CLI Flags & Configuration |
| [48](v1/08-cli-flags.md#scenario-48-pr-comment-suppressed-via-config-m5) | PR comment suppressed via config | M5 | CLI Flags & Configuration |
| [49](v1/08-cli-flags.md#scenario-49-pr-comment-updated-on-second-run-m5) | PR comment updated on second run | M5 | CLI Flags & Configuration |
| [50](v1/08-cli-flags.md#scenario-50-custom-rerun_command-template-m3) | Custom rerun_command template | M3 | CLI Flags & Configuration |
| [51](v1/08-cli-flags.md#scenario-51---verbose-and---quiet-flags-m2) | --verbose and --quiet flags | M2 | CLI Flags & Configuration |
| [52](v1/09-test-runner-edge-cases.md#scenario-52-quarantine-run-without----separator-m2) | quarantine run without -- separator | M2 | Test Runner Edge Cases |
| [53](v1/09-test-runner-edge-cases.md#scenario-53-test-command-not-found-m2) | Test command not found | M2 | Test Runner Edge Cases |
| [54](v1/09-test-runner-edge-cases.md#scenario-54-no-junit-xml-produced-m2) | No JUnit XML produced | M2 | Test Runner Edge Cases |
| [55](v1/09-test-runner-edge-cases.md#scenario-55-malformed-junit-xml-m2) | Malformed JUnit XML | M2 | Test Runner Edge Cases |
| [56](v1/09-test-runner-edge-cases.md#scenario-56-multiple-xml-files-some-malformed-parallel-runners-m2) | Multiple XML files, some malformed (parallel runners) | M2 | Test Runner Edge Cases |
| [57](v1/09-test-runner-edge-cases.md#scenario-57-all-tests-in-the-suite-are-quarantined--jestvartest-m4) | All tests in the suite are quarantined — Jest/Vitest | M4 | Test Runner Edge Cases |
| [58](v1/09-test-runner-edge-cases.md#scenario-58-all-tests-in-the-suite-are-quarantined--rspec-m4) | All tests in the suite are quarantined — RSpec | M4 | Test Runner Edge Cases |
| [59](v1/10-github-api-edge-cases.md#scenario-59-search-api-result-limit-exceeded-during-unquarantine-detection-m4) | Search API result limit exceeded during unquarantine detection | M4 | GitHub API Edge Cases |
| [60](v1/10-github-api-edge-cases.md#scenario-60-rate-limit-warning-m4) | Rate limit warning | M4 | GitHub API Edge Cases |
| [61](v1/10-github-api-edge-cases.md#scenario-61-issues-disabled-on-repository-m5) | Issues disabled on repository | M5 | GitHub API Edge Cases |
| [62](v1/10-github-api-edge-cases.md#scenario-62-quarantinejson-exceeds-size-limit-m4) | quarantine.json exceeds size limit | M4 | GitHub API Edge Cases |
| [63](v1/10-github-api-edge-cases.md#scenario-63-cas-conflict-exhaustion-all-3-retries-fail-m4) | CAS conflict exhaustion (all 3 retries fail) | M4 | GitHub API Edge Cases |
| [64](v1/11-config-edge-cases.md#scenario-64-config-resolution-order-m2) | Config resolution order | M2 | Configuration Edge Cases |
| [65](v1/11-config-edge-cases.md#scenario-65-minimal-valid-config-m1) | Minimal valid config | M1 | Configuration Edge Cases |
| [66](v1/11-config-edge-cases.md#scenario-66-unsupported-config-version-m1) | Unsupported config version | M1 | Configuration Edge Cases |

## v2+ Scenarios

See [v2/01-v2-scenarios.md](v2/01-v2-scenarios.md) for post-v1 scenarios including:
GitHub App, OAuth, Jira, Slack notifications, Jenkins CI, and adaptive polling.
