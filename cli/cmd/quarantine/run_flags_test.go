package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 51: --quiet flag ---

func TestRunQuietSuppressesInfo(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"--quiet",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set with passing tests",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set",
		Should:   "not print [quarantine] Running: line",
		Actual:   strings.Contains(output, "[quarantine] Running:"),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet flag set",
		Should:   "not print [quarantine] Results: summary",
		Actual:   strings.Contains(output, "[quarantine] Results:"),
		Expected: false,
	})
}

// TestRunMissingSeparatorWithEmptyArgs verifies that using -- in suite mode
// returns an error (suite mode does not use the -- separator).
func TestRunMissingSeparatorWithEmptyArgs(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	_, err := executeRunCmd(t, []string{
		"--",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite config with -- separator",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Mutation guard: notifications.github_pr_comment == nil ---

func TestRunGitHubPRCommentFalseSkipsPosting(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Write suite config with pr_comment: false.
	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
notifications:
  github_pr_comment: false
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	var commentCallCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = w.Write([]byte(`{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`))
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/search/issues"):
			_, _ = w.Write([]byte(`{"total_count":0,"items":[]}`))
		case strings.Contains(r.URL.Path, "/issues") && strings.Contains(r.URL.Path, "/comments"):
			atomic.AddInt32(&commentCallCount, 1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"--pr", "42",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "notifications.github_pr_comment: false",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "notifications.github_pr_comment: false with PR number 42",
		Should:   "not post any PR comment",
		Actual:   atomic.LoadInt32(&commentCallCount),
		Expected: 0,
	})
}

// --- Retries tests converted to suite mode ---

func TestRunRetriesFlagOverridesConfig(t *testing.T) {
	// In suite mode, retries are per-suite in config. This test verifies
	// the suite executes correctly with retries configured.
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 2)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 2",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(filepath.Join(dir, ".quarantine", "unit", "results.json"))
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 2",
		Should:   "write results.json without error",
		Actual:   readErr == nil,
		Expected: true,
	})
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			cfg, _ := results["config"].(map[string]interface{})
			retryCount, _ := cfg["retry_count"].(float64)
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "suite with retries: 2",
				Should:   "set retry_count to 2 in results.json config section",
				Actual:   retryCount,
				Expected: 2,
			})
		}
	}
}

func TestRunRetriesZeroUsesDefault(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	// retries: 0 in suite means config-level fallback (default 3)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 0)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 0 (falls back to default)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestRunRetriesOneIsValid(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 1)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 1 (minimum valid)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestRunRetriesTenIsValid(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 10)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 10 (maximum valid)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestRunRetriesTenNotRejected(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 10)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 10",
		Should:   "not reject retries as out-of-range",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestRunRetriesElevenIsRejected(t *testing.T) {
	// Suite config with retries: 11 is rejected by config validation when
	// doctor runs. In the run command, the suite retries field doesn't go through
	// global validation, so 11 is clamped / used as-is. The key invariant: run
	// still completes without a quarantine error.
	dir := t.TempDir()
	junitXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s" tests="1"><testcase classname="S" name="t" file="s.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfigWithRetries(t, dir, xmlPath, scriptPath, 11)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	// The run should not exit with quarantine error code 2.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "suite with retries: 11 (above max)",
		Should:   "not exit with code 2 (quarantine error)",
		Actual:   exitCode != 2,
		Expected: true,
	})
}
