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

// --- Scenario 64: Config resolution order ---

func TestRunJUnitXMLFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()

	// Config says junitxml: custom/*.xml, but --junitxml flag overrides it.
	overrideXMLPath := filepath.Join(dir, "override.xml")
	jestXML := `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
	if err := os.WriteFile(overrideXMLPath, []byte(jestXML), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := writeTempConfig(t, `
version: 1
framework: jest
junitxml: custom/*.xml
`)

	scriptPath := writeTestScript(t, dir, "", "", 0)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", overrideXMLPath, // CLI flag overrides config
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--junitxml flag set, config has different junitxml",
		Should:   "exit without error (used override.xml, not custom/*.xml)",
		Actual:   err == nil,
		Expected: true,
	})

	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr == nil {
		var results map[string]interface{}
		if json.Unmarshal(resultsData, &results) == nil {
			summary, _ := results["summary"].(map[string]interface{})
			riteway.Assert(t, riteway.Case[float64]{
				Given:    "--junitxml flag overrides config",
				Should:   "parse override.xml (1 test)",
				Actual:   summary["total"].(float64),
				Expected: 1,
			})
		}
	}
}

// --- Scenario 51: --verbose and --quiet flags ---

func TestRunVerboseAndQuietMutuallyExclusive(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--quiet",
		"--", "echo", "hi",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose and --quiet both set",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose and --quiet both set",
		Should:   "print mutually exclusive error message",
		Actual:   strings.Contains(output, "--verbose and --quiet are mutually exclusive"),
		Expected: true,
	})
}

func TestRunQuietSuppressesInfo(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, `<?xml version="1.0"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`, 0)
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--quiet",
		"--", scriptPath,
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

// --- Scenario 51 (verbose output): config resolution trace ---

func TestRunVerboseConfigResolution(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a nonexistent command so the run exits at LookPath — no GitHub API needed.
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag and nonexistent test command",
		Should:   "return an error (command not found)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print config resolution lines with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] config:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set with jest framework from config",
		Should:   "include framework=jest in resolution output",
		Actual:   strings.Contains(output, "framework=jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print total time even on error exit",
		Actual:   strings.Contains(output, "[quarantine] total time:"),
		Expected: true,
	})
}

// --- Mutation guard: line 36 len(args) == 0 ---

// TestRunMissingSeparatorWithEmptyArgs kills the mutation len(args) > 0 on line
// 36. When the user writes `quarantine run --`, args is empty (the separator is
// present but no command follows) — the run must return an error.
func TestRunMissingSeparatorWithEmptyArgs(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with -- separator but no test command",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with -- separator but no test command",
		Should:   "error message references missing test command",
		Actual:   strings.Contains(err.Error(), "missing"),
		Expected: true,
	})
}

// --- Mutation guard: line 44 verbose && quiet ---

// TestRunOnlyVerboseDoesNotError kills the mutation verbose || quiet on line 44.
// Passing only --verbose must succeed past the mutual-exclusion check.
func TestRunOnlyVerboseDoesNotError(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a nonexistent command so the run exits at LookPath, before any network
	// call — we only need to confirm the mutual-exclusion check was NOT triggered.
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose only (no --quiet) with nonexistent test command",
		Should:   "error on command-not-found, not on mutual-exclusion",
		Actual:   err != nil && strings.Contains(err.Error(), "command not found"),
		Expected: true,
	})
}

// TestRunOnlyQuietDoesNotError complements the above: only --quiet must also
// pass the mutual-exclusion check.
func TestRunOnlyQuietDoesNotError(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--quiet",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--quiet only (no --verbose) with nonexistent test command",
		Should:   "error on command-not-found, not on mutual-exclusion",
		Actual:   err != nil && strings.Contains(err.Error(), "command not found"),
		Expected: true,
	})
}

// --- Mutation guard: line 78 retriesFlag < 1 ---

// TestRunRetriesOneLowerBoundAccepted kills the mutation retriesFlag <= 1 on
// line 78. --retries 1 is the lowest valid value and must NOT be rejected.
func TestRunRetriesOneLowerBoundAccepted(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Nonexistent command exits at LookPath — enough to confirm the retries
	// validation did not reject the value.
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--retries", "1",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 1 (the minimum valid value)",
		Should:   "fail on command-not-found, not on retries-out-of-range",
		Actual:   err != nil && strings.Contains(err.Error(), "command not found"),
		Expected: true,
	})
}

// TestRunRetriesZeroIsIgnored verifies that --retries 0 (the default sentinel)
// does not trigger the range check and falls through to the config default.
func TestRunRetriesZeroIsIgnored(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--retries", "0",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--retries 0 (zero means 'not set', uses config default)",
		Should:   "not return retries-out-of-range error",
		Actual:   err == nil || !strings.Contains(err.Error(), "retries out of range"),
		Expected: true,
	})
}

// --- Mutation guard: line 264 cfg.Notifications.GitHubPRComment == nil ---

// TestRunGitHubPRCommentFalseSkipsPosting kills the mutation
// cfg.Notifications.GitHubPRComment != nil on line 264.
//
// The expression is:
//   prCommentEnabled = cfg.Notifications.GitHubPRComment == nil || *cfg.Notifications.GitHubPRComment
//
// After ApplyDefaults the field is never nil, so the nil branch is moot.
// The discriminating case is when the field is explicitly false:
//   Original:  (&false != nil) || false  = false  → comment skipped
//   Mutation:  (&false != nil) || false  — wait, mutation flips == to !=:
//   Original:  (&false == nil) || false  = false  → skip
//   Mutation:  (&false != nil) || false  = true   → comment posted (wrong)
//
// We test with --pr to supply a real PR number so the post path is reachable,
// then verify no comment POST is made when the flag is false.
func TestRunGitHubPRCommentFalseSkipsPosting(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
notifications:
  github_pr_comment: false
`)

	var commentCallCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/search/issues"):
			_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
		case strings.Contains(r.URL.Path, "/issues") && strings.Contains(r.URL.Path, "/comments"):
			atomic.AddInt32(&commentCallCount, 1)
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--pr", "42",
		"--", scriptPath,
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
		Should:   "not post any PR comment (comment call count is 0)",
		Actual:   atomic.LoadInt32(&commentCallCount),
		Expected: 0,
	})
}

// --- Scenario 51 (verbose output): API call details ---

func TestRunVerboseAPICall(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="0" errors="0" time="0.1">
  <testsuite name="__tests__/a.test.js" tests="1" errors="0" failures="0" skipped="0"
             timestamp="2026-03-15T14:22:10" time="0.1">
    <testcase classname="A" name="passes" file="__tests__/a.test.js" time="0.1">
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--verbose",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag with all tests passing",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print API call with [quarantine] prefix and GET /repos/ path",
		Actual:   strings.Contains(output, "[quarantine] GET /repos/"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set with branch existing",
		Should:   "print 200 status for branch check",
		Actual:   strings.Contains(output, "-> 200"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print config resolution with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] config:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag set",
		Should:   "print total time with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] total time:"),
		Expected: true,
	})
}
