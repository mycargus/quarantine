package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/config"
	riteway "github.com/mycargus/riteway-golang"
)

// executeRunCmd is a test helper that runs the run command with given args,
// capturing output and returning the exit error.
func executeRunCmd(t *testing.T, args []string, env map[string]string) (output string, exitErr error) {
	t.Helper()

	// Clear GITHUB_EVENT_PATH by default so CI's event payload doesn't
	// cause unexpected PR-comment requests during unit tests.
	if _, ok := env["GITHUB_EVENT_PATH"]; !ok {
		t.Setenv("GITHUB_EVENT_PATH", "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs(append([]string{"run"}, args...))
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// fakeGitHubAPI returns an httptest server that responds to the branch check
// endpoint. If branchExists is true, it returns 200; otherwise 404.
func fakeGitHubAPI(t *testing.T, branchExists bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state") {
			if branchExists {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// writeSuiteConfigFull creates a suite config with custom owner, repo, and rerun command.
func writeSuiteConfigFull(t *testing.T, dir, owner, repo, xmlPath, scriptPath, rerunScriptPath string) string {
	t.Helper()
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("writeSuiteConfigFull: mkdir: %v", err)
	}
	configPath := filepath.Join(suiteDir, "config.yml")
	content := fmt.Sprintf(`version: 1
github:
  owner: %s
  repo: %s
test_suites:
  - name: unit
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s"]
    retries: 3
`, owner, repo, scriptPath, xmlPath, rerunScriptPath)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writeSuiteConfigFull: write config: %v", err)
	}
	return configPath
}

// writeSuiteConfigWithRetries creates a suite config with custom retries count.
func writeSuiteConfigWithRetries(t *testing.T, dir, xmlPath, scriptPath string, retries int) string {
	t.Helper()
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("writeSuiteConfigWithRetries: mkdir: %v", err)
	}
	configPath := filepath.Join(suiteDir, "config.yml")
	content := fmt.Sprintf(`version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["false"]
    retries: %d
`, scriptPath, xmlPath, retries)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writeSuiteConfigWithRetries: write config: %v", err)
	}
	return configPath
}

// writeSuiteConfigWithRerunScript creates a suite config with a custom rerun command script.
func writeSuiteConfigWithRerunScript(t *testing.T, dir, xmlPath, scriptPath, rerunScriptPath string) string {
	t.Helper()
	suiteDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("writeSuiteConfigWithRerunScript: mkdir: %v", err)
	}
	configPath := filepath.Join(suiteDir, "config.yml")
	content := fmt.Sprintf(`version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: unit
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s"]
    retries: 3
`, scriptPath, xmlPath, rerunScriptPath)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writeSuiteConfigWithRerunScript: write config: %v", err)
	}
	return configPath
}

// writeTestScript creates an executable shell script that exits with exitCode.
// It also pre-writes xmlContent to xmlPath before returning so that the
// CLI can parse it after the script runs.
func writeTestScript(t *testing.T, dir, xmlPath, xmlContent string, exitCode int) string {
	t.Helper()
	// Write the XML file directly — simulates the test runner having written it.
	if xmlContent != "" && xmlPath != "" {
		if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
			t.Fatalf("writeTestScript: write xml: %v", err)
		}
	}
	scriptPath := filepath.Join(dir, "fake-test")
	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("writeTestScript: %v", err)
	}
	return scriptPath
}

// executeRunCmdWithExitCode runs the run command and returns the numeric exit
// code. Returns 0 for no error, 1 for exitCodeError(1), 2 for other errors.
func executeRunCmdWithExitCode(t *testing.T, args []string, env map[string]string) int {
	t.Helper()
	// Clear GITHUB_EVENT_PATH by default so CI's event payload doesn't
	// cause unexpected PR-comment requests during unit tests.
	if _, ok := env["GITHUB_EVENT_PATH"]; !ok {
		t.Setenv("GITHUB_EVENT_PATH", "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"run"}, args...))
	err := rootCmd.Execute()
	if err == nil {
		return 0
	}
	if code, ok := err.(exitCodeError); ok {
		return int(code)
	}
	return 2
}

// chdirTest changes the working directory to dir for the duration of the test.
func chdirTest(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

// gitInit creates a bare git repo in dir with a remote origin URL.
func gitInit(t *testing.T, dir, remoteURL string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "--initial-branch=main", dir},
		{"git", "-C", dir, "remote", "add", "origin", remoteURL},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("gitInit %v: %v\n%s", args, err, out)
		}
	}
}

// --- Scenario 12: quarantine run without prior init ---

func TestRunWithoutPriorInit(t *testing.T) {
	dir := t.TempDir()
	chdirTest(t, dir)
	output, err := executeRunCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .quarantine/config.yml exists",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .quarantine/config.yml exists",
		Should:   "print message to run quarantine init first",
		Actual:   strings.Contains(output, "Run 'quarantine init' first"),
		Expected: true,
	})
}

// --- 2A: checkBranchExists with unresolvable owner/repo ---

func TestCheckBranchExistsUnresolvableOwnerRepo(t *testing.T) {
	// Change to a non-git temp directory so ParseRemote also fails.
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := &config.Config{} // empty github.owner / github.repo

	_, err = checkBranchExists(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with no owner/repo and cwd is not a git repo",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config with no owner/repo and cwd is not a git repo",
		Should:   "error message contains 'not initialized'",
		Actual:   strings.Contains(err.Error(), "not initialized"),
		Expected: true,
	})
}
