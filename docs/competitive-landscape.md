# Competitive Landscape

## Overview

This document maps the competitive landscape for Quarantine, a CLI tool that automatically detects, quarantines, and tracks flaky tests in CI pipelines. The analysis covers direct competitors (tools with a quarantine model), adjacent competitors (monitoring/analytics without true quarantine), CI-native features, declining open-source tools, and framework-level retry capabilities. The goal is to identify where Quarantine fits in the market and where its architectural choices create defensible advantages. This document is current as of March 2026.

---

## Competitive Matrix

| Capability | **Quarantine** | **Trunk** | **Captain (RWX)** | **BuildPulse** | **Datadog** | **Buildkite TE** |
|---|---|---|---|---|---|---|
| Flaky detection (auto) | Yes | Yes | Yes | Yes | Yes | Yes |
| Runtime quarantine | Yes | Yes | Yes | Yes | No | No (manual) |
| Dashboard | Yes (self-hosted) | Yes (SaaS) | Yes (Pro tier) | Yes (SaaS) | Yes (SaaS) | Yes (SaaS) |
| GitHub issue creation | Yes | No (Jira/Linear) | No | No | No | No |
| CLI wrapper model | Yes | Partial | Yes | No | No | No |
| Zero code changes | Yes | No (runtime wrapper) | Yes | Unclear | No (SDK) | No (collector) |
| Self-hostable | Yes | No | No | No | No | No |
| GitHub-native state | Yes | No | No | No | No | No |
| JUnit XML ingestion | Yes | Yes | Unclear | Likely | No | Yes |
| Framework plugins needed | No | Optional (RSpec) | No | No | Yes (per-language SDK) | Optional |
| PR comments | Planned | Yes | No | No | No | No |
| Slack/chat alerts | Planned | Yes | No | No | Yes | Yes (digest) |
| AI failure grouping | No | Yes | No | No | No | No |
| Test partitioning | No | No | Yes | No | No | No |
| Free tier | Yes | Yes (5 committers) | Yes (file-based) | Unknown | No | Yes |
| Open source | Yes | No | Yes (core) | No | No | No |
| CI providers (v1) | GitHub Actions | GHA, CircleCI, Buildkite, Jenkins, GitLab, Semaphore, Harness | Multiple | GHA, CircleCI, Semaphore, BitBucket, Travis | Multiple | Buildkite, GHA, Jenkins, CircleCI |
| Frameworks (v1) | RSpec, Jest, Vitest | 30+ | 14+ | 8 | Many (via SDK) | 5+ |

---

## Direct Competitors

### 1. Trunk Flaky Tests

**Website:** trunk.io/flaky-tests
**Position:** Market leader in flaky test management. Most feature-complete product in the space.

**What they do:**
Trunk provides detection, quarantine, tracking, a dashboard, ticketing integrations (Jira, Linear), PR comments, and Slack alerts. Their standout feature is AI-powered failure grouping that recognizes different manifestations of the same underlying flaky failure. They also distinguish environmental failures (CI infra issues, network timeouts) from genuine test flakiness.

**Integration model:**
Test results are uploaded via CLI or CI provider plugins. Quarantine operates at runtime -- tests still execute but failures are non-blocking on merges. They also offer an RSpec plugin for deeper integration. This is more complex than a simple JUnit XML upload.

**Framework and CI support:**
Over 30 frameworks including Jest, Mocha, Cypress, Playwright, Pytest, RSpec, Go, Rust, GoogleTest, PHPUnit, minitest, Vitest, Karma, Jasmine, Nightwatch, Robot Framework, Behave, Gradle, Maven, Kotest, XCTest, Swift Testing, Dart, Bazel, NUnit, and Pest. CI support spans GitHub Actions, CircleCI, Buildkite, Jenkins, GitLab, Semaphore, and Harness.

**Pricing:**
- Free: 5 committers, 5M test spans/month
- Team: $0/committer but usage-based (1M spans/committer/month included)
- Enterprise: custom

**Customers:** Zillow, Metabase, and others.

**Quarantine's advantages over Trunk:**
- Zero code changes required (Trunk needs a runtime wrapper for quarantine)
- Self-hostable (Trunk is SaaS-only)
- GitHub-native state storage (no external DB dependency)
- Simpler integration model ("prefix your command" vs. SDK/plugin setup)
- GitHub issue creation for tracking (Trunk targets Jira/Linear)

**Trunk's advantages over Quarantine:**
- AI-powered failure grouping
- Environmental failure detection
- Breadth of framework and CI support (30+ frameworks, 7+ CI providers)
- PR comments, Slack alerts, Jira/Linear integration
- Established customer base and market presence

---

### 2. Captain by RWX

**Website:** rwx.com/captain
**Position:** Closest architectural analog to Quarantine. Open-source CLI wrapper model.

**What they do:**
Captain is an open-source CLI that detects flaky tests, quarantines them, retries only failed tests, and partitions tests for parallel execution. The CLI wrapper approach is architecturally similar to Quarantine's `quarantine run -- <test command>` model.

**Integration model:**
CLI wrapper around test commands. The free/OSS tier uses file-based configuration only (no UI). The Pro tier adds a dashboard for quarantine management.

**Framework support:**
14+ frameworks: Jest, pytest, RSpec, Playwright, Cypress, Vitest, Go test, PHPUnit, Mocha, ExUnit, minitest, Karma, Cucumber, Ginkgo.

**Pricing:**
- Free/OSS: file-based config, basic partitioning
- Pro: $10/million test results, UI quarantine management, 90-day retention
- Enterprise: custom

**Quarantine's advantages over Captain:**
- GitHub-native state storage (no external DB)
- Self-hosted dashboard included at no cost
- Automatic GitHub issue creation for tracking
- Simpler architecture (JUnit XML parsing, no framework-specific integration)

**Captain's advantages over Quarantine:**
- Test partitioning for parallel execution
- Broader framework support (14+ vs. 3 at v1)
- Established open-source community
- More mature product

---

### 3. BuildPulse

**Website:** buildpulse.io
**Position:** SaaS flaky test platform with detection, quarantine, and root cause analysis.

**What they do:**
BuildPulse detects flaky tests, quarantines them, provides reporting and root cause analysis, and offers a dashboard. They also offer BuildPulse Runners for CI infrastructure optimization.

**Integration model:**
Ingests test results (likely JUnit XML). Details of the quarantine mechanism are less publicly documented than Trunk or Captain.

**Framework support:** Cypress, Go, Jest, Minitest, Mocha, PHPUnit, Pytest, RSpec (8 frameworks).

**CI support:** GitHub Actions, CircleCI, Semaphore, BitBucket Pipelines, Travis CI.

**Pricing:** Not publicly disclosed. This opacity is a competitive disadvantage.

**Quarantine's advantages over BuildPulse:**
- Transparent pricing
- Self-hostable
- GitHub-native architecture
- Open source

**BuildPulse's advantages over Quarantine:**
- Root cause analysis features
- More CI provider support
- Established product

---

## Adjacent Competitors

These tools detect or monitor flaky tests but do not provide true runtime quarantine (making flaky failures non-blocking automatically).

### Datadog Test Optimization (formerly Test Visibility)

Part of Datadog's observability platform. Provides test performance monitoring, flaky test detection, and test impact analysis (running only tests affected by code changes). Integration requires per-language SDK instrumentation. Pricing is usage-based within Datadog's broader model and becomes expensive at scale. Covers major frameworks across Java, Python, JavaScript, .NET, Ruby, Go, and Swift. **No quarantine mechanism** -- detection and monitoring only. Requires an existing Datadog subscription.

### Buildkite Test Engine (formerly Test Analytics)

Flaky test detection, test ownership via CODEOWNERS, reliability metrics, performance timing, and automated digest reports. Tests can be marked "Disabled" or "Flaky" but this is a manual process, not runtime quarantine. Integrates via JUnit XML upload or dedicated collectors for RSpec, Jest, Cypress, pytest, and Swift. Works with Buildkite, GitHub Actions, Jenkins, and CircleCI. Uses P90 billing (measures daily managed test executions, excludes top 10% usage days). Free plan available.

### CloudBees Smart Tests (formerly Launchable)

AI-driven test selection claiming 80% test time reduction, with flaky test categorization. Originally Launchable, acquired by CloudBees and rebranded. Customers include Salesforce, Adobe, and Capital One. Primarily a test selection/prioritization tool, not a quarantine solution. Now buried inside the CloudBees enterprise platform, reducing accessibility.

### Develocity (formerly Gradle Enterprise)

Build scan analytics, predictive test selection, flaky test detection, test retry, and test distribution. Available as a Gradle/Maven plugin, limited to the JVM ecosystem. Enterprise licensing makes it expensive. Overkill if the primary need is flaky test management.

---

## CI-Native Features

| CI Platform | Flaky Detection | Quarantine/Muting | Test-Level Retry | Notes |
|---|---|---|---|---|
| **TeamCity** | Yes (flip rate, same-revision, multi-outcome) | Yes (muting) | No | Best built-in support. Investigation assignment, auto-unmute policies. TeamCity-only. |
| **GitLab CI** | Yes (JUnit XML parsing across pipelines) | No | No | Detection and tracking only. |
| **CircleCI** | No | No | Partial (selective rerun of failed files) | Manual trigger, no auto-detection. |
| **GitHub Actions** | No | No | No | Zero native flaky test features. Manual "Re-run failed jobs" only. No test-level rerun. |

**Key insight:** GitHub Actions, the largest CI platform for open-source and a major platform for private repositories, has zero native flaky test features. This is the primary market gap Quarantine targets.

---

## Dead and Declining Tools

These tools are relevant context because they created the category vocabulary and demonstrate both demand for the problem and the failure modes of prior approaches.

| Tool | Status | Why It Matters |
|---|---|---|
| **Flexport quarantine gem** | Low activity, 65 stars | The direct inspiration for Quarantine. RSpec-only, requires external DB (DynamoDB/Google Sheets), depends on rspec-retry (now archived). Demonstrates demand in the Ruby ecosystem but failed to generalize beyond RSpec or simplify infrastructure requirements. |
| **rspec-retry** | Archived July 2025 by NoRedInk | 593 stars. The Ruby community's standard retry mechanism is now unmaintained. NoRedInk moved away from Ruby entirely. Creates an acute gap for RSpec users. |
| **Google FlakyBot** | Deprecated, shutdown planned August 2025 | Opened/closed GitHub issues for failing tests. Validates the GitHub-issue-based tracking approach but was limited to detection without quarantine. |
| **pytest-quarantine** | Last release November 2019 | Saved failing tests to file, marked as xfail. Validates the "quarantine" concept for Python but unmaintained. |
| **Jenkins Flaky Test Handler** | 0.43% install rate, security vulnerabilities | Reruns via Maven Surefire. Demonstrates minimal adoption even within Jenkins ecosystem. |

The pattern: demand exists for flaky test quarantine, but prior solutions either (a) coupled too tightly to a single framework, (b) required complex infrastructure, or (c) were abandoned by their maintainers. Quarantine's framework-agnostic, infrastructure-light design addresses all three failure modes.

---

## Framework Retry Features

Native retry prevents a test from failing the build on transient failures but does not track, quarantine, or provide visibility into flakiness patterns.

| Framework | Native Retry | Flaky Detection | Quarantine | Notes |
|---|---|---|---|---|
| **Playwright** | Yes (`--retries=N`) | Yes (built-in, marks flaky in report) | No | Best native support of any framework. |
| **Cypress** | Yes (`retries` config) | Yes (via Cypress Cloud, flaky badge) | No | Detection requires paid Cypress Cloud. |
| **Vitest** | Yes (`--retry=N`) | No | No | |
| **RSpec** | Via rspec-retry (ARCHIVED July 2025) | No | No | Primary retry mechanism is now dead. |
| **pytest** | Via pytest-rerunfailures plugin | No | No | |
| **Jest** | No native retry | No | No | |
| **Go test** | No (`-count=N` runs all tests) | No | No | |
| **JUnit 5** | Via `@RepeatedTest` or extensions | No | No | |

**Key insight:** No test framework provides quarantine. Even frameworks with good retry support (Playwright, Cypress) do not track flakiness over time or prevent known-flaky tests from blocking CI. This is the layer Quarantine operates at -- above the framework, below the CI system.

---

## Competitive Gaps and Quarantine's Positioning

### Gap 1: GitHub Actions has zero native flaky test features

GitHub Actions is the largest CI platform for open source and a dominant platform for private repositories. It offers no flaky test detection, no quarantine, no test-level retry, and no test analytics. Users can only manually re-run entire failed jobs. Quarantine targets this underserved platform first with a GitHub-native architecture (state on a branch, results in GitHub Artifacts, issues via GitHub API).

### Gap 2: The Ruby/RSpec ecosystem is unserved

The archival of rspec-retry in July 2025 and the stagnation of the Flexport quarantine gem leave the RSpec community without a maintained flaky test solution. Quarantine's v1 targets RSpec as a first-class framework, filling this specific gap.

### Gap 3: Simplicity of integration

Trunk requires a runtime wrapper or framework plugin for quarantine. Datadog requires per-language SDKs. Buildkite requires collectors. Captain is the closest competitor on simplicity, but Quarantine's model -- `quarantine run -- <your test command>` with zero code changes, zero plugins, and zero configuration files -- is the simplest integration path in the market.

### Gap 4: No self-hosted option exists

Trunk, Captain (Pro), BuildPulse, and Datadog are all SaaS-only for their dashboard and quarantine management features. Enterprise customers with data residency requirements or security policies prohibiting third-party test data transmission have no option today. Quarantine's self-hosted dashboard (React Router v7 + SQLite) and GitHub-native state storage (branch + artifacts) mean all data stays within the customer's own infrastructure.

### Gap 5: GitHub-native architecture is unique

No competitor stores quarantine state on a Git branch, test results in GitHub Artifacts, or creates GitHub Issues for flaky test tracking. This architecture eliminates external database dependencies and aligns with the workflows GitHub-centric teams already use.

### Gap 6: No tool combines retry + quarantine + ticketing + dashboard simply

Individual tools cover subsets of this stack. Framework retry handles retries. Trunk handles quarantine and dashboards but uses Jira/Linear for ticketing. Google FlakyBot handled GitHub issues but not quarantine. Quarantine combines all four in a single CLI tool with a single integration point.

### Gap 7: Pricing transparency

BuildPulse does not publish pricing. Datadog's usage-based model is notoriously difficult to predict. CloudBees and Develocity use enterprise sales motions. Transparent, predictable pricing is a competitive advantage for developer tools.

---

## Key Takeaways

1. **The primary competitive threat is Trunk.** They are the market leader with the most features, broadest framework support, and established customers. Quarantine competes on simplicity (zero code changes), self-hosting, and GitHub-native architecture -- not on feature breadth.

2. **Captain by RWX is the closest architectural competitor.** The CLI wrapper model is shared, but Quarantine differentiates on GitHub-native state storage, self-hosted dashboard, and GitHub issue integration. Captain's test partitioning feature is a capability Quarantine does not currently address.

3. **GitHub Actions is the strategic beachhead.** As the largest CI platform with zero native flaky test features, GitHub Actions users are the most underserved audience. Quarantine's GitHub-native design makes it the natural choice for teams already centered on GitHub.

4. **The dead open-source ecosystem validates demand and informs design.** Flexport's gem, rspec-retry, and pytest-quarantine all demonstrate that developers want flaky test quarantine but prior tools failed by coupling to single frameworks or requiring complex infrastructure. Quarantine's framework-agnostic, infrastructure-light approach directly addresses these historical failure modes.

5. **Self-hosting is an uncontested position.** No competitor offers a self-hosted quarantine solution with a dashboard. For enterprise customers with data residency or security requirements, Quarantine is currently the only option.
