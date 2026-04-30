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

// --- Scenario 176: run reads owner/repo from config; never inspects gerrit origin ---
//
// Per ADR-037, `quarantine run` must rely solely on `github.owner` and
// `github.repo` from the user's `.quarantine/config.yml`. It must NOT fall back
// to scanning the git origin URL (which fails on Gerrit / GitLab / Jenkins
// hosting and was the root cause this milestone removes).
//
// We use a Gerrit origin throughout this file to demonstrate the requirement
// explicitly. The legacy `git.ParseRemote` path returns an error for Gerrit
// URLs ("not a GitHub URL"), so if that fallback ever runs, owner/repo would
// resolve to "" / "" and the run would degrade to no-token mode — failing the
// "exit 0 + GitHub state read" assertions below.

// fakeGerritOriginGitHubAPI returns a mock GitHub API server that ONLY answers
// requests scoped to my-org/my-project. Requests to any other owner/repo path
// or to PR comment endpoints fall through to 404 and surface as test failures.
//
// commentPostCount tracks calls to POST /repos/.../issues/{N}/comments so the
// test can assert the run did not attempt to post a PR comment.
func fakeGerritOriginGitHubAPI(t *testing.T, commentPostCount *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// GET /repos/my-org/my-project/git/ref/heads/quarantine/state — branch exists
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/quarantine/state",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		})

	// GET /repos/my-org/my-project/contents/.quarantine/backend/state.json — empty state
	mux.HandleFunc("/repos/my-org/my-project/contents/.quarantine/backend/state.json",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

	// GET /search/issues — no closed quarantine issues
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

	// PR comment endpoint — track to assert no posts. Fall-through 404 on any GET.
	// We register a generic handler under /repos/my-org/my-project/issues/ to catch
	// /repos/my-org/my-project/issues/{N}/comments POSTs as well as issue creation.
	mux.HandleFunc("/repos/my-org/my-project/issues",
		func(w http.ResponseWriter, r *http.Request) {
			// POST creates an issue — accept (the run may create one for new flaky tests,
			// though our XML reports all-pass so this should not fire).
			if r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"number":1,"html_url":"https://github.com/my-org/my-project/issues/1"}`)
				return
			}
			http.NotFound(w, r)
		})
	mux.HandleFunc("/repos/my-org/my-project/issues/",
		func(w http.ResponseWriter, r *http.Request) {
			// /comments suffix — assert no PR comments were posted.
			if strings.HasSuffix(r.URL.Path, "/comments") && r.Method == http.MethodPost {
				atomic.AddInt32(commentPostCount, 1)
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

func TestRunReadsOwnerRepoFromConfigWhenOriginIsGerrit(t *testing.T) {
	dir := t.TempDir()

	// Gerrit origin proves run does NOT inspect the remote URL: a Gerrit URL
	// would fail `ParseRemote`'s `u.Host == "github.com"` check, returning ""
	// for owner/repo. With the fallback removed, run must read owner/repo
	// directly from config.yml and succeed.
	gitInit(t, dir, "https://gerrit.example.com/foo/bar.git")

	// Synthetic JUnit XML — one passing test. Suite "backend" runs successfully.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="rspec" tests="1" failures="0" errors="0" time="0.5">
  <testsuite name="spec/orders_spec.rb" tests="1" failures="0" time="0.5">
    <testcase classname="OrdersService" name="creates an order"
              file="spec/orders_spec.rb" time="0.5"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "rspec.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Pre-create the config.yml as the scenario requires:
	// owner/repo are explicit; suite "backend" runs an rspec-shaped command.
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
    rerun_command: ["false"]
    retries: 1
`, scriptPath, xmlPath)
	if err := os.WriteFile(filepath.Join(suiteDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	chdirTest(t, dir)

	var commentPostCount int32
	server := fakeGerritOriginGitHubAPI(t, &commentPostCount)

	resultsPath := filepath.Join(dir, ".quarantine", "backend", "results.json")
	output, err := executeRunCmd(t, []string{
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "config has owner/repo and origin is a Gerrit URL, all tests pass",
		Should:   "exit with code 0",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run targets my-org/my-project from config (origin is Gerrit)",
		Should:   "not print a degraded-mode warning about Gerrit/GitHub URL",
		Actual:   strings.Contains(output, "not a GitHub URL"),
		Expected: false,
	})

	_, statErr := os.Stat(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config-driven run completes successfully",
		Should:   "write .quarantine/backend/results.json to disk",
		Actual:   statErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "no GITHUB_EVENT_PATH and no --pr flag",
		Should:   "post zero PR comments",
		Actual:   atomic.LoadInt32(&commentPostCount),
		Expected: 0,
	})
}
