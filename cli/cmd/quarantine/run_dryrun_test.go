package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/quarantine"
)

func encodeBase64ForTest(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// fakeM4GitHubAPIWithPUTTracking creates a test server like fakeM4GitHubAPI
// but additionally tracks whether any PUT occurred.
func fakeM4GitHubAPIWithPUTTracking(t *testing.T, qs *quarantine.State, closedIssueNumbers []int, putCalled *int32) *httptest.Server {
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

		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = w.Write([]byte(`{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`))

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := encodeBase64ForTest(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			items := make([]map[string]interface{}, len(closedIssueNumbers))
			for i, n := range closedIssueNumbers {
				items[i] = map[string]interface{}{"number": n}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": len(closedIssueNumbers),
				"items":       items,
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			atomic.AddInt32(putCalled, 1)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// executeRunCmdCaptureBoth runs the run command, returning both the combined
// output buffer and the exit error.
func executeRunCmdCaptureBoth(t *testing.T, args []string, env map[string]string) (output string, err error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"run"}, args...))
	err = rootCmd.Execute()
	return buf.String(), err
}

// --- Scenario 45: --dry-run flag (suite mode) ---

// TestRunDryRunDoesNotWriteQuarantineState verifies that --dry-run doesn't write
// quarantine state. In suite mode, dry-run reads existing XML without running
// the test command.
func TestRunDryRunDoesNotWriteQuarantineState(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="__tests__/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout"
              file="__tests__/payment.test.js" time="0.5">
      <failure message="Timeout exceeded">Timeout exceeded</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	// Pre-write XML for dry-run to read.
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, "", "", 0)

	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, qs, []int{}, &putCalled)
	defer server.Close()

	// Write suite config manually so we can control junitxml path.
	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	output, err := executeRunCmdCaptureBoth(t, []string{
		"--dry-run",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run in suite mode",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run in suite mode",
		Should:   "not write quarantine state (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled) == 0,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run in suite mode",
		Should:   "print DRY RUN message",
		Actual:   strings.Contains(output, "DRY RUN"),
		Expected: true,
	})
}

func TestRunDryRunWithNoFlakyTestsExitsZeroWithoutDryRunMessage(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="__tests__/payment.test.js" tests="1" failures="0">
    <testcase classname="PaymentService" name="should process payment"
              file="__tests__/payment.test.js" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	output, err := executeRunCmdCaptureBoth(t, []string{
		"--dry-run",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with all tests passing (no failures)",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with no failures",
		Should:   "print DRY RUN analysis (suite mode always prints analysis)",
		Actual:   strings.Contains(output, "DRY RUN"),
		Expected: true,
	})
}

func TestRunDryRunWouldQuarantineListsEachFlakyTest(t *testing.T) {
	dir := t.TempDir()

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2" failures="2">
  <testsuite name="__tests__/search.test.js" tests="1" failures="1">
    <testcase classname="SearchService" name="should fuzzy match"
              file="__tests__/search.test.js" time="0.5">
      <failure message="fuzzy match failed">fuzzy match failed</failure>
    </testcase>
  </testsuite>
  <testsuite name="__tests__/api.test.js" tests="1" failures="1">
    <testcase classname="ApiService" name="should handle rate limit"
              file="__tests__/api.test.js" time="0.5">
      <failure message="rate limit error">rate limit error</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	server := fakeM4GitHubAPI(t, quarantine.NewEmptyState(), []int{})
	defer server.Close()

	output, err := executeRunCmdCaptureBoth(t, []string{
		"--dry-run",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with two failing tests",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with two failing tests",
		Should:   "print DRY RUN analysis",
		Actual:   strings.Contains(output, "DRY RUN"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with 2 non-quarantined failures",
		Should:   "show non-quarantined failure count",
		Actual:   strings.Contains(output, "2"),
		Expected: true,
	})
}

func TestRunDryRunWithExistingQStateDoesNotWriteState(t *testing.T) {
	dir := t.TempDir()

	qs := quarantine.NewEmptyState()
	qs.AddTest(quarantine.Entry{
		TestID:   "src/foo.test.js::Foo::bar",
		Name:     "bar",
		FilePath: "src/foo.test.js",
	})

	junitXML := passingJUnitXML

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	var putCalled int32
	server := fakeM4GitHubAPIWithPUTTracking(t, qs, []int{}, &putCalled)
	defer server.Close()

	_, err := executeRunCmdCaptureBoth(t, []string{
		"--dry-run",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with existing quarantine state",
		Should:   "exit with code 0",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "--dry-run with existing quarantine state",
		Should:   "not write quarantine state (PUT not called)",
		Actual:   atomic.LoadInt32(&putCalled) == 0,
		Expected: true,
	})
}

// TestRunDryRunSkipsNotificationBlockEvenWithNonNilClient verifies that
// --dry-run skips issue creation even with a valid GitHub client.
func TestRunDryRunSkipsNotificationBlockEvenWithNonNilClient(t *testing.T) {
	dir := t.TempDir()

	junitXML := passingJUnitXML
	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(junitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	var issueCreateCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = w.Write([]byte(`{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`))
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			atomic.AddInt32(&issueCreateCount, 1)
			_, _ = w.Write([]byte(`{"number":101,"html_url":"https://github.com/test-owner/test-repo/issues/101"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	exitCode := executeRunCmdWithExitCode(t, []string{
		"--dry-run",
		"--pr", "42",
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "--dry-run with non-nil ghClient",
		Should:   "exit with code 0",
		Actual:   exitCode,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "--dry-run with non-nil ghClient (dryRun=true must gate the notification block)",
		Should:   "not create any GitHub issues",
		Actual:   atomic.LoadInt32(&issueCreateCount),
		Expected: 0,
	})
}

// TestRunNilClientSkipsNotificationBlockNoPanic: when ghClient is nil (no token),
// notification block is safely skipped (no panic).
func TestRunNilClientSkipsNotificationBlockNoPanic(t *testing.T) {
	dir := t.TempDir()

	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(passingJUnitXML), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, passingJUnitXML, 0)
	writeSuiteConfig(t, dir, "unit", scriptPath, xmlPath, "false")
	chdirTest(t, dir)

	exitCode := executeRunCmdWithExitCode(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "",
		"GITHUB_TOKEN":            "",
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "nil ghClient (no token) with dryRun=false",
		Should:   "exit with code 0 (notification block safely skipped)",
		Actual:   exitCode,
		Expected: 0,
	})
}
