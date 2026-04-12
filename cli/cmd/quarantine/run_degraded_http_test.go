package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// passingJUnitXML is a minimal JUnit XML with one passing test.
const passingJUnitXML = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

// makeSuiteRunArgs builds suite-mode run arguments.
func makeSuiteRunArgs() []string {
	return []string{
		"unit",
	}
}

// --- Scenario 95: 401 Unauthorized ---

func TestRunDegradedWith401(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"Bad credentials"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 401 for quarantine state read",
		Should:   "exit with code 0 (degraded mode, test passes)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 401",
		Should:   "include '401' in degraded warning",
		Actual:   strings.Contains(output, "401"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 401",
		Should:   "mention token env var in degraded warning",
		Actual:   strings.Contains(output, "QUARANTINE_GITHUB_TOKEN") || strings.Contains(output, "GITHUB_TOKEN"),
		Expected: true,
	})
}

// --- Scenario 96: 403 Forbidden (no Retry-After) ---

func TestRunDegradedWith403NoRetryAfter(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			w.Header().Set("X-GitHub-Request-Id", "req123abc")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `{"message":"Forbidden"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 403 with X-GitHub-Request-Id header",
		Should:   "exit with code 0 (degraded mode)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 403 with X-GitHub-Request-Id: req123abc",
		Should:   "include '403' in degraded warning",
		Actual:   strings.Contains(output, "403"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 403 with X-GitHub-Request-Id: req123abc",
		Should:   "include request ID 'req123abc' in degraded warning",
		Actual:   strings.Contains(output, "req123abc"),
		Expected: true,
	})
}

// --- Scenario 97: 403 with Retry-After — retry succeeds ---

func TestRunDegradedWith403AndRetryAfterSucceeds(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	var contentsRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			n := atomic.AddInt32(&contentsRequests, 1)
			if n == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusForbidden)
				_, _ = fmt.Fprint(w, `{"message":"Forbidden"}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 403+Retry-After on first attempt and 200 on retry",
		Should:   "exit with code 0 (retry succeeded)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retry succeeded after 403+Retry-After",
		Should:   "NOT emit a degraded mode WARNING",
		Actual:   !strings.Contains(output, "degraded mode"),
		Expected: true,
	})
}

// --- Scenario 98: 5xx after retry ---

func TestRunDegradedWith5xxAfterRetry(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, `{"message":"Service Unavailable"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 503 on all contents requests",
		Should:   "exit with code 0 (degraded mode)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 503 on all contents requests",
		Should:   "include server error status in degraded warning",
		Actual:   strings.Contains(output, "503") || strings.Contains(output, "server error"),
		Expected: true,
	})
}

// --- Scenario 99: Timeout / network error ---

func TestRunDegradedWithNetworkError(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
		})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API is unreachable (network error)",
		Should:   "exit with code 0 (degraded mode, test passes)",
		Actual:   exitCode,
		Expected: 0,
	})
}

func TestRunDegradedWithNetworkErrorLogsWarning(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	output, _ := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": fakeUnreachableAPIURL(t),
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API is unreachable (network error)",
		Should:   "log a [quarantine] WARNING about degraded mode",
		Actual:   strings.Contains(output, "[quarantine] WARNING:"),
		Expected: true,
	})
}

// --- Scenario 100: 429 with short Retry-After — retry succeeds ---

func TestRunDegradedWith429ShortRetryAfterSucceeds(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	var contentsRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			n := atomic.AddInt32(&contentsRequests, 1)
			if n == 1 {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprint(w, `{"message":"rate limited"}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 429+Retry-After:1 on first attempt, 404 on retry",
		Should:   "exit with code 0 (retry succeeded)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retry succeeded after 429+Retry-After:1",
		Should:   "NOT emit a degraded mode WARNING",
		Actual:   !strings.Contains(output, "degraded mode"),
		Expected: true,
	})
}

// --- Scenario 101: 429 with long Retry-After (>30s) ---

func TestRunDegradedWith429LongRetryAfter(t *testing.T) {
	t.Setenv("QUARANTINE_RETRY_DELAY_SECONDS", "0")
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc","type":"commit"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			w.Header().Set("Retry-After", "31")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"message":"rate limited"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)
	}))
	defer server.Close()

	output, err := executeRunCmd(t, makeSuiteRunArgs(),
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 429 with Retry-After: 31 (exceeds 30s threshold)",
		Should:   "exit with code 0 (degraded mode)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 429 with Retry-After: 31",
		Should:   "include '31s' in degraded warning",
		Actual:   strings.Contains(output, "31s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API returns 429 with Retry-After: 31",
		Should:   "include 'threshold' in degraded warning",
		Actual:   strings.Contains(output, "threshold"),
		Expected: true,
	})
}
