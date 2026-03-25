package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mycargus/quarantine/internal/cas"
	"github.com/mycargus/quarantine/internal/config"
	"github.com/mycargus/quarantine/internal/git"
	gh "github.com/mycargus/quarantine/internal/github"
	"github.com/mycargus/quarantine/internal/parser"
	qstate "github.com/mycargus/quarantine/internal/quarantine"
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

	runStart := time.Now()
	defer func() {
		if verbose {
			cmd.PrintErrf("[verbose] Total time: %dms\n", time.Since(runStart).Milliseconds())
		}
	}()

	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	// Snapshot pre-defaults values for verbose source attribution.
	cfgRetries := cfg.Retries
	cfgJUnitXML := cfg.JUnitXML

	// Read CLI flags before ApplyDefaults so we know what was explicitly set.
	junitxmlFlag, _ := cmd.Flags().GetString("junitxml")
	retriesFlag, _ := cmd.Flags().GetInt("retries")

	cfg.ApplyDefaults()

	// Apply CLI flag overrides (after defaults, so flags win).
	if junitxmlFlag != "" {
		cfg.JUnitXML = junitxmlFlag
	}
	if retriesFlag != 0 {
		if retriesFlag < 1 || retriesFlag > 10 {
			cmd.PrintErrf("Error: --retries value %d is out of range. Must be between 1 and 10.\n", retriesFlag)
			return fmt.Errorf("retries out of range")
		}
		cfg.Retries = retriesFlag
	}

	if verbose {
		for _, line := range configResolutionTrace(cfg, retriesFlag, cfgRetries, junitxmlFlag, cfgJUnitXML) {
			cmd.PrintErrln(line)
		}
	}

	// Verify test command exists.
	testCmd := args[0]
	testArgs := args[1:]
	if _, err := exec.LookPath(testCmd); err != nil {
		cmd.PrintErrf("Error: command not found: %q. Ensure the test runner is installed and on PATH.\n", testCmd)
		return fmt.Errorf("command not found: %s", testCmd)
	}

	// Check quarantine/state branch exists.
	check, err := checkBranchExists(cfg)
	if err != nil {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return err
	}
	if check.warnMsg != "" {
		cmd.PrintErrf("[quarantine] WARNING: %s\n", check.warnMsg)
	}
	if verbose {
		if check.skipped {
			cmd.PrintErrf("[verbose] API call: skipped (no token)\n")
		} else if check.endpoint != "" {
			elapsedMs := check.elapsed.Milliseconds()
			if check.apiErr != nil {
				cmd.PrintErrf("[verbose] API call: %s -> error (%dms)\n", check.endpoint, elapsedMs)
			} else if check.exists {
				cmd.PrintErrf("[verbose] API call: %s -> 200 (%dms)\n", check.endpoint, elapsedMs)
			} else {
				cmd.PrintErrf("[verbose] API call: %s -> 404 (%dms)\n", check.endpoint, elapsedMs)
			}
		}
	}
	if !check.skipped && check.apiErr == nil && !check.exists {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	ctx := context.Background()

	strict, _ := cmd.Flags().GetBool("strict")

	// Create a single GitHub client shared across all quarantine state operations.
	// Rate-limit warnings are forwarded to stderr via cmd.PrintErrln.
	var ghClient *gh.Client
	if owner, repo := resolveOwnerRepo(cfg); owner != "" && repo != "" {
		if c, clientErr := gh.NewClient(owner, repo); clientErr == nil {
			c.SetRateLimitWarningFunc(func(msg string) { cmd.PrintErrln(msg) })
			ghClient = c
		} else {
			// Detect missing token specifically for a clearer message.
			reason := fmt.Sprintf("%v", clientErr)
			if strings.Contains(reason, "no GitHub token") {
				reason = "No GitHub token found. Running in degraded mode."
			}
			if strict {
				cmd.PrintErrf("[quarantine] ERROR: infrastructure failure (--strict mode): %v\n", clientErr)
				cmd.PrintErrf("[quarantine] ERROR: exiting with code 2. Remove --strict to run in degraded mode.\n")
				return exitCodeError(2)
			}
			emitDegradedWarning(cmd, reason)
		}
	}

	// Read quarantine state from the quarantine/state branch.
	qState, qStateContent, qStateSHA := loadQuarantineState(ctx, cmd, cfg, ghClient)

	// In --strict mode, a nil state from a non-nil client means the API was unreachable.
	if strict && ghClient != nil && qState == nil {
		cmd.PrintErrf("[quarantine] ERROR: infrastructure failure (--strict mode): unable to read quarantine state\n")
		cmd.PrintErrf("[quarantine] ERROR: exiting with code 2. Remove --strict to run in degraded mode.\n")
		return exitCodeError(2)
	}

	// Batch-check closed issues and remove unquarantined tests from in-memory state.
	var removedTestIDs []string
	if qState != nil {
		removedTestIDs = removeUnquarantinedTests(ctx, cmd, cfg, qState, ghClient)
	}

	// Augment the test command with framework-specific exclusion flags.
	if qState != nil && qState.Tests != nil {
		exclusionArgs := buildExclusionArgsFromState(runner.Framework(cfg.Framework), qState)
		if len(exclusionArgs) > 0 {
			testArgs = append(testArgs, exclusionArgs...)
		}
	}

	// Execute test command.
	if !quiet {
		cmd.PrintErrf("[quarantine] Running: %s %s\n", testCmd, strings.Join(testArgs, " "))
	}

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

	// For RSpec, apply post-execution filtering to suppress quarantined failures.
	if qState != nil && runner.Framework(cfg.Framework) == runner.RSpec {
		testResults = qstate.FilterQuarantinedFailures(testResults, qState)
	}

	// Collect exclude patterns: merge config and CLI flag values.
	excludeFlag, _ := cmd.Flags().GetStringArray("exclude")
	excludePatterns := mergeExcludePatterns(cfg.Exclude, excludeFlag)

	// Retry failing tests individually (skip quarantined and excluded tests).
	retryOutcomes := retryFailingTests(ctx, testResults, cfg, excludePatterns)

	// Build results.
	meta := buildMetadata(cfg)
	res := result.BuildWithRetries(testResults, retryOutcomes, meta)

	// Add newly detected flaky tests to quarantine state and write via CAS.
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if qState != nil && !dryRun {
		flakyAdded := addNewFlakyTests(qState, res, excludePatterns)
		stateChanged := flakyAdded || len(removedTestIDs) > 0
		if stateChanged {
			writeUpdatedQuarantineState(ctx, cmd, cfg, qState, qStateContent, qStateSHA, ghClient, removedTestIDs)
		}
	} else if dryRun && res.Summary.FlakyDetected > 0 {
		printDryRunSummary(cmd, res)
	}

	// Create GitHub issues for newly detected flaky tests and post PR comment.
	if !dryRun && ghClient != nil {
		prFlag, _ := cmd.Flags().GetInt("pr")
		eventPath := os.Getenv("GITHUB_EVENT_PATH")
		prNumber, _ := detectPRNumber(prFlag, eventPath)

		branch := meta.Branch
		commitSHA := meta.CommitSHA

		issueRefs := createIssuesForNewFlakyTests(ctx, cmd, ghClient, res, excludePatterns, branch, commitSHA, prNumber)

		prCommentEnabled := cfg.Notifications.GitHubPRComment == nil || *cfg.Notifications.GitHubPRComment
		if prCommentEnabled {
			flakyEntries := buildFlakyEntries(res, issueRefs)
			commentData := PRCommentData{
				Total:         res.Summary.Total,
				Passed:        res.Summary.Passed,
				Failed:        res.Summary.Failed,
				Flaky:         res.Summary.FlakyDetected,
				Quarantined:   res.Summary.Quarantined,
				Unquarantined: len(removedTestIDs),
				Version:       version,
				NewlyFlaky:    flakyEntries,
			}
			commentBody := renderPRComment(commentData)
			postOrUpdatePRComment(ctx, cmd, ghClient, prNumber, commentBody)
		}
	}

	// Warn when all tests are quarantined (nothing meaningful ran or all suppressed).
	// Only fire when quarantine state was loaded and had entries — otherwise
	// Total==0 could just mean an empty test suite or degraded mode.
	hadQuarantinedTests := qState != nil && len(qState.Tests) > 0
	if hadQuarantinedTests && allTestsQuarantined(res) {
		cmd.PrintErrln("[quarantine] WARNING: All tests are quarantined. The entire test suite was skipped. Review and close resolved quarantine issues.")
	}

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
		cmd.PrintErrf("  Flaky:           %d\n", res.Summary.FlakyDetected)
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

// loadQuarantineState reads the quarantine.json file from the quarantine/state branch.
// Returns nil state on error (degraded mode — operation continues without quarantine awareness).
// Also returns raw content and SHA for subsequent CAS writes.
// client may be nil (no token / not configured) — degraded mode in that case.
func loadQuarantineState(ctx context.Context, cmd *cobra.Command, cfg *config.Config, client *gh.Client) (*qstate.State, []byte, string) {
	if client == nil {
		return nil, nil, ""
	}

	branch := cfg.Storage.Branch
	content, sha, err := client.GetContents(ctx, "quarantine.json", branch)
	if err != nil {
		emitDegradedWarning(cmd, fmt.Sprintf("Unable to reach GitHub API and no cached quarantine state available. Running without quarantine exclusions. (reason: %v)", err))
		return nil, nil, ""
	}

	if len(content) == 0 {
		// File not found — use empty state.
		return qstate.NewEmptyState(), nil, ""
	}

	state, err := qstate.ParseState(bytes.NewReader(content))
	if err != nil {
		cmd.PrintErrf("[quarantine] WARNING: Cannot parse quarantine state: %v. Continuing in degraded mode.\n", err)
		return nil, nil, ""
	}

	return state, content, sha
}

// removeUnquarantinedTests batch-checks issue status and removes tests whose
// issues are closed from the in-memory quarantine state.
// Returns the list of test IDs that were removed.
// client may be nil — in that case the check is skipped (state unchanged, empty slice returned).
func removeUnquarantinedTests(ctx context.Context, cmd *cobra.Command, cfg *config.Config, state *qstate.State, client *gh.Client) []string {
	if client == nil {
		return nil
	}

	label := "quarantine"
	if len(cfg.Labels) > 0 {
		label = cfg.Labels[0]
	}

	closedNumbers, truncated, totalCount, searchErr := client.SearchClosedIssues(ctx, label)
	if searchErr != nil {
		cmd.PrintErrf("[quarantine] WARNING: Cannot check issue status: %v. Quarantine state unchanged.\n", searchErr)
		return nil
	}
	if truncated {
		cmd.PrintErrf("[quarantine] WARNING: GitHub Search returned 1,000 closed issues but total_count is %d. Some issues may not be checked. Consider cleaning up old quarantine issues to stay within the 1,000 result limit.\n", totalCount)
	}

	closedSet := make(map[int]bool, len(closedNumbers))
	for _, n := range closedNumbers {
		closedSet[n] = true
	}

	var removed []string
	for testID, entry := range state.Tests {
		if entry.IssueNumber != nil && closedSet[*entry.IssueNumber] {
			state.RemoveTest(testID)
			removed = append(removed, testID)
		}
	}
	return removed
}

// buildExclusionArgsFromState returns the framework-specific exclusion flags
// for tests in the quarantine state. Pure logic wrapped in I/O context.
// This is a thin adapter — pure function is in runner.BuildExclusionArgs.
func buildExclusionArgsFromState(fw runner.Framework, state *qstate.State) []string {
	entries := make([]qstate.Entry, 0, len(state.Tests))
	for _, e := range state.Tests {
		entries = append(entries, e)
	}
	return runner.BuildExclusionArgs(fw, entries)
}

// addNewFlakyTests adds newly detected flaky tests from the run results to the
// quarantine state. A test is newly flaky if it is not already in the state
// and its result status is "flaky". Tests matching excludePatterns are skipped.
// Returns true if the state was modified.
func addNewFlakyTests(state *qstate.State, res result.Result, excludePatterns []string) bool {
	changed := false
	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range res.Tests {
		if t.Status != "flaky" {
			continue
		}
		if runner.MatchesExcludePattern(t.TestID, excludePatterns) {
			continue
		}
		if state.HasTest(t.TestID) {
			// Already quarantined — update last_flaky_at and flaky count.
			entry := state.Tests[t.TestID]
			entry.LastFlakyAt = now
			entry.FlakyCount++
			state.AddTest(entry)
			changed = true
			continue
		}
		// New flaky test — add to state.
		changed = true
		state.AddTest(qstate.Entry{
			TestID:        t.TestID,
			FilePath:      t.FilePath,
			Classname:     t.Classname,
			Name:          t.Name,
			FirstFlakyAt:  now,
			LastFlakyAt:   now,
			FlakyCount:    1,
			QuarantinedAt: now,
			QuarantinedBy: "auto",
		})
	}
	return changed
}

// writeUpdatedQuarantineState writes the updated quarantine state to GitHub via CAS.
// On error, logs a warning (degraded mode — never breaks the build).
// removedTestIDs is the list of test IDs that were unquarantined this run; used to
// detect re-quarantine after CAS conflict and emit a warning per ADR-012.
// client may be nil — in that case the write is skipped silently.
func writeUpdatedQuarantineState(ctx context.Context, cmd *cobra.Command, cfg *config.Config, state *qstate.State, originalContent []byte, sha string, client *gh.Client, removedTestIDs []string) {
	if client == nil {
		return
	}

	content, marshalErr := state.Marshal()
	if marshalErr != nil {
		cmd.PrintErrf("[quarantine] WARNING: Cannot marshal quarantine state: %v\n", marshalErr)
		return
	}

	// If content is unchanged, skip the write.
	if bytes.Equal(content, originalContent) {
		return
	}

	branch := cfg.Storage.Branch
	reQuarantined, writeErr := cas.WriteStateWithCAS(ctx, client, state, content, sha, branch, 3, removedTestIDs)
	if writeErr != nil {
		var apiErr *gh.APIError
		switch {
		case errors.As(writeErr, &apiErr) && apiErr.StatusCode == 403:
			cmd.PrintErrf("[quarantine] WARNING: Branch '%s' is protected. Quarantine state saved to Actions cache. A workflow with write access must apply the update.\n", branch)
		case errors.As(writeErr, &apiErr) && apiErr.StatusCode == 422:
			cmd.PrintErrf("[quarantine] WARNING: quarantine.json exceeds the 1 MB GitHub Contents API limit. Review and close resolved quarantine issues to reduce size. Skipping state update.\n")
		case errors.Is(writeErr, cas.ErrCASExhausted):
			cmd.PrintErrf("[quarantine] WARNING: Failed to update quarantine.json after 3 attempts (concurrent write conflicts). The new flaky test will be re-detected on the next run.\n")
		default:
			cmd.PrintErrf("[quarantine] WARNING: Cannot write quarantine state: %v\n", writeErr)
		}
	}
	for _, testID := range reQuarantined {
		cmd.PrintErrf("[quarantine] WARNING: Test '%s' was unquarantined (issue closed) but re-quarantined due to concurrent update. It will be unquarantined on the next build.\n", testID)
	}
}

// retryFailingTests reruns each failed test individually up to cfg.Retries times.
// Returns a map of TestID -> RetryOutcome for tests that were retried.
// Exits the retry loop early for each test on the first passing attempt.
// Tests whose TestID matches any excludePattern are skipped entirely.
func retryFailingTests(ctx context.Context, tests []parser.TestResult, cfg *config.Config, excludePatterns []string) map[string]result.RetryOutcome {
	outcomes := make(map[string]result.RetryOutcome)

	for _, t := range tests {
		if t.Status != "failed" && t.Status != "error" {
			continue
		}
		if runner.MatchesExcludePattern(t.TestID, excludePatterns) {
			continue
		}

		rerunCmd, rerunArgs := runner.RerunCommand(
			runner.Framework(cfg.Framework),
			t.Name, t.Classname, t.FilePath,
			cfg.RerunCommand,
		)

		var attempts []result.RetryEntry
		for attempt := 1; attempt <= cfg.Retries; attempt++ {
			rerunExitCode, runErr := runner.Run(ctx, rerunCmd, rerunArgs, os.Stdout, os.Stderr)
			if runErr != nil {
				attempts = append(attempts, result.RetryEntry{
					Attempt: attempt,
					Status:  "failed",
				})
				continue
			}
			status := "failed"
			if rerunExitCode == 0 {
				status = "passed"
			}
			attempts = append(attempts, result.RetryEntry{
				Attempt: attempt,
				Status:  status,
			})
			if status == "passed" {
				break
			}
		}

		outcomes[t.TestID] = result.RetryOutcome{Attempts: attempts}
	}

	return outcomes
}

// branchCheckResult holds the result of a branch existence check.
type branchCheckResult struct {
	skipped  bool          // true if check was skipped (no token — degraded mode)
	warnMsg  string        // non-empty if a [quarantine] WARNING should be emitted
	exists   bool          // true if the branch was found
	elapsed  time.Duration // round-trip time for the API call
	endpoint string        // e.g. "GET /repos/owner/repo/git/ref/heads/quarantine/state"
	apiErr   error         // non-fatal GetRef error
}

// resolveOwnerRepo returns the GitHub owner and repo from config, falling back
// to the git remote in the current working directory.
func resolveOwnerRepo(cfg *config.Config) (owner, repo string) {
	owner, repo = cfg.GitHub.Owner, cfg.GitHub.Repo
	if owner == "" || repo == "" {
		if cwd, err := os.Getwd(); err == nil {
			owner, repo, _ = git.ParseRemote(cwd)
		}
	}
	return owner, repo
}

// checkBranchExists verifies the quarantine/state branch exists via GitHub API.
// Returns (zero, error) for fatal configuration errors (owner/repo unresolvable).
// Returns (result, nil) for successful checks or degraded mode (no token, API error).
// Callers are responsible for printing warnings and verbose output from the result.
func checkBranchExists(cfg *config.Config) (branchCheckResult, error) {
	owner, repo := resolveOwnerRepo(cfg)

	if owner == "" || repo == "" {
		return branchCheckResult{}, fmt.Errorf("not initialized")
	}

	client, clientErr := gh.NewClient(owner, repo)
	if clientErr != nil {
		return branchCheckResult{
			skipped: true,
			warnMsg: clientErr.Error(),
		}, nil
	}

	branch := cfg.Storage.Branch
	endpoint := fmt.Sprintf("GET /repos/%s/%s/git/ref/heads/%s", owner, repo, branch)

	ctx := context.Background()
	apiStart := time.Now()
	_, exists, apiErr := client.GetRef(ctx, branch)
	elapsed := time.Since(apiStart)

	if apiErr != nil {
		return branchCheckResult{
			endpoint: endpoint,
			elapsed:  elapsed,
			apiErr:   apiErr,
			warnMsg:  fmt.Sprintf("Could not check branch '%s': %v", branch, apiErr),
		}, nil
	}

	return branchCheckResult{
		endpoint: endpoint,
		elapsed:  elapsed,
		exists:   exists,
	}, nil
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
	owner, repo := resolveOwnerRepo(cfg)
	branch := getEnvOrGit("GITHUB_REF_NAME", "git", "rev-parse", "--abbrev-ref", "HEAD")
	commitSHA := getEnvOrGit("GITHUB_SHA", "git", "rev-parse", "HEAD")
	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" {
		runID = fmt.Sprintf("local-%d", time.Now().UnixNano())
	}
	return assembleMetadata(owner, repo, branch, commitSHA, runID, cfg.Framework, cfg.Retries)
}

// assembleMetadata builds a Metadata struct from pre-resolved values.
// This is a pure function — no I/O.
func assembleMetadata(owner, repo, branch, commitSHA, runID, framework string, retries int) result.Metadata {
	return result.Metadata{
		RunID:      runID,
		Repo:       repoString(owner, repo),
		Branch:     branch,
		CommitSHA:  commitSHA,
		CLIVersion: version,
		Framework:  framework,
		RetryCount: retries,
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

// mergeExcludePatterns combines exclude patterns from the config file and
// CLI --exclude flags into a single slice.
// This is a pure function — no I/O.
func mergeExcludePatterns(fromConfig, fromFlags []string) []string {
	merged := make([]string, 0, len(fromConfig)+len(fromFlags))
	merged = append(merged, fromConfig...)
	merged = append(merged, fromFlags...)
	return merged
}

// printDryRunSummary prints the dry-run summary to stderr listing each test
// that would have been quarantined.
func printDryRunSummary(cmd *cobra.Command, res result.Result) {
	cmd.PrintErrf("[quarantine] DRY RUN — no changes written.\n")
	for _, t := range res.Tests {
		if t.Status == "flaky" {
			cmd.PrintErrf("  Would quarantine: %s\n", t.Name)
		}
	}
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

// configResolutionTrace returns verbose log lines describing how each config field
// was resolved. This is a pure function — no I/O.
//
// Parameters:
//   - cfg: config after ApplyDefaults and flag overrides have been applied
//   - retriesFlag: value from --retries flag (0 = not set)
//   - cfgRetries: cfg.Retries before ApplyDefaults (0 = not set in file)
//   - junitxmlFlag: value from --junitxml flag ("" = not set)
//   - cfgJUnitXML: cfg.JUnitXML before ApplyDefaults ("" = not set in file)
func configResolutionTrace(cfg *config.Config, retriesFlag, cfgRetries int, junitxmlFlag, cfgJUnitXML string) []string {
	retriesSource := "default"
	if retriesFlag != 0 {
		retriesSource = "flag"
	} else if cfgRetries != 0 {
		retriesSource = "config"
	}

	junitxmlSource := "default"
	if junitxmlFlag != "" {
		junitxmlSource = "flag"
	} else if cfgJUnitXML != "" {
		junitxmlSource = "config"
	}

	return []string{
		"[verbose] Config resolution:",
		fmt.Sprintf("[verbose]   framework = %s (source: config)", cfg.Framework),
		fmt.Sprintf("[verbose]   retries   = %d (source: %s)", cfg.Retries, retriesSource),
		fmt.Sprintf("[verbose]   junitxml  = %s (source: %s)", cfg.JUnitXML, junitxmlSource),
	}
}

// emitDegradedWarning prints a [quarantine] WARNING to stderr and, when running
// inside GitHub Actions (GITHUB_ACTIONS=true), also emits a GHA workflow annotation.
func emitDegradedWarning(cmd *cobra.Command, reason string) {
	cmd.PrintErrf("[quarantine] WARNING: %s\n", reason)
	if os.Getenv("GITHUB_ACTIONS") != "" {
		cmd.PrintErrf("::warning title=Quarantine Degraded Mode::%s\n", reason)
	}
}

// allTestsQuarantined reports whether the result indicates that every test in
// the suite was accounted for by quarantine — either because exclusion flags
// caused no tests to run (Total == 0, Jest case) or because post-execution
// filtering suppressed all results (Total > 0 && Quarantined == Total, RSpec case).
// This is a pure function — no I/O.
func allTestsQuarantined(res result.Result) bool {
	if res.Summary.Total == 0 {
		return true
	}
	return res.Summary.Total > 0 && res.Summary.Quarantined == res.Summary.Total
}

// exitCodeError is an error that carries a specific exit code.
type exitCodeError int

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}
