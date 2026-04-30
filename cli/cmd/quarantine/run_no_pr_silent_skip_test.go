package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 183: quarantine run — Jenkins, no PR number available ---
//
// Per ADR-037 (Scenario 183), when a Jenkins branch build invokes
// `quarantine run backend` with valid config + token but neither
// `GITHUB_EVENT_PATH` nor `--pr` supplies a PR number, the CLI MUST:
//   1. Complete flaky detection, state update, and issue creation normally.
//   2. Silently skip PR comment posting (no warning, no error).
//   3. Write `.quarantine/backend/results.json` to disk.
//   4. Exit 0 (one flaky test detected, no genuine failures).
//
// Scenario 176 already proves the run reads owner/repo from config and posts
// zero PR comments. This scenario is the complementary regression: it
// specifically asserts the skip is silent — no message about a missing PR
// number is emitted to stdout/stderr. If a future change adds a misguided
// warning ("[quarantine] WARNING: no PR number…"), this test will fail.

// fakeNoPRSilentSkipAPI is a mock GitHub API server scoped to my-org/my-project.
// It supports the full flaky-detection + issue-creation flow, and tracks any
// POST to `/repos/.../issues/{N}/comments` so the test can assert that no PR
// comment was attempted.
//
// commentPostCount counts every POST to any `…/comments` path under
// `/repos/my-org/my-project/issues/`, regardless of issue number, so a future
// regression that picks an arbitrary PR number would still be caught.
func fakeNoPRSilentSkipAPI(t *testing.T, commentPostCount *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Branch exists check: GET /repos/my-org/my-project/git/ref/heads/quarantine/state → 200
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

	// Closed-issue search (unquarantine check) and open-issue search (dedup):
	// no matching issues — the flaky test is brand new.
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

	// POST /repos/my-org/my-project/issues — issue creation succeeds (the flaky
	// test is new, so the run will create one).
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

	// Catch-all under /repos/my-org/my-project/issues/ — any POST to a `…/comments`
	// path increments the counter. The expected behavior is zero such posts.
	mux.HandleFunc("/repos/my-org/my-project/issues/",
		func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/comments") && r.Method == http.MethodPost {
				atomic.AddInt32(commentPostCount, 1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"id":1,"body":"comment"}`)
				return
			}
			http.NotFound(w, r)
		})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestRunSilentlySkipsPRCommentWhenNoPRNumberAvailable(t *testing.T) {
	dir := t.TempDir()

	// Gerrit origin demonstrates a non-GitHub-Actions / Jenkins context: the
	// run must not depend on the git remote being a GitHub URL.
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")

	// Synthetic JUnit XML reporting one failed test; combined with a passing
	// rerun script, this drives the flaky-detection path (one detected flaky
	// test, retries up to 3).
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
	// suite with retries: 3, matching scenario 182's flaky-test pattern.
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

	var commentPostCount int32
	server := fakeNoPRSilentSkipAPI(t, &commentPostCount)

	resultsPath := filepath.Join(dir, ".quarantine", "backend", "results.json")
	output, err := executeRunCmd(t, []string{
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "", // Jenkins context: no event payload
		// No --pr flag passed in args → prNumber resolves to 0 → silent skip.
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Jenkins run with no GITHUB_EVENT_PATH and no --pr flag, one flaky test detected",
		Should:   "exit with code 0 (flaky only, no genuine failures)",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "neither GITHUB_EVENT_PATH nor --pr supplies a PR number",
		Should:   "post zero PR comments (silent skip)",
		Actual:   atomic.LoadInt32(&commentPostCount),
		Expected: 0,
	})

	_, statErr := os.Stat(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Jenkins run completes flaky detection without a PR number",
		Should:   "write .quarantine/backend/results.json to disk",
		Actual:   statErr == nil,
		Expected: true,
	})

	// Silent-skip assertions: the run must not warn or error about the missing
	// PR number. Each forbidden substring is asserted independently so a
	// regression failure points at the exact phrase that was emitted.
	forbiddenPhrases := []string{
		"PR number",
		"no PR",
		"missing PR",
		"GITHUB_EVENT_PATH",
		"--pr flag",
	}
	for _, phrase := range forbiddenPhrases {
		riteway.Assert(t, riteway.Case[bool]{
			Given:    fmt.Sprintf("a Jenkins run with no PR number available (forbidden phrase: %q)", phrase),
			Should:   "not emit any warning/error mentioning the missing PR",
			Actual:   strings.Contains(output, phrase),
			Expected: false,
		})
	}
}
