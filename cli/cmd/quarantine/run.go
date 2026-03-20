package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mycargus/quarantine/internal/config"
	"github.com/mycargus/quarantine/internal/git"
	gh "github.com/mycargus/quarantine/internal/github"
	"github.com/mycargus/quarantine/internal/parser"
	"github.com/mycargus/quarantine/internal/result"
	"github.com/mycargus/quarantine/internal/runner"
	"github.com/spf13/cobra"
)

const separatorErrorMsg = "Error: missing '--' separator. Usage: quarantine run [flags] -- <test command>\n\nExample: quarantine run --retries 3 -- jest --ci"

// runRun implements the `quarantine run` command logic.
func runRun(cmd *cobra.Command, args []string) error {
	// Check for -- separator.
	if cmd.ArgsLenAtDash() == -1 {
		cmd.PrintErrln(separatorErrorMsg)
		return fmt.Errorf("missing separator")
	}

	if len(args) == 0 {
		cmd.PrintErrln(separatorErrorMsg)
		return fmt.Errorf("missing test command")
	}

	// Check mutually exclusive flags.
	verbose, _ := cmd.Flags().GetBool("verbose")
	quiet, _ := cmd.Flags().GetBool("quiet")
	if verbose && quiet {
		cmd.PrintErrln("Error: --verbose and --quiet are mutually exclusive.")
		return fmt.Errorf("verbose and quiet are mutually exclusive")
	}

	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	cfg.ApplyDefaults()

	// Apply CLI flag overrides.
	if junitxmlFlag, _ := cmd.Flags().GetString("junitxml"); junitxmlFlag != "" {
		cfg.JUnitXML = junitxmlFlag
	}
	if retriesFlag, _ := cmd.Flags().GetInt("retries"); retriesFlag != 0 {
		cfg.Retries = retriesFlag
	}

	// Verify test command exists.
	testCmd := args[0]
	testArgs := args[1:]
	if _, err := exec.LookPath(testCmd); err != nil {
		cmd.PrintErrf("Error: command not found: %q. Ensure the test runner is installed and on PATH.\n", testCmd)
		return fmt.Errorf("command not found: %s", testCmd)
	}

	// Check quarantine/state branch exists.
	if err := checkBranchExists(cmd, cfg); err != nil {
		return err
	}

	// Execute test command.
	if !quiet {
		cmd.PrintErrf("[quarantine] Running: %s %s\n", testCmd, strings.Join(testArgs, " "))
	}

	ctx := context.Background()
	exitCode, runErr := runner.Run(ctx, testCmd, testArgs, os.Stdout, os.Stderr)
	if runErr != nil {
		cmd.PrintErrf("[quarantine] WARNING: Failed to execute test command: %v\n", runErr)
		return fmt.Errorf("test command execution failed")
	}

	// Parse JUnit XML.
	testResults, parseWarnings := parseJUnitXML(cfg.JUnitXML)
	for _, w := range parseWarnings {
		cmd.PrintErrf("[quarantine] WARNING: %s\n", w)
	}

	// If no XML found, warn and exit with runner's code.
	if testResults == nil && exitCode != 0 {
		cmd.PrintErrf("[quarantine] WARNING: No JUnit XML found at '%s'. Cannot determine test results. Suggest checking --junitxml flag or test runner configuration.\n", cfg.JUnitXML)
		return exitCodeError(exitCode)
	}

	if testResults == nil {
		testResults = []parser.TestResult{}
	}

	// Build results.
	meta := buildMetadata(cfg)
	res := result.Build(testResults, meta)

	// Write results.json.
	outputPath, _ := cmd.Flags().GetString("output")
	if err := writeResults(res, outputPath); err != nil {
		cmd.PrintErrf("[quarantine] WARNING: Failed to write results: %v\n", err)
	} else if !quiet {
		cmd.PrintErrf("[quarantine] Results written to %s\n", outputPath)
	}

	// Print summary (unless quiet).
	if !quiet {
		cmd.PrintErrf("\n[quarantine] Results:\n")
		cmd.PrintErrf("  Total:           %d\n", res.Summary.Total)
		cmd.PrintErrf("  Passed:          %d\n", res.Summary.Passed)
		cmd.PrintErrf("  Failed:          %d\n", res.Summary.Failed)
		cmd.PrintErrf("  Skipped:         %d\n", res.Summary.Skipped)
	}

	// Determine exit code based on test results.
	if res.Summary.Failed > 0 {
		return exitCodeError(1)
	}

	// If the test runner exited non-zero but we parsed 0 failures,
	// still respect the runner's exit code.
	if exitCode != 0 && len(testResults) == 0 {
		return exitCodeError(exitCode)
	}

	return nil
}

// checkBranchExists verifies the quarantine/state branch exists via GitHub API.
func checkBranchExists(cmd *cobra.Command, cfg *config.Config) error {
	owner, repo := cfg.GitHub.Owner, cfg.GitHub.Repo
	if owner == "" || repo == "" {
		if cwd, err := os.Getwd(); err == nil {
			owner, repo, _ = git.ParseRemote(cwd)
		}
	}

	if owner == "" || repo == "" {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	client, err := gh.NewClient(owner, repo)
	if err != nil {
		// No token — proceed if we can (degraded mode for branch check).
		cmd.PrintErrf("[quarantine] WARNING: %v\n", err)
		return nil
	}

	ctx := context.Background()
	branch := cfg.Storage.Branch
	_, exists, err := client.GetRef(ctx, branch)
	if err != nil {
		cmd.PrintErrf("[quarantine] WARNING: Could not check branch '%s': %v\n", branch, err)
		return nil
	}

	if !exists {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	return nil
}

// parseAttempt holds the outcome of attempting to parse one JUnit XML file.
type parseAttempt struct {
	results []parser.TestResult // non-nil on success
	warning string              // non-empty on failure
}

// parseJUnitXML resolves the junitxml glob and parses all matching files.
// Returns parsed results and any warnings. Returns nil results if no files found.
func parseJUnitXML(pattern string) ([]parser.TestResult, []string) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, []string{fmt.Sprintf("Invalid glob pattern '%s': %v", pattern, err)}
	}

	if len(matches) == 0 {
		return nil, nil
	}

	var attempts []parseAttempt
	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			attempts = append(attempts, parseAttempt{warning: fmt.Sprintf("Failed to open %s: %v", path, err)})
			continue
		}
		results, err := parser.Parse(f)
		_ = f.Close()
		if err != nil {
			attempts = append(attempts, parseAttempt{warning: fmt.Sprintf("Failed to parse %s: %v. Skipping.", path, err)})
			continue
		}
		attempts = append(attempts, parseAttempt{results: results})
	}

	return mergeParseResults(attempts)
}

// mergeParseResults combines results from multiple parse attempts and emits a
// summary warning when any files were skipped.
// This is a pure function — no I/O.
func mergeParseResults(attempts []parseAttempt) ([]parser.TestResult, []string) {
	var allResults []parser.TestResult
	var warnings []string
	succeeded := 0
	total := len(attempts)

	for _, a := range attempts {
		if a.warning != "" {
			warnings = append(warnings, a.warning)
			continue
		}
		allResults = append(allResults, a.results...)
		succeeded++
	}

	if succeeded < total {
		warnings = append(warnings, fmt.Sprintf("Parsed %d/%d JUnit XML files. %d malformed, skipped.", succeeded, total, total-succeeded))
	}

	if succeeded == 0 && total > 0 {
		return nil, warnings
	}
	return allResults, warnings
}

// buildMetadata constructs result metadata from config and environment.
func buildMetadata(cfg *config.Config) result.Metadata {
	owner, repo := cfg.GitHub.Owner, cfg.GitHub.Repo
	if owner == "" || repo == "" {
		if cwd, err := os.Getwd(); err == nil {
			owner, repo, _ = git.ParseRemote(cwd)
		}
	}

	repoStr := repoString(owner, repo)

	branch := getEnvOrGit("GITHUB_REF_NAME", "git", "rev-parse", "--abbrev-ref", "HEAD")
	commitSHA := getEnvOrGit("GITHUB_SHA", "git", "rev-parse", "HEAD")
	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" {
		runID = fmt.Sprintf("local-%d", time.Now().UnixNano())
	}

	return result.Metadata{
		RunID:      runID,
		Repo:       repoStr,
		Branch:     branch,
		CommitSHA:  commitSHA,
		CLIVersion: version,
		Framework:  cfg.Framework,
		RetryCount: cfg.Retries,
	}
}

// getEnvOrGit returns an env var value, or falls back to running a git command.
func getEnvOrGit(envVar string, gitCmd ...string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if len(gitCmd) > 1 {
		out, err := exec.Command(gitCmd[0], gitCmd[1:]...).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// repoString returns "owner/repo" or empty string if either part is absent.
// This is a pure function — no I/O.
func repoString(owner, repo string) string {
	if owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}

// writeResults writes the result JSON to the given path, creating directories
// as needed.
func writeResults(res result.Result, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// exitCodeError is an error that carries a specific exit code.
type exitCodeError int

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}
