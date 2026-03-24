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

	"github.com/mycargus/quarantine/internal/config"
	riteway "github.com/mycargus/riteway-golang"
)

// executeRunCmd is a test helper that runs the run command with given args,
// capturing output and returning the exit error.
func executeRunCmd(t *testing.T, args []string, env map[string]string) (output string, exitErr error) {
	t.Helper()

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

// --- Scenario 12: quarantine run without prior init ---

func TestRunWithoutPriorInit(t *testing.T) {
	dir := t.TempDir()
	output, err := executeRunCmd(t, []string{"--config", dir + "/quarantine.yml", "--", "jest", "--ci"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "print message to run quarantine init first",
		Actual:   strings.Contains(output, "Run 'quarantine init' first"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no quarantine.yml exists",
		Should:   "not contain test runner output (test was not executed)",
		Actual:   strings.Contains(output, "jest"),
		Expected: false,
	})
}

// --- Scenario 52: quarantine run without -- separator ---

func TestRunWithoutSeparator(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{"--config", configPath, "jest", "--ci"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "print error about missing separator",
		Actual:   strings.Contains(output, "missing '--' separator"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine run without -- separator",
		Should:   "print usage example",
		Actual:   strings.Contains(output, "quarantine run [flags] -- <test command>"),
		Expected: true,
	})
}

// --- Scenario 52 (variant): -- separator present but no command after it ---

func TestRunEmptyArgsAfterSeparator(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	output, err := executeRunCmd(t, []string{"--config", configPath, "--"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN": "ghp_test",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "-- separator present but no command after it",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "-- separator present but no command after it",
		Should:   "print error about missing test command",
		Actual:   strings.Contains(output, "missing '--' separator"),
		Expected: true,
	})
}

// --- Scenario 53: Test command not found ---

func TestRunCommandNotFound(t *testing.T) {
	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)
	args := []string{"--config", configPath, "--", "jset", "--ci"}
	env := map[string]string{"QUARANTINE_GITHUB_TOKEN": "ghp_test"}

	exitCode := executeRunCmdWithExitCode(t, args, env)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "a typo in the test command (jset instead of jest)",
		Should:   "exit with quarantine error code (2)",
		Actual:   exitCode,
		Expected: 2,
	})

	output, _ := executeRunCmd(t, args, env)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a typo in the test command (jset instead of jest)",
		Should:   "print command not found error with the command name",
		Actual:   strings.Contains(output, `command not found: "jset"`),
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
