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

// --- Scenario 188: quarantine run — creates state branch on first invocation when missing ---
//
// Per ADR-038, when `quarantine run` discovers the `quarantine/state` branch
// is missing, it MUST create the branch (using the same idempotent pattern as
// `init` phase 2) and continue the run normally — instead of exiting with
// "Quarantine is not initialized." Three sub-cases:
//
//   A. Happy path — POST /git/refs returns 201; the run continues normally.
//   B. Concurrent race — POST /git/refs returns 422; treated as "branch exists,"
//      the run continues normally with no warning.
//   C. Degraded mode — POST /git/refs returns 403/5xx/network error; the CLI
//      emits `[quarantine] WARNING: Cannot create state branch ...` and
//      continues without quarantine awareness. The build is never broken.

// bootstrapMockState tracks request counts and the configured behavior of the
// fake GitHub API for `quarantine run` self-bootstrap tests. The CreateRefStatus
// field controls how the POST /git/refs handler responds (201, 422, 403, etc).
type bootstrapMockState struct {
	createRefStatus     int
	createRefPostCount  int32
	getDefaultBranchHit int32
	getRepoHit          int32
}

// fakeRunBootstrapAPI returns a mock GitHub API server that simulates the
// self-bootstrap flow: GET /repos/my-org/my-project returns default_branch=main,
// GET /git/ref/heads/quarantine/state returns 404, GET /git/ref/heads/main
// returns sha=basesha123, POST /git/refs uses the configured status, and other
// state/issue/search endpoints return empty/no-op responses.
func fakeRunBootstrapAPI(t *testing.T, state *bootstrapMockState) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// GET /repos/my-org/my-project — returns default_branch=main.
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&state.getRepoHit, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             1,
			"full_name":      "my-org/my-project",
			"default_branch": "main",
			"private":        false,
		})
	})

	// GET /repos/my-org/my-project/git/ref/heads/{branch}
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		// quarantine/state branch — always missing in these scenarios
		if strings.HasSuffix(r.URL.Path, "/heads/quarantine/state") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// default branch SHA lookup
		if strings.HasSuffix(r.URL.Path, "/heads/main") {
			atomic.AddInt32(&state.getDefaultBranchHit, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ref": "refs/heads/main",
				"object": map[string]interface{}{
					"sha":  "basesha123",
					"type": "commit",
				},
			})
			return
		}
		http.NotFound(w, r)
	})

	// POST /repos/my-org/my-project/git/refs — create the state branch with the
	// configured status code.
	mux.HandleFunc("/repos/my-org/my-project/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&state.createRefPostCount, 1)
		switch state.createRefStatus {
		case http.StatusCreated:
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ref": "refs/heads/quarantine/state",
				"object": map[string]interface{}{
					"sha":  "basesha123",
					"type": "commit",
				},
			})
		case http.StatusUnprocessableEntity:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Reference already exists"})
		case http.StatusForbidden:
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Resource not accessible by integration"})
		default:
			w.WriteHeader(state.createRefStatus)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "unexpected"})
		}
	})

	// State read: empty (404). State write: succeed.
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

	// Closed-issue search and open-issue search — both empty.
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

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// writeBootstrapSuiteConfig writes a `.quarantine/config.yml` under dir with
// owner=my-org, repo=my-project, and a single backend suite. The suite's
// command, junitxml, and rerun_command are interpolated from the inputs.
func writeBootstrapSuiteConfig(t *testing.T, dir, scriptPath, xmlPath, rerunScript string) {
	t.Helper()
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
`, scriptPath, xmlPath, rerunScript)
	if err := os.WriteFile(filepath.Join(suiteDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// --- Sub-case A: Happy path — branch missing → bootstrap → continue ---

func TestRunBootstrapsStateBranchOnFirstInvocation(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	// Sentinel file proves the test command actually executed.
	sentinelPath := filepath.Join(dir, "sentinel.txt")
	commandScript := filepath.Join(dir, "fake-test-with-sentinel")
	scriptBody := fmt.Sprintf("#!/bin/sh\ntouch %q\n%s\n", sentinelPath, initialScript)
	if err := os.WriteFile(commandScript, []byte(scriptBody), 0755); err != nil {
		t.Fatalf("write command script: %v", err)
	}
	writeBootstrapSuiteConfig(t, dir, commandScript, xmlPath, rerunScript)
	chdirTest(t, dir)

	mockState := &bootstrapMockState{createRefStatus: http.StatusCreated}
	server := fakeRunBootstrapAPI(t, mockState)

	output, err := executeRunCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "the quarantine/state branch does not exist and POST /git/refs returns 201",
		Should:   "exit with code 0 (success)",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the CLI created the state branch",
		Should:   "print the state-branch-created message to stderr",
		Actual:   strings.Contains(output, "[quarantine] State branch 'quarantine/state' created."),
		Expected: true,
	})

	_, sentinelErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the CLI bootstrapped the state branch and continued the run",
		Should:   "execute the suite's test command (sentinel file exists)",
		Actual:   sentinelErr == nil,
		Expected: true,
	})

	_, resultsErr := os.Stat(filepath.Join(dir, ".quarantine", "backend", "results.json"))
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the run completed normally after bootstrap",
		Should:   "write .quarantine/backend/results.json",
		Actual:   resultsErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a single CLI invocation that bootstrapped the state branch",
		Should:   "POST /git/refs exactly once",
		Actual:   atomic.LoadInt32(&mockState.createRefPostCount),
		Expected: 1,
	})
}

// --- Sub-case B: Concurrent race — POST returns 422 → treat as already exists ---

func TestRunBootstrapTreats422AsBenignRace(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	sentinelPath := filepath.Join(dir, "sentinel.txt")
	commandScript := filepath.Join(dir, "fake-test-with-sentinel")
	scriptBody := fmt.Sprintf("#!/bin/sh\ntouch %q\n%s\n", sentinelPath, initialScript)
	if err := os.WriteFile(commandScript, []byte(scriptBody), 0755); err != nil {
		t.Fatalf("write command script: %v", err)
	}
	writeBootstrapSuiteConfig(t, dir, commandScript, xmlPath, rerunScript)
	chdirTest(t, dir)

	mockState := &bootstrapMockState{createRefStatus: http.StatusUnprocessableEntity}
	server := fakeRunBootstrapAPI(t, mockState)

	output, err := executeRunCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "POST /git/refs returns 422 (concurrent shard already created the branch)",
		Should:   "exit with code 0 (continue normally)",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /git/refs returns 422",
		Should:   "not print a [quarantine] WARNING about state branch creation (422 is benign)",
		Actual:   strings.Contains(output, "WARNING: Cannot create state branch"),
		Expected: false,
	})

	_, sentinelErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /git/refs returns 422 and the run continues",
		Should:   "execute the suite's test command (sentinel file exists)",
		Actual:   sentinelErr == nil,
		Expected: true,
	})
}

// --- Sub-case C: Degraded mode — POST returns 403 → warn and continue ---

func TestRunBootstrap403WarnsAndContinues(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")

	xmlPath := filepath.Join(dir, "junit.xml")
	initialScript := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	rerunScript := writeAlwaysPassScript(t, dir, "rerun-script")

	sentinelPath := filepath.Join(dir, "sentinel.txt")
	commandScript := filepath.Join(dir, "fake-test-with-sentinel")
	scriptBody := fmt.Sprintf("#!/bin/sh\ntouch %q\n%s\n", sentinelPath, initialScript)
	if err := os.WriteFile(commandScript, []byte(scriptBody), 0755); err != nil {
		t.Fatalf("write command script: %v", err)
	}
	writeBootstrapSuiteConfig(t, dir, commandScript, xmlPath, rerunScript)
	chdirTest(t, dir)

	mockState := &bootstrapMockState{createRefStatus: http.StatusForbidden}
	server := fakeRunBootstrapAPI(t, mockState)

	output, err := executeRunCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_ACTIONS":                 "",
		"GITHUB_EVENT_PATH":              "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "POST /git/refs returns 403 (token lacks write scope)",
		Should:   "exit with code 0 — degraded mode keeps the build green",
		Actual:   extractExitCode(t, err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "branch creation fails with 403",
		Should:   "print [quarantine] WARNING: Cannot create state branch 'quarantine/state'",
		Actual:   strings.Contains(output, "[quarantine] WARNING: Cannot create state branch 'quarantine/state'"),
		Expected: true,
	})

	_, sentinelErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "branch creation failed but degraded mode continues",
		Should:   "execute the suite's test command (sentinel file exists)",
		Actual:   sentinelErr == nil,
		Expected: true,
	})
}

// --- Pure helpers ---

func TestFormatStateBranchCreatedMessage(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    `the branch name "quarantine/state"`,
		Should:   "return the canonical state-branch-created message",
		Actual:   formatStateBranchCreatedMessage("quarantine/state"),
		Expected: "[quarantine] State branch 'quarantine/state' created.",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    `a custom branch name "qa/state"`,
		Should:   "interpolate the branch name into the message",
		Actual:   formatStateBranchCreatedMessage("qa/state"),
		Expected: "[quarantine] State branch 'qa/state' created.",
	})
}

func TestFormatStateBranchCreationFailedWarning(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    `the branch name "quarantine/state" and reason "403 forbidden"`,
		Should:   "return the canonical degraded-mode WARNING with reason interpolated",
		Actual:   formatStateBranchCreationFailedWarning("quarantine/state", "403 forbidden"),
		Expected: "Cannot create state branch 'quarantine/state': 403 forbidden. Continuing in degraded mode.",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    `a custom branch name "qa/state" and a network error reason`,
		Should:   "interpolate both branch and reason into the WARNING",
		Actual:   formatStateBranchCreationFailedWarning("qa/state", "network unreachable"),
		Expected: "Cannot create state branch 'qa/state': network unreachable. Continuing in degraded mode.",
	})
}
