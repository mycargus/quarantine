# User Scenarios

This document contains v2+ user scenarios. For v1 scenarios, see [`docs/scenarios/index.md`](../index.md).

---

## v2+ Flows

### Scenario 1: Code sync adapter creates PR with skip markers [v2+]

**Risk:** Quarantined RSpec tests continue consuming CI execution time because there is no mechanism to add skip markers to source code automatically (FR-1.7.1, ADR-003).

**Given** the CLI has quarantined `tests/test_payment.py::test_charge_timeout` and updated `quarantine.json`, and the code sync adapter is enabled in `quarantine.yml` with `code_sync: true`
**When** the adapter runs (triggered by the quarantine.json update)
**Then** the adapter opens a new PR titled "quarantine: skip tests/test_payment.py::test_charge_timeout" that adds a `@pytest.mark.skip(reason="Quarantined: flaky — see #42")` decorator to the `test_charge_timeout` function in `tests/test_payment.py`, referencing the GitHub Issue number, with a PR body explaining the change and linking to the issue

---

### Scenario 2: Code sync adapter removes skip markers when issue is closed [v2+]

**Risk:** Skip markers remain in source code after a test is unquarantined, permanently disabling the test even though it was fixed.

**Given** the GitHub Issue for `tests/test_payment.py::test_charge_timeout` has been closed, the test has been removed from `quarantine.json`, and the source code still contains `@pytest.mark.skip(reason="Quarantined: flaky -- see #42")`
**When** the code sync adapter detects the issue closure (via webhook or polling)
**Then** the adapter opens a new PR titled "unquarantine: re-enable tests/test_payment.py::test_charge_timeout" that removes the `@pytest.mark.skip` decorator from the test function, with a PR body noting the linked issue was resolved

---

### Scenario 3: User installs GitHub App on org [v2+]

**Risk:** Each repository requires manual PAT setup, making org-wide adoption impractical at scale.

**Given** an organization admin navigates to the Quarantine GitHub App installation page
**When** the admin installs the GitHub App on the organization, granting access to all repositories (or a selected subset)
**Then** the GitHub App sends an `installation` webhook event to the dashboard, the dashboard auto-discovers all accessible repositories, begins polling them for CI artifacts, and displays the newly discovered repositories on the org overview page within the next polling cycle

---

### Scenario 4: User logs into dashboard via GitHub OAuth [v2+]

**Risk:** The dashboard exposes test result data and repository information to unauthorized users.

**Given** the React Router v7 dashboard is deployed and configured with GitHub OAuth credentials (client ID and secret), and a developer has a GitHub account that belongs to an organization with Quarantine installed
**When** the developer clicks "Sign in with GitHub" on the dashboard login page
**Then** the dashboard redirects to GitHub's OAuth authorization page, the developer authorizes the app, GitHub redirects back to the dashboard with an auth code, the dashboard exchanges the code for an access token, creates a session for the user, and displays the org-level overview filtered to repos the user has access to

---

### Scenario 5: Slack notification when flaky test count exceeds threshold [v2+]

**Risk:** The number of quarantined tests grows unchecked because teams are not proactively alerted when thresholds are exceeded (FR-1.12.2).

**Given** `quarantine.yml` is configured with:

```yaml
notifications:
  github_pr_comment: true
  slack:
    webhook_url: https://hooks.slack.com/services/T00/B00/xxxxx
    threshold: 10
```

and the organization currently has 9 quarantined tests
**When** a CI run detects a new flaky test, bringing the total to 10
**Then** the CLI (or dashboard) sends a Slack notification to the configured webhook with a message: "Warning: 10 tests are now quarantined in acme/payments-service. Review the dashboard: https://quarantine.example.com/acme/payments-service"

---

### Scenario 6: Jenkins CI run (non-GitHub Actions environment) [v2+]

**Risk:** Teams using Jenkins cannot get dashboard analytics because artifact upload depends on GitHub Actions (FR-1.9.1).

**Given** a project uses Jenkins for CI (not GitHub Actions), the Quarantine CLI is installed on the Jenkins agent, and `quarantine.yml` is configured with `ci_provider: jenkins`
**When** the Jenkins pipeline executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and detects a flaky test
**Then** the CLI updates `quarantine.json` on the `quarantine/state` branch via the GitHub Contents API (same as GitHub Actions), creates a GitHub Issue, but instead of relying on `actions/upload-artifact` (unavailable in Jenkins), the CLI uploads results via the dashboard's HTTP API endpoint or stores them in a configured artifact backend, and the Jenkins build exits with status code 0

---

### Scenario 7: Jira ticket created instead of GitHub issue [v2+]

**Risk:** Teams using Jira for issue tracking cannot use quarantine's lifecycle management, forcing a parallel workflow in GitHub Issues (FR-1.10.1).

**Given** `quarantine.yml` is configured with:

```yaml
issue_tracker: jira
jira:
  host: https://acme.atlassian.net
  project: FLAKY
  issue_type: Bug
```

**When** the CLI detects `tests/test_payment.py::test_charge_timeout` as flaky
**Then** the CLI creates a Jira ticket in the `FLAKY` project with summary "Flaky test: tests/test_payment.py::test_charge_timeout", issue type `Bug`, a description including failure logs and retry details, and a label `quarantine`; the `quarantine.json` entry references the Jira ticket key (e.g., `FLAKY-123`) instead of a GitHub Issue number; when `FLAKY-123` is transitioned to Done, the test is unquarantined on the next CI run

---

### Scenario 8: Periodic flaky test report generated [v2+]

**Risk:** Stakeholders lack periodic visibility into test quality trends, making it impossible to measure or communicate the impact of test reliability efforts (FR-1.12.5).

**Given** `quarantine.yml` is configured with:

```yaml
reports:
  schedule: weekly
  email: team-leads@acme.com
```

and the dashboard has accumulated flaky test data over the past week
**When** the scheduled report trigger fires (e.g., every Monday at 9:00 AM UTC)
**Then** the dashboard generates a report summarizing: total quarantined tests (new this week, resolved this week, still active), longest-quarantined tests, most frequently flaky tests, and trend data (improving or worsening); the report is emailed to `team-leads@acme.com` with a link to the full dashboard view

---

### Scenario 9: Dashboard handles stale or inactive repos [v2+]

**Risk:** Polling inactive repositories at the same frequency as active ones wastes GitHub API rate limit budget, reducing capacity for repos that need it (ADR-015).

**Given** the dashboard is configured to poll 10 repositories, but 3 of them have had no CI activity in over 30 days
**When** a background polling cycle runs
**Then** the dashboard uses adaptive polling to reduce the frequency for the 3 inactive repos (e.g., polling once per hour instead of every 5 minutes), continues polling the 7 active repos at the normal 5-minute interval, and if an inactive repo produces a new artifact, the dashboard detects it on the next (less frequent) poll and resumes normal polling frequency for that repo
