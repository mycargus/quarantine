package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 182: quarantine run — Jenkins with explicit --pr flag ---
//
// Per ADR-037 (Scenario 182), when `quarantine run backend --pr 42` runs in a
// non-GitHub-Actions context (Jenkins) where `GITHUB_EVENT_PATH` is unset, the
// CLI MUST resolve the PR number from the `--pr` flag, complete flaky detection
// + state update + issue creation normally, and post a PR comment to PR #42 on
// `github.com/{owner}/{repo}` with the quarantine summary.
//
// The Gerrit origin proves run does not depend on a GitHub-style git remote,
// matching the Jenkins / non-GitHub-origin scenario.

// fakeJenkinsPRFlagAPI is a mock GitHub API server scoped to my-org/my-project.
// It tracks POSTs to /repos/my-org/my-project/issues/{prNumber}/comments and
// captures the most recent comment body for assertion. Requests outside the
// expected endpoints fall through to 404 so unexpected paths surface clearly.
func fakeJenkinsPRFlagAPI(
	t *testing.T,
	prNumber int,
	prCommentPostCount *int32,
	lastCommentBody *atomic.Value,
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Branch exists check: GET /repos/my-org/my-project/git/ref/heads/quarantine/state → 200, sha=abc123
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/quarantine/state",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		})

	// State read/write: empty state on GET (404), succeed on PUT (CAS write).
	mux.HandleFunc("/repos/my-org/my-project/contents/.quarantine/backend/state.json",
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusNotFound)
			case http.MethodPut:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"content":{"sha":"def456"}}`)
			default:
				http.NotFound(w, r)
			}
		})

	// Closed-issue search (unquarantine check): no closed issues.
	// Open-issue search (dedup): no existing open issue.
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	})

	// POST /repos/my-org/my-project/issues — create issue for new flaky test.
	mux.HandleFunc("/repos/my-org/my-project/issues",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"number":201,"html_url":"https://github.com/my-org/my-project/issues/201"}`)
		})

	// PR comments: GET → empty list (no existing comment); POST → 201, track body.
	prCommentsPath := fmt.Sprintf("/repos/my-org/my-project/issues/%d/comments", prNumber)
	mux.HandleFunc(prCommentsPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]interface{}{})
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			if bodyStr, ok := req["body"].(string); ok {
				lastCommentBody.Store(bodyStr)
			}
			atomic.AddInt32(prCommentPostCount, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id":999}`)
		default:
			http.NotFound(w, r)
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestRunJenkinsPRFlagPostsCommentWithoutEventPath(t *testing.T) {
	dir := t.TempDir()

	// Gerrit origin demonstrates a non-GitHub-Actions / Jenkins context.
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")

	// Synthetic JUnit XML that reports one failed test. Combined with a passing
	// rerun script, this produces a flaky-test detection.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="rspec" tests="1" failures="1" errors="0" time="1.0">
  <testsuite name="spec/orders_spec.rb" tests="1" failures="1" time="1.0">
    <testcase classname="OrdersService" name="creates an order"
              file="spec/orders_spec.rb" time="0.5">
      <failure message="intermittent timeout" type="Error">intermittent timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	initialScript := writeTestScript(t, dir, xmlPath, failXML, 1)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	// Pre-create config with valid github.owner / github.repo and a backend
	// suite with retries: 3, matching the scenario.
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configContent := fmt.Sprintf(`version: 1
github:
  owner: my-org
  repo: my-project
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s"]
    retries: 3
`, initialScript, xmlPath, rerunScript)
	if err := os.WriteFile(filepath.Join(suiteDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	prNumber := 42
	var prCommentPostCount int32
	var lastCommentBody atomic.Value
	server := fakeJenkinsPRFlagAPI(t, prNumber, &prCommentPostCount, &lastCommentBody)

	output, err := executeRunCmd(t, []string{
		"--pr", fmt.Sprintf("%d", prNumber),
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "", // Jenkins context: no event payload
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Jenkins run with --pr 42, no GITHUB_EVENT_PATH, and a flaky test detected",
		Should:   "exit with code 0 (flaky only, no genuine failures)",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "--pr 42 supplies the PR number without GITHUB_EVENT_PATH",
		Should:   "post exactly one PR comment to PR #42",
		Actual:   atomic.LoadInt32(&prCommentPostCount),
		Expected: 1,
	})

	body, _ := lastCommentBody.Load().(string)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the PR comment posted to PR #42",
		Should:   "include the quarantine summary header",
		Actual:   strings.Contains(body, "## Quarantine Summary"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the PR comment posted to PR #42",
		Should:   "include the suite-specific marker for the backend suite",
		Actual:   strings.Contains(body, suitePRCommentMarker("backend")),
		Expected: true,
	})

	// Sanity: run must not have degraded (degraded mode skips PR comment posting,
	// which would also fail the count assertion above, but this gives a clearer
	// failure signal if the API base URL is wrong).
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jenkins run with valid config and explicit --pr flag",
		Should:   "not degrade due to a missing GitHub URL",
		Actual:   strings.Contains(output, "not a GitHub URL"),
		Expected: false,
	})
}
