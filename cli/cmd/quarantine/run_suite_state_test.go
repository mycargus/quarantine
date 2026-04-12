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

// --- Unit tests for pure path/marker/label functions (Scenario 121) ---

func TestSuiteStatePath(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'backend'",
		Should:   "return '.quarantine/backend/state.json'",
		Actual:   suiteStatePath("backend"),
		Expected: ".quarantine/backend/state.json",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'frontend'",
		Should:   "return '.quarantine/frontend/state.json'",
		Actual:   suiteStatePath("frontend"),
		Expected: ".quarantine/frontend/state.json",
	})
}

func TestSuitePRCommentMarker(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'backend'",
		Should:   "return '<!-- quarantine:backend -->'",
		Actual:   suitePRCommentMarker("backend"),
		Expected: "<!-- quarantine:backend -->",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'frontend'",
		Should:   "return '<!-- quarantine:frontend -->'",
		Actual:   suitePRCommentMarker("frontend"),
		Expected: "<!-- quarantine:frontend -->",
	})
}

func TestSuiteIssueLabel(t *testing.T) {
	// SHA-256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	// First 8 hex chars: 2cf24dba
	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'backend' and test ID 'hello'",
		Should:   "return 'quarantine:backend:2cf24dba'",
		Actual:   suiteIssueLabel("backend", "hello"),
		Expected: "quarantine:backend:2cf24dba",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "suite name 'frontend' and test ID 'hello'",
		Should:   "return 'quarantine:frontend:2cf24dba'",
		Actual:   suiteIssueLabel("frontend", "hello"),
		Expected: "quarantine:frontend:2cf24dba",
	})
}

// --- Integration test helpers for suite-mode per-suite state/PR-comment/label (Scenario 121) ---

// flakySuiteResult holds all observable outcomes of a flaky-suite integration run.
type flakySuiteResult struct {
	err              error
	capturedStatePath string // URL path of the state-file PUT request
	capturedLabel    string // comma-joined labels sent to POST /issues
	capturedComment  string // body of the PR comment POST
}

// runFlakySuiteIntegration is the shared arrange-act helper for Scenarios 121's
// three integration tests. It sets up a flaky suite scenario, runs the command
// once, and returns all observable outcomes. Each test then asserts its unique
// concern against the result.
func runFlakySuiteIntegration(t *testing.T, suiteName string, prNumber int) flakySuiteResult {
	t.Helper()
	dir := t.TempDir()

	var result flakySuiteResult

	server := fakeSuiteFlakyGitHubAPI(t, suiteName, prNumber,
		&result.capturedStatePath,
		&result.capturedLabel,
		&result.capturedComment,
	)
	defer server.Close()

	buildFlakySuiteScenario(t, dir, suiteName)

	_, result.err = executeRunCmd(t, []string{
		"--pr", fmt.Sprintf("%d", prNumber),
		suiteName,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_BASE_REF":                "main",
	})

	return result
}

// fakeSuiteFlakyGitHubAPI creates a test server for a flaky-suite run. It:
//   - handles branch check (returns branch exists)
//   - returns 404 for the per-suite state file GET (first run, no file yet)
//   - accepts PUT for the per-suite state file and captures the URL path
//   - handles search and issue/comment endpoints
//   - captures the issue label and PR comment body
func fakeSuiteFlakyGitHubAPI(
	t *testing.T,
	suiteName string,
	prNumber int,
	statePathCapturePtr *string,
	issueLabelCapturePtr *string,
	prCommentBodyCapturePtr *string,
) *httptest.Server {
	t.Helper()

	var issueCreated int32
	var prCommentCreated int32
	statePath := suiteStatePath(suiteName) // e.g. .quarantine/backend/state.json

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Read per-suite state file — return 404 (first run, no file yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"+statePath):
			w.WriteHeader(http.StatusNotFound)

		// CAS write for per-suite state file — capture the path for explicit assertion.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"+statePath):
			if statePathCapturePtr != nil {
				// Extract the file path from the URL: /repos/{owner}/{repo}/contents/{path}
				const contentsPrefix = "/contents/"
				idx := strings.Index(r.URL.Path, contentsPrefix)
				if idx >= 0 {
					*statePathCapturePtr = r.URL.Path[idx+len(contentsPrefix):]
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha123"}}`)

		// Batch closed-issue search.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Dedup open-issue search — no existing issue.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// List PR comments (none exist yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Create issue — capture labels.
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues") &&
			!strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			if atomic.AddInt32(&issueCreated, 1) == 1 {
				body, _ := io.ReadAll(r.Body)
				var req map[string]interface{}
				_ = json.Unmarshal(body, &req)
				if labels, ok := req["labels"].([]interface{}); ok {
					parts := make([]string, len(labels))
					for i, l := range labels {
						parts[i] = fmt.Sprintf("%v", l)
					}
					*issueLabelCapturePtr = strings.Join(parts, ",")
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   201,
				"html_url": "https://github.com/test-owner/test-repo/issues/201",
			})

		// Post PR comment — capture body.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			if atomic.AddInt32(&prCommentCreated, 1) == 1 {
				body, _ := io.ReadAll(r.Body)
				var req map[string]interface{}
				_ = json.Unmarshal(body, &req)
				if bodyStr, ok := req["body"].(string); ok {
					*prCommentBodyCapturePtr = bodyStr
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       1001,
				"html_url": fmt.Sprintf("https://github.com/test-owner/test-repo/issues/%d#issuecomment-1001", prNumber),
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// buildFlakySuiteScenario writes a .quarantine/config.yml for a suite whose test
// command fails on the first call and passes on subsequent calls (simulating a
// flaky test). Returns the config file path and the JUnit XML path.
func buildFlakySuiteScenario(t *testing.T, dir, suiteName string) (configPath, xmlPath string) {
	t.Helper()

	xmlPath = filepath.Join(dir, "rspec.xml")
	counterFile := filepath.Join(dir, "run-count.txt")

	failureXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="1" skipped="0" failures="1" errors="0" time="0.1">
  <testcase classname="spec.models.user_spec"
            name="User validates email"
            file="./spec/models/user_spec.rb"
            time="0.1">
    <failure type="RSpec::Expectations::ExpectationNotMetError"
             message="expected valid but got invalid"/>
  </testcase>
</testsuite>`

	fakeBin := filepath.Join(dir, "fake-rspec")
	script := fmt.Sprintf(`#!/bin/sh
if [ ! -f %s ]; then
  touch %s
  cat > %s << 'XMLEOF'
%s
XMLEOF
  exit 1
fi
exit 0
`, counterFile, counterFile, xmlPath, failureXML)
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-rspec: %v", err)
	}

	fakeRerun := filepath.Join(dir, "fake-rspec-rerun")
	if err := os.WriteFile(fakeRerun, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake-rspec-rerun: %v", err)
	}

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath = filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: %s
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s", "{name}"]
    retries: 3
`, suiteName, fakeBin, xmlPath, fakeRerun)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
	chdirTest(t, dir)

	return configPath, xmlPath
}

// --- Integration tests: Scenario 121 ---
// Each test shares the same scenario (backend suite, flaky test) via
// runFlakySuiteIntegration and asserts its unique concern.

func TestRunSuiteCreatesPerSuiteStatePath(t *testing.T) {
	r := runFlakySuiteIntegration(t, "backend", 42)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite where the test is flaky",
		Should:   "exit without error (code 0)",
		Actual:   r.err == nil,
		Expected: true,
	})

	// Explicit path assertion — the PUT request URL path must be the per-suite path.
	riteway.Assert(t, riteway.Case[string]{
		Given:    "state file PUT for suite 'backend'",
		Should:   "use path '.quarantine/backend/state.json'",
		Actual:   r.capturedStatePath,
		Expected: ".quarantine/backend/state.json",
	})
}

func TestRunSuiteUsesPerSuitePRCommentMarker(t *testing.T) {
	r := runFlakySuiteIntegration(t, "backend", 42)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with a flaky test running on PR 42",
		Should:   "exit without error (code 0)",
		Actual:   r.err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name is 'backend'",
		Should:   "post PR comment starting with '<!-- quarantine:backend -->'",
		Actual:   strings.HasPrefix(r.capturedComment, "<!-- quarantine:backend -->"),
		Expected: true,
	})
}

func TestRunSuiteIssueHasSuiteNameInLabel(t *testing.T) {
	r := runFlakySuiteIntegration(t, "backend", 42)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with a flaky test",
		Should:   "exit without error (code 0)",
		Actual:   r.err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite name is 'backend'",
		Should:   "create issue with label containing 'quarantine:backend:'",
		Actual:   strings.Contains(r.capturedLabel, "quarantine:backend:"),
		Expected: true,
	})

	// The hash portion must be exactly 8 hex characters.
	idx := strings.Index(r.capturedLabel, "quarantine:backend:")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite issue label contains 'quarantine:backend:'",
		Should:   "have exactly 8 hex chars after 'quarantine:backend:'",
		Actual: func() bool {
			if idx < 0 {
				return false
			}
			rest := r.capturedLabel[idx+len("quarantine:backend:"):]
			if end := strings.IndexAny(rest, ","); end >= 0 {
				rest = rest[:end]
			}
			return len(rest) == 8
		}(),
		Expected: true,
	})
}
