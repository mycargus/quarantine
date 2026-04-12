package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 137: zero-failure run still writes results.json and posts PR comment ---

// fakeZeroFailureAPI creates a server for a zero-failure run with quarantined tests.
// It returns existing quarantine state with 2 quarantined tests, handles branch check,
// and captures any PR comment posted.
func fakeZeroFailureAPI(t *testing.T, prNumber int, commentBodyPtr *string, putCalled *int32) *httptest.Server {
	t.Helper()

	// State with 2 quarantined tests.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/models/user_spec.rb::UserModel::validates email",
		"validates email",
		"spec/models/user_spec.rb",
		10,
	))
	qs.AddTest(makeQuarantineEntry(
		"spec/models/post_spec.rb::PostModel::validates title",
		"validates title",
		"spec/models/post_spec.rb",
		11,
	))
	stateContent, err := qs.Marshal()
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(stateContent)
	statePath := suiteStatePath("backend")

	var commentCreated int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// Return the pre-populated state.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"+statePath):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-existing",
			})

		// State PUT should NOT be called (no changes in a zero-failure run).
		// Constrained to the suite's state path so unrelated PUTs don't fire it.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"+statePath):
			atomic.AddInt32(putCalled, 1)
			w.WriteHeader(http.StatusOK)

		// Closed-issue search (no closed issues → nothing unquarantined).
		case strings.Contains(r.URL.Path, "/search/issues") && strings.Contains(r.URL.RawQuery, "is%3Aclosed"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// Open-issue dedup search.
		case strings.Contains(r.URL.Path, "/search/issues") && strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// List PR comments (none exist yet).
		case r.Method == "GET" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			_ = json.NewEncoder(w).Encode([]interface{}{})

		// Capture PR comment body.
		case r.Method == "POST" && strings.Contains(r.URL.Path, fmt.Sprintf("/issues/%d/comments", prNumber)):
			if atomic.AddInt32(&commentCreated, 1) == 1 {
				var req map[string]interface{}
				if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr == nil {
					if body, ok := req["body"].(string); ok {
						*commentBodyPtr = body
					}
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

// TestRunSuiteZeroFailuresWritesResultsAndComment verifies that when all tests
// pass (but quarantined tests exist in state), the CLI still writes results.json,
// still posts the PR comment, does NOT update the state file, and exits 0.
func TestRunSuiteZeroFailuresWritesResultsAndComment(t *testing.T) {
	dir := t.TempDir()
	prNumber := 77

	// XML with 1 passing test (zero failures).
	xmlPath := filepath.Join(dir, "rspec.xml")
	rspecXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="1" skipped="0" failures="0" errors="0" time="0.1">
  <testcase classname="spec.controllers.health_spec"
            name="HealthController returns 200"
            file="./spec/controllers/health_spec.rb"
            time="0.1">
  </testcase>
</testsuite>`
	if err := os.WriteFile(xmlPath, []byte(rspecXML), 0644); err != nil {
		t.Fatalf("write rspec.xml: %v", err)
	}

	// Test command: exits 0, does not rewrite the XML.
	fakeBin := filepath.Join(dir, "fake-rspec")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake-rspec: %v", err)
	}

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    retries: 1
`, fakeBin, xmlPath)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}

	var commentBody string
	var putCalled int32
	server := fakeZeroFailureAPI(t, prNumber, &commentBody, &putCalled)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, runErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"--pr", fmt.Sprintf("%d", prNumber),
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-failure run with 2 quarantined tests in state",
		Should:   "exit with code 0",
		Actual:   runErr == nil,
		Expected: true,
	})

	_, statErr := os.Stat(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-failure run with quarantined tests",
		Should:   "write results.json even when zero failures",
		Actual:   statErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-failure run on a PR with quarantined tests",
		Should:   "post PR comment (comment body is non-empty)",
		Actual:   commentBody != "",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-failure run on a PR with 2 quarantined tests",
		Should:   "PR comment contains the suite-specific marker '<!-- quarantine:backend -->'",
		Actual:   strings.Contains(commentBody, "<!-- quarantine:backend -->"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a zero-failure run where quarantined count is 2",
		Should:   "PR comment mentions quarantined count in summary table",
		Actual:   strings.Contains(commentBody, "| Quarantined (excluded) | 2 |"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "a zero-failure run with no state changes",
		Should:   "NOT update the state file (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled),
		Expected: 0,
	})
}

// --- Scenario 138: quarantine run --dry-run analyzes existing JUnit XML ---

// fakeStateBranchAPI creates a minimal server that handles the branch check
// and returns pre-populated quarantine state. It does NOT serve issue/comment
// endpoints since dry-run should not call them.
func fakeStateBranchAPI(t *testing.T, qs *quarantine.State, suiteName string, putCalled *int32) *httptest.Server {
	t.Helper()

	statePath := suiteStatePath(suiteName)
	var stateContent []byte
	if qs != nil {
		var err error
		stateContent, err = qs.Marshal()
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"+statePath):
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-dry",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			atomic.AddInt32(putCalled, 1)
			w.WriteHeader(http.StatusOK)

		default:
			// Dry-run must NOT reach issues or comments endpoints.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestRunSuiteDryRunAnalyzesExistingXML verifies the --dry-run behavior in
// suite mode: reads existing rspec.xml, does NOT execute the test command,
// does NOT write results.json, does NOT update state, and prints analysis text.
func TestRunSuiteDryRunAnalyzesExistingXML(t *testing.T) {
	dir := t.TempDir()

	// Pre-write rspec.xml with 3 failures (simulates existing output on disk).
	xmlPath := filepath.Join(dir, "rspec.xml")
	rspecXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="3" skipped="0" failures="3" errors="0" time="0.3">
  <testcase classname="spec.a_spec" name="A fails" file="./spec/a_spec.rb" time="0.1">
    <failure type="Error" message="expected true"/>
  </testcase>
  <testcase classname="spec.b_spec" name="B fails" file="./spec/b_spec.rb" time="0.1">
    <failure type="Error" message="expected true"/>
  </testcase>
  <testcase classname="spec.c_spec" name="C fails" file="./spec/c_spec.rb" time="0.1">
    <failure type="Error" message="expected true"/>
  </testcase>
</testsuite>`
	if err := os.WriteFile(xmlPath, []byte(rspecXML), 0644); err != nil {
		t.Fatalf("write rspec.xml: %v", err)
	}

	// Sentinel file: if the test command were executed, it would create this file.
	sentinelPath := filepath.Join(dir, "command-executed.txt")

	// Test command writes sentinel — must NOT be run in dry-run mode.
	fakeBin := filepath.Join(dir, "fake-rspec")
	script := fmt.Sprintf("#!/bin/sh\ntouch %s\nexit 0\n", sentinelPath)
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-rspec: %v", err)
	}

	// Quarantine state with 1 quarantined test.
	qs := quarantine.NewEmptyState()
	qs.AddTest(makeQuarantineEntry(
		"spec/existing_spec.rb::ExistingClass::is quarantined",
		"is quarantined",
		"spec/existing_spec.rb",
		5,
	))

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    retries: 1
`, fakeBin, xmlPath)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}

	var putCalled int32
	server := fakeStateBranchAPI(t, qs, "backend", &putCalled)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, runErr := executeRunCmdCaptureBoth(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"--dry-run",
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run with existing rspec.xml",
		Should:   "exit with code 0",
		Actual:   runErr == nil,
		Expected: true,
	})

	// The test command must NOT have been executed.
	_, sentinelErr := os.Stat(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run",
		Should:   "NOT execute the test command (sentinel file must not exist)",
		Actual:   os.IsNotExist(sentinelErr),
		Expected: true,
	})

	// results.json must NOT be written.
	_, resultsErr := os.Stat(resultsPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run",
		Should:   "NOT write results.json",
		Actual:   os.IsNotExist(resultsErr),
		Expected: true,
	})

	// State file must NOT be updated.
	riteway.Assert(t, riteway.Case[int32]{
		Given:    "quarantine run backend --dry-run with 3 failures in existing XML",
		Should:   "NOT write or update the state file (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled),
		Expected: 0,
	})

	// Output must contain dry-run analysis text.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run with 3 failures in rspec.xml",
		Should:   "print '[quarantine] DRY RUN' prefix in output",
		Actual:   strings.Contains(output, "[quarantine] DRY RUN"),
		Expected: true,
	})
}

// TestRunSuiteDryRunNoXMLExitsZeroWithWarning verifies that when --dry-run is
// used and no rspec.xml exists, the CLI prints a warning and exits 0.
func TestRunSuiteDryRunNoXMLExitsZeroWithWarning(t *testing.T) {
	dir := t.TempDir()

	// XML file does NOT exist.
	xmlPath := filepath.Join(dir, "rspec.xml")

	fakeBin := filepath.Join(dir, "fake-rspec")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake-rspec: %v", err)
	}

	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    retries: 1
`, fakeBin, xmlPath)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}

	var putCalled int32
	server := fakeStateBranchAPI(t, nil, "backend", &putCalled)
	defer server.Close()

	output, runErr := executeRunCmdCaptureBoth(t, []string{
		"--config", configPath,
		"--dry-run",
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run with no rspec.xml on disk",
		Should:   "exit with code 0",
		Actual:   runErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run backend --dry-run with no rspec.xml",
		Should:   "print a warning about missing XML",
		Actual:   strings.Contains(output, "WARNING") && strings.Contains(output, "No JUnit XML"),
		Expected: true,
	})
}
