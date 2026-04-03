package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// executeRunCmdSeparateStreams captures cobra stdout and stderr into separate
// buffers. This allows asserting that [quarantine] messages go only to stderr.
func executeRunCmdSeparateStreams(t *testing.T, args []string, env map[string]string) (stdout, stderr string, exitErr error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	rootCmd.SetOut(stdoutBuf)
	rootCmd.SetErr(stderrBuf)
	rootCmd.SetArgs(append([]string{"run"}, args...))
	exitErr = rootCmd.Execute()
	return stdoutBuf.String(), stderrBuf.String(), exitErr
}

// fakeGitHubAPIWithRateLimit returns a fake GitHub API server that injects
// X-RateLimit-* headers into every response.
func fakeGitHubAPIWithRateLimit(t *testing.T, remaining int, limit int, resetUnix int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetUnix))

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/search/issues"):
			_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 91: --verbose output includes GitHub API timing and rate limit headers ---

func TestRunVerboseRateLimitHeadersLogged(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a reset time known to format as a specific "HH:MM UTC" value.
	// Unix 1704067200 = 2024-01-01 00:00:00 UTC → "00:00 UTC"
	server := fakeGitHubAPIWithRateLimit(t, 987, 1000, 1704067200)
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
		Given:    "--verbose flag and GitHub API returning rate limit headers",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag and GitHub API returning X-RateLimit-Remaining=987",
		Should:   "print X-RateLimit-Remaining in verbose output",
		Actual:   strings.Contains(output, "[quarantine] X-RateLimit-Remaining: 987"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag and GitHub API returning X-RateLimit-Reset=1704067200 (00:00 UTC)",
		Should:   "print reset time in verbose output",
		Actual:   strings.Contains(output, "resets at 00:00 UTC"),
		Expected: true,
	})
}

// --- Scenario 91: --verbose config resolution uses [quarantine] prefix ---

func TestRunVerboseConfigResolutionUsesQuarantinePrefix(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	// Use a nonexistent command so the run exits at LookPath — no GitHub API needed.
	output, _ := executeRunCmd(t, []string{
		"--config", configPath,
		"--verbose",
		"--", "nonexistent-command-xyz",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag with jest framework",
		Should:   "print config resolution lines with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] config:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag with jest framework from config file",
		Should:   "include framework=jest (from quarantine.yml)",
		Actual:   strings.Contains(output, "framework=jest (from quarantine.yml)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag",
		Should:   "print total time with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] total time:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag",
		Should:   "not print old [verbose] Config resolution: header line",
		Actual:   strings.Contains(output, "[verbose] Config resolution:"),
		Expected: false,
	})
}

// --- Scenario 91: --verbose API call uses [quarantine] prefix ---

func TestRunVerboseAPICallUsesQuarantinePrefix(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.1"/>
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
		Given:    "--verbose flag with passing tests",
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
		Given:    "--verbose flag with branch existing",
		Should:   "print -> 200 status for branch check",
		Actual:   strings.Contains(output, "-> 200"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag",
		Should:   "not print old [verbose] API call: prefix",
		Actual:   strings.Contains(output, "[verbose] API call:"),
		Expected: false,
	})
}

// --- Scenario 91: --verbose no token uses [quarantine] prefix ---

func TestRunVerboseNoTokenUsesQuarantinePrefix(t *testing.T) {
	dir := t.TempDir()
	junitXML := `<?xml version="1.0" encoding="UTF-8"?><testsuites tests="1"><testsuite name="s.test.js" tests="1"><testcase classname="S" name="t" file="s.test.js" time="0.001"></testcase></testsuite></testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	configPath := writeTempConfig(t, `
version: 1
framework: jest
github:
  owner: testowner
  repo: testrepo
`)

	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", filepath.Join(dir, "results.json"),
		"--verbose",
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --verbose flag",
		Should:   "exit without error (degraded mode continues)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --verbose flag",
		Should:   "print [quarantine] API: skipped (no token)",
		Actual:   strings.Contains(output, "[quarantine] API: skipped (no token)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token and --verbose flag",
		Should:   "not print old [verbose] API call: skipped (no token)",
		Actual:   strings.Contains(output, "[verbose] API call: skipped (no token)"),
		Expected: false,
	})
}

// --- Scenario 93: All [quarantine] log output goes to stderr, not stdout ---

func TestRunAllQuarantineOutputGoesToStderr(t *testing.T) {
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
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	stdout, stderr, err := executeRunCmdSeparateStreams(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a normal run with passing tests",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine emits [quarantine] info messages",
		Should:   "write [quarantine] messages to stderr",
		Actual:   strings.Contains(stderr, "[quarantine]"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine emits [quarantine] info messages",
		Should:   "not write [quarantine] messages to stdout",
		Actual:   strings.Contains(stdout, "[quarantine]"),
		Expected: false,
	})
}
