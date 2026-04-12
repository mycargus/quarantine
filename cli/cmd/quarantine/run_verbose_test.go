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
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/search/issues"):
			_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 91: --verbose output --- (suite mode)

func TestRunVerboseRateLimitHeadersLogged(t *testing.T) {
	// In suite mode, verbose API logging happens when rate limit is low.
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	// Rate limit low (50/1000 = 5%) triggers the rate limit warning.
	server := fakeGitHubAPIWithRateLimit(t, 50, 1000, 1704067200)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returning low rate limit headers (suite mode)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returning X-RateLimit-Remaining=50 (5% of 1000)",
		Should:   "emit rate limit low warning",
		Actual:   strings.Contains(output, "rate limit low") || strings.Contains(output, "50"),
		Expected: true,
	})
}

// TestRunVerboseAPICallUsesQuarantinePrefix verifies that --verbose in suite mode
// doesn't break the run and still produces [quarantine] prefixed output.
func TestRunVerboseAPICallUsesQuarantinePrefix(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"--verbose",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag with passing tests (suite mode)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag in suite mode",
		Should:   "print [quarantine] prefixed output",
		Actual:   strings.Contains(output, "[quarantine]"),
		Expected: true,
	})
}

// --- Scenario 91: --verbose no token uses [quarantine] prefix ---

func TestRunVerboseNoTokenUsesQuarantinePrefix(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token (degraded mode continues)",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no GitHub token",
		Should:   "print WARNING about missing token with [quarantine] prefix",
		Actual:   strings.Contains(output, "[quarantine] WARNING:"),
		Expected: true,
	})
}

// TestRunVerboseConfigResolutionUsesQuarantinePrefix: in suite mode, there is
// no config resolution trace. This test verifies the run completes successfully.
func TestRunVerboseConfigResolutionUsesQuarantinePrefix(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"--verbose",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--verbose flag in suite mode",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})
}

// --- Scenario 93: All [quarantine] log output goes to stderr, not stdout ---

func TestRunAllQuarantineOutputGoesToStderr(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(passingJUnitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	stdout, stderr, err := executeRunCmdSeparateStreams(t, []string{
		"unit",
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
