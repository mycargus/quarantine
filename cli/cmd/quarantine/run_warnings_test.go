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
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

// fakeWarningsGitHubAPI creates a test server with configurable responses.
func fakeWarningsGitHubAPI(
	t *testing.T,
	qs *quarantine.State,
	searchItems []int,
	searchTotalCount int,
	putStatusCode int,
	rateLimitRemaining, rateLimitLimit int,
) *httptest.Server {
	t.Helper()

	var stateContent []byte
	if qs != nil {
		var err error
		stateContent, err = qs.Marshal()
		if err != nil {
			t.Fatalf("marshal quarantine state: %v", err)
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if rateLimitLimit > 0 {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimitLimit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rateLimitRemaining))
			w.Header().Set("X-RateLimit-Reset", "1711000000")
		}

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			items := make([]map[string]interface{}, len(searchItems))
			for i, n := range searchItems {
				items[i] = map[string]interface{}{"number": n}
			}
			tc := searchTotalCount
			if tc == 0 {
				tc = len(searchItems)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": tc,
				"items":       items,
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(putStatusCode)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 59: Search API result limit exceeded ---

func TestRunSearchTruncatedEmitsWarning(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	entry := quarantine.Entry{
		TestID:   "src/foo.test.js::Foo::passes",
		Name:     "passes",
		FilePath: "src/foo.test.js",
	}
	n := 1
	entry.IssueNumber = &n
	qs.AddTest(entry)

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/foo.test.js" tests="1" failures="0">
    <testcase classname="Foo" name="passes" file="src/foo.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	closedItems := make([]int, 1000)
	for i := range closedItems {
		closedItems[i] = i + 1
	}
	server := fakeWarningsGitHubAPI(t, qs, closedItems, 2000, http.StatusOK, 0, 0)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SearchClosedIssues returns 1000 items but total_count=2000 (truncated)",
		Should:   "exit with code 0 (search truncation is a warning, not fatal)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SearchClosedIssues returns truncated=true (total_count=2000 > 1000 items)",
		Should:   "log a warning mentioning the 1,000 closed issues limit",
		Actual:   strings.Contains(output, "1,000 closed issues"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SearchClosedIssues returns truncated=true with total_count=2000",
		Should:   "include the actual total_count in the warning",
		Actual:   strings.Contains(output, "2000"),
		Expected: true,
	})
}

// --- Scenario 60: Rate limit warning (integration) ---

func TestRunRateLimitLowEmitsWarningAndContinues(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	junitXML := passingJUnitXML

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeWarningsGitHubAPI(t, qs, []int{}, 0, http.StatusOK, 50, 1000)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API returns X-RateLimit-Remaining=50, X-RateLimit-Limit=1000 (5% remaining)",
		Should:   "exit with code 0 (rate limit warning is not fatal)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API rate limit is low (5% remaining)",
		Should:   "emit warning mentioning rate limit low",
		Actual:   strings.Contains(output, "rate limit low"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API rate limit is low with 50 remaining",
		Should:   "include the remaining count in the warning",
		Actual:   strings.Contains(output, "50"),
		Expected: true,
	})
}

// --- Scenario 62: quarantine.json size limit (422 from GitHub) ---

func TestRunSizeLimitExceededEmitsWarningAndExits0(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfigWithRerunScript(t, dir, xmlPath, scriptPath, rerunScriptPath)
	chdirTest(t, dir)

	server := fakeWarningsGitHubAPI(t, qs, []int{}, 0, http.StatusUnprocessableEntity, 0, 0)
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT quarantine state returns 422 (file exceeds 1 MB GitHub limit)",
		Should:   "exit with code 0 (size limit is a warning, not fatal)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT quarantine state returns 422",
		Should:   "log warning mentioning 1 MB limit",
		Actual:   strings.Contains(output, "1 MB"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT quarantine state returns 422",
		Should:   "log warning mentioning skipping state update",
		Actual:   strings.Contains(output, "Skipping state update"),
		Expected: true,
	})
}

// --- Scenario 63: CAS retry exhaustion ---

func TestRunCASExhaustionEmitsWarningAndExits0(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfigWithRerunScript(t, dir, xmlPath, scriptPath, rerunScriptPath)
	chdirTest(t, dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			if len(qs.Tests) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			content, _ := qs.Marshal()
			encoded := base64.StdEncoding.EncodeToString(content)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "new-sha",
			})
		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			// Always 409 — forces CAS exhaustion.
			w.WriteHeader(http.StatusConflict)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	output, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all 3 CAS write attempts return 409 (concurrent write conflicts)",
		Should:   "exit with code 0 (CAS exhaustion is a warning, not fatal)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "WriteStateWithCAS exhausts all 3 retries",
		Should:   "log warning mentioning 3 attempts",
		Actual:   strings.Contains(output, "3 attempts"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "CAS exhaustion due to concurrent write conflicts",
		Should:   "log warning mentioning re-detection on next run",
		Actual:   strings.Contains(output, "next run"),
		Expected: true,
	})
}
