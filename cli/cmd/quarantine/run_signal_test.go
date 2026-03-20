package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 70: Signal forwarding to child process ---

func TestRunSignalForwardingSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("signal test skipped in short mode")
	}

	dir := t.TempDir()
	// Script: sleep briefly then exit — so it's interruptible.
	// We'll send SIGTERM to the CLI process and verify the child exits cleanly.
	scriptPath := filepath.Join(dir, "slow-test")
	// Script that writes its PID to a file, then sleeps.
	pidFile := filepath.Join(dir, "child.pid")
	script := fmt.Sprintf(`#!/bin/sh
echo $$ > %q
sleep 30
exit 0
`, pidFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	configPath := writeTempConfig(t, `
version: 1
framework: jest
`)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	// Run the command in a subprocess so we can send signals to it.
	binary := buildTestBinary(t)
	cmd := exec.Command(binary,
		"run",
		"--config", configPath,
		"--junitxml", filepath.Join(dir, "junit.xml"),
		"--output", filepath.Join(dir, "results.json"),
		"--", scriptPath,
	)
	cmd.Env = append(os.Environ(),
		"QUARANTINE_GITHUB_TOKEN=ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL="+server.URL,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for child script to write its PID.
	var childPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil && len(data) > 0 {
			_, _ = fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &childPID)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "child process started",
		Should:   "write PID file",
		Actual:   childPID > 0,
		Expected: true,
	})

	// Send SIGTERM to the CLI process.
	_ = cmd.Process.Signal(syscall.SIGTERM)
	waitErr := cmd.Wait()

	// The CLI should have exited — check exit code is 143 (128+SIGTERM) or similar.
	exitCode := 0
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SIGTERM sent to CLI process",
		Should:   "exit with non-zero code (signal-based exit)",
		Actual:   exitCode != 0,
		Expected: true,
	})

	// Child process should no longer be running.
	if childPID > 0 {
		childRunning := isProcessRunning(childPID)
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "SIGTERM forwarded to child",
			Should:   "child process should have exited",
			Actual:   childRunning,
			Expected: false,
		})
	}
}

// buildTestBinary compiles the CLI to a temp binary for subprocess testing.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "quarantine-test")
	out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

// isProcessRunning returns true if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
