package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mycargus/quarantine/cli/internal/cas"
	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/mycargus/quarantine/cli/internal/git"
	gh "github.com/mycargus/quarantine/cli/internal/github"
	"github.com/mycargus/quarantine/cli/internal/parser"
	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	"github.com/mycargus/quarantine/cli/internal/result"
	"github.com/mycargus/quarantine/cli/internal/runner"
	"github.com/spf13/cobra"
)

// runRun implements the `quarantine run` command logic.
func runRun(cmd *cobra.Command, args []string) error {
	cfg, cfgErr := config.Load(".quarantine/config.yml")

	if cfgErr != nil || !cfg.IsSuiteConfig() {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	if cmd.ArgsLenAtDash() != -1 {
		cmd.PrintErrln("Error: the -- separator is not used with suite configs. Usage: quarantine run <suite-name>")
		return exitCodeError(2)
	}

	return runSuiteMode(cmd, args, cfg)
}

// runSuiteMode handles `quarantine run <suite-name>` when the config contains test_suites.
// It selects the named suite, executes its command unmodified via exec.Command, parses
// the suite's JUnit XML output, and exits 0 on success.
func runSuiteMode(cmd *cobra.Command, args []string, cfg *config.Config) error {
	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	suite, err := selectSuite(cfg.TestSuites, nameArg)
	if err != nil {
		cmd.PrintErrln(err.Error())
		return exitCodeError(2)
	}

	suiteName := suite.Name

	suiteCmd := suite.Commands()
	if len(suiteCmd) == 0 {
		cmd.PrintErrf("Error: suite %q has no executable command\n", suiteName)
		return exitCodeError(2)
	}

	// Check quarantine/state branch exists.
	cfg.ApplyDefaults()
	check, checkErr := checkBranchExists(cfg)
	if checkErr != nil {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return checkErr
	}
	if check.warnMsg != "" {
		cmd.PrintErrf("[quarantine] WARNING: %s\n", check.warnMsg)
	}
	if !check.skipped && check.apiErr == nil && !check.exists {
		cmd.PrintErrln("Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.")
		return fmt.Errorf("not initialized")
	}

	ctx := context.Background()

	// Create a shared GitHub client.
	var ghClient *gh.Client
	if owner, repo := resolveOwnerRepo(cfg); owner != "" && repo != "" {
		if c, clientErr := gh.NewClient(owner, repo); clientErr == nil {
			c.SetRateLimitWarningFunc(func(msg string) { cmd.PrintErrln(msg) })
			ghClient = c
		} else {
			reason := fmt.Sprintf("%v", clientErr)
			if strings.Contains(reason, "no GitHub token") {
				reason = "No GitHub token found. Running in degraded mode."
			}
			emitDegradedWarning(cmd, reason)
		}
	}

	// Load quarantine state from the per-suite state path.
	statePath := suiteStatePath(suiteName)
	qState, qStateContent, qStateSHA := loadQuarantineState(ctx, cmd, cfg, ghClient, statePath)

	// Batch-check closed issues and remove unquarantined tests.
	var removedEntries []qstate.Entry
	if qState != nil {
		removedEntries = removeUnquarantinedTests(ctx, cmd, cfg, qState, ghClient)
	}
	removedTestIDs := make([]string, len(removedEntries))
	for i, e := range removedEntries {
		removedTestIDs[i] = e.TestID
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// In --dry-run mode, skip command execution and read existing JUnit XML directly.
	// This lets developers analyze results without re-running tests.
	if dryRun {
		testResults, parseWarnings := parseJUnitXML(suite.JUnitXML)
		for _, w := range parseWarnings {
			cmd.PrintErrf("[quarantine] WARNING: %s\n", w)
		}
		if testResults == nil {
			cmd.PrintErrf("[quarantine] WARNING: No JUnit XML found at '%s'. Cannot determine test results.\n", suite.JUnitXML)
			return nil
		}
		// Print dry-run analysis and exit without writing anything.
		cmd.PrintErrf("[quarantine] DRY RUN — reading existing JUnit XML at '%s'.\n", suite.JUnitXML)
		quarantinedCount := 0
		if qState != nil {
			quarantinedCount = len(qState.Tests)
		}
		nonQuarantinedFailures := 0
		quarantinedFailures := 0
		for _, tr := range testResults {
			if tr.FailureMessage != nil {
				if qState != nil && qState.Tests[tr.TestID].TestID != "" {
					quarantinedFailures++
				} else {
					nonQuarantinedFailures++
				}
			}
		}
		cmd.PrintErrf("[quarantine] DRY RUN analysis:\n")
		cmd.PrintErrf("  Quarantined tests in state: %d\n", quarantinedCount)
		cmd.PrintErrf("  Failures in XML (non-quarantined): %d\n", nonQuarantinedFailures)
		cmd.PrintErrf("  Failures in XML (quarantined): %d\n", quarantinedFailures)
		return nil
	}

	// Execute suite command unmodified via exec.Command.
	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		cmd.PrintErrf("[quarantine] Running: %s\n", strings.Join(suiteCmd, " "))
	}

	// Resolve timeout: suite-level overrides, zero means no timeout.
	const gracePeriod = 5 * time.Second
	timeoutDuration, _ := suite.TimeoutDuration()
	timeoutStr := suite.Timeout

	// --timeout flag overrides the suite config timeout for this invocation only.
	if flagTimeout, _ := cmd.Flags().GetString("timeout"); flagTimeout != "" {
		if override, err := time.ParseDuration(flagTimeout); err == nil {
			timeoutDuration = override
			timeoutStr = flagTimeout
		}
	}

	exitCode, timedOut, runErr := runner.RunWithTimeout(ctx, timeoutDuration, gracePeriod, suiteCmd[0], suiteCmd[1:], os.Stdout, os.Stderr)
	if runErr != nil {
		cmd.PrintErrf("[quarantine] WARNING: Failed to execute test command: %v\n", runErr)
		return fmt.Errorf("test command execution failed")
	}

	// Timeout detection: command was killed by the timeout.
	if timedOut {
		if !xmlFileExists(suite.JUnitXML) {
			cmd.PrintErrf("Error [timeout]: test command timed out after %s and produced no JUnit XML at '%s'.\nCheck that your test runner can start successfully outside of quarantine.\n", timeoutStr, suite.JUnitXML)
			return exitCodeError(2)
		}
		cmd.PrintErrf("Error [timeout]: test command timed out after %s.\n", timeoutStr)
	}

	// Crash detection: if the command exited non-zero and no JUnit XML file was
	// produced, the test runner itself crashed before generating results.
	// (Skip crash detection when the command was killed by timeout — partial XML is expected.)
	if !timedOut && exitCode != 0 && !xmlFileExists(suite.JUnitXML) {
		cmd.PrintErrf("Error [crash]: test command exited with code %d but no JUnit XML files found at '%s'. This usually means the test runner crashed before producing results.\n", exitCode, suite.JUnitXML)
		return exitCodeError(2)
	}

	// Parse JUnit XML using the suite's junitxml path.
	testResults, parseWarnings := parseJUnitXML(suite.JUnitXML)
	for _, w := range parseWarnings {
		cmd.PrintErrf("[quarantine] WARNING: %s\n", w)
	}

	if testResults != nil && timedOut {
		cmd.PrintErrf("Partial results processed: %d tests from %s.\n", len(testResults), suite.JUnitXML)
	}

	if testResults == nil {
		if timedOut {
			return exitCodeError(2)
		}
		if exitCode != 0 {
			cmd.PrintErrf("[quarantine] WARNING: No JUnit XML found at '%s'. Cannot determine test results.\n", suite.JUnitXML)
			return exitCodeError(exitCode)
		}
		cmd.PrintErrf("[quarantine] WARNING: No JUnit XML found at '%s'. Cannot determine test results.\n", suite.JUnitXML)
		return nil
	}

	// Determine retries: suite-level > config-level > default.
	retries := suite.Retries
	if retries == 0 {
		retries = cfg.Retries
	}
	if retries == 0 {
		retries = 3
	}

	rerunCommand := strings.Join(suite.RerunCommand, " ")

	// Resolve rerun_timeout: suite-level only, zero means no timeout.
	rerunTimeoutDuration, _ := suite.RerunTimeoutDuration()

	// Suite mode: rerun command crash is classified as unresolved (infrastructure failure).
	retryOutcomes, retryWarnings, rerunErrors := retryFailingTests(ctx, testResults, retries, rerunCommand, rerunTimeoutDuration, true)
	for _, w := range retryWarnings {
		cmd.PrintErrf("[quarantine] WARNING: %s", w)
	}
	for _, e := range rerunErrors {
		cmd.PrintErrf("%s\n", e)
	}
	if len(rerunErrors) > 0 {
		cmd.PrintErrf("%d test(s) could not be retried — rerun command timed out.\n", len(rerunErrors))
	}

	// Build results using the suite name.
	meta := assembleSuiteMetadata(cfg, suiteName, retries)
	res := result.BuildWithRetries(testResults, retryOutcomes, meta)

	// Skip PR scope checks (will be wired in a later scenario).
	skipReasons := map[string]string{}

	// Write quarantine state.
	if qState != nil && !dryRun {
		flakyAdded := addNewFlakyTests(qState, res, skipReasons)
		stateChanged := flakyAdded || len(removedTestIDs) > 0
		if stateChanged {
			writeUpdatedQuarantineState(ctx, cmd, cfg, qState, qStateContent, qStateSHA, ghClient, removedTestIDs, statePath)
		}
	}

	// Create GitHub issues and post PR comment.
	if !dryRun && ghClient != nil {
		prFlag, _ := cmd.Flags().GetInt("pr")
		eventPath := os.Getenv("GITHUB_EVENT_PATH")
		prNumber, _ := detectPRNumber(prFlag, eventPath)
		branch := meta.Branch
		commitSHA := meta.CommitSHA
		issueRefs := createIssuesForNewFlakyTests(ctx, cmd, ghClient, res, nil, branch, commitSHA, prNumber, skipReasons, suiteName)
		backfillIssueNumbers(res.Tests, issueRefs)

		// Post PR comment with suite-specific marker.
		prCommentEnabled := cfg.Notifications.GitHubPRComment == nil || *cfg.Notifications.GitHubPRComment
		if prNumber > 0 && prCommentEnabled {
			suiteMarker := suitePRCommentMarker(suiteName)
			flakyWithIssue, flakyNewToPR := buildFlakyEntries(res, issueRefs, skipReasons)
			// Use the quarantined count from state (not results summary) because suite mode
			// excludes quarantined tests from the command entirely — they never appear in XML.
			quarantinedStateCount := 0
			if qState != nil {
				quarantinedStateCount = len(qState.Tests)
			}
			commentData := PRCommentData{
				Total:        res.Summary.Total,
				Passed:       res.Summary.Passed,
				Failed:       res.Summary.Failed,
				Flaky:        res.Summary.FlakyDetected,
				Quarantined:  quarantinedStateCount,
				Version:      version,
				NewlyFlaky:   flakyWithIssue,
				NewToPRFlaky: flakyNewToPR,
			}
			commentBody := renderPRComment(commentData, suiteMarker)
			postOrUpdatePRComment(ctx, cmd, ghClient, prNumber, commentBody, suiteMarker)
		}
	}

	// Warn when all tests are quarantined.
	hadQuarantinedTests := qState != nil && len(qState.Tests) > 0
	if hadQuarantinedTests && allTestsQuarantined(res) {
		cmd.PrintErrln("[quarantine] WARNING: All tests are quarantined. The entire test suite was skipped. Review and close resolved quarantine issues.")
	}

	// Write results.json.
	outputPath := fmt.Sprintf(".quarantine/%s/results.json", suiteName)
	if err := writeResults(res, outputPath); err != nil {
		cmd.PrintErrf("[quarantine] WARNING: Failed to write results: %v\n", err)
	} else if !quiet {
		cmd.PrintErrf("[quarantine] Results written to %s\n", outputPath)
	}

	// Print summary.
	if !quiet {
		cmd.PrintErrf("\n[quarantine] Results:\n")
		cmd.PrintErrf("  Total:        %d\n", res.Summary.Total)
		cmd.PrintErrf("  Passed:       %d\n", res.Summary.Passed)
		cmd.PrintErrf("  Failed:       %d\n", res.Summary.Failed)
		cmd.PrintErrf("  Skipped:      %d\n", res.Summary.Skipped)
		cmd.PrintErrf("  Flaky:        %d\n", res.Summary.FlakyDetected)
		cmd.PrintErrf("  Quarantined:  %d\n", res.Summary.Quarantined)
	}

	// When the command timed out, always exit 2 (quarantine infrastructure error),
	// regardless of what the partial results would otherwise indicate.
	if timedOut {
		return exitCodeError(2)
	}

	code := resolveExitCode(res)
	if code != 0 {
		return exitCodeError(code)
	}
	return nil
}

// assembleSuiteMetadata builds result metadata for a named suite run.
func assembleSuiteMetadata(cfg *config.Config, suiteName string, retries int) result.Metadata {
	owner, repo := resolveOwnerRepo(cfg)
	branch := getEnvOrGit("GITHUB_REF_NAME", "git", "rev-parse", "--abbrev-ref", "HEAD")
	commitSHA := getEnvOrGit("GITHUB_SHA", "git", "rev-parse", "HEAD")
	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" {
		runID = fmt.Sprintf("local-%d", time.Now().UnixNano())
	}
	return assembleMetadata(owner, repo, branch, commitSHA, runID, suiteName, retries)
}

// loadQuarantineState reads the state file at statePath from the quarantine/state branch.
// Returns nil state on error (degraded mode — operation continues without quarantine awareness).
// Also returns raw content and SHA for subsequent CAS writes.
// client may be nil (no token / not configured) — degraded mode in that case.
// statePath is the file path within the repo (e.g. "quarantine.json" for legacy mode,
// or ".quarantine/backend/state.json" for suite mode).
func loadQuarantineState(ctx context.Context, cmd *cobra.Command, cfg *config.Config, client *gh.Client, statePath string) (*qstate.State, []byte, string) {
	if client == nil {
		return nil, nil, ""
	}

	branch := cfg.Storage.Branch
	content, sha, err := client.GetContents(ctx, statePath, branch)
	if err != nil {
		emitDegradedWarning(cmd, degradedMsg(err))
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
// Returns the full entries for tests that were removed (for PR comment rendering).
// client may be nil — in that case the check is skipped (state unchanged, empty slice returned).
func removeUnquarantinedTests(ctx context.Context, cmd *cobra.Command, cfg *config.Config, state *qstate.State, client *gh.Client) []qstate.Entry {
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

	var removed []qstate.Entry
	for testID, entry := range state.Tests {
		if entry.IssueNumber != nil && closedSet[*entry.IssueNumber] {
			state.RemoveTest(testID)
			removed = append(removed, entry)
		}
	}
	return removed
}

// addNewFlakyTests adds newly detected flaky tests from the run results to the
// quarantine state. A test is newly flaky if it is not already in the state
// and its result status is "flaky". Tests in skipReasons are skipped
// (new-to-PR tests per ADR-022 — no persistent quarantine without a GitHub Issue).
// Returns true if the state was modified.
func addNewFlakyTests(state *qstate.State, res result.Result, skipReasons map[string]string) bool {
	changed := false
	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range res.Tests {
		if t.Status != "flaky" {
			continue
		}
		if skipReasons[t.TestID] != "" {
			continue
		}
		if state.HasTest(t.TestID) {
			// Already quarantined — update last_flaky_at and flaky count.
			entry := state.Tests[t.TestID]
			entry.LastFailureAt = now
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
			LastFailureAt:   now,
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
// statePath is the file path within the repo (e.g. "quarantine.json" for legacy mode,
// or ".quarantine/backend/state.json" for suite mode).
func writeUpdatedQuarantineState(ctx context.Context, cmd *cobra.Command, cfg *config.Config, state *qstate.State, originalContent []byte, sha string, client *gh.Client, removedTestIDs []string, statePath string) {
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
	reQuarantined, writeErr := cas.WriteStateWithCAS(ctx, client, state, content, sha, branch, 3, removedTestIDs, statePath)
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

// retryFailingTests reruns each failed test individually up to retries times.
// Returns a map of TestID -> RetryOutcome for tests that were retried, a slice
// of warning messages for tests whose rerun command failed to execute, and a
// slice of error messages for tests whose rerun timed out.
// Exits the retry loop early on the first passing attempt or if the rerun
// command itself fails to execute.
// When classifyRerunCrashAsUnresolved is true (suite mode), a rerun command that
// fails to execute marks the outcome as Unresolved rather than as a failed attempt.
// When rerunTimeout > 0, each rerun is executed with that timeout; a timed-out
// rerun is classified as Unresolved and recorded in the returned errors slice.
func retryFailingTests(ctx context.Context, tests []parser.TestResult, retries int, rerunCommand string, rerunTimeout time.Duration, classifyRerunCrashAsUnresolved bool) (map[string]result.RetryOutcome, []string, []string) {
	outcomes := make(map[string]result.RetryOutcome)
	var warnings []string
	var rerunErrors []string

	for _, t := range tests {
		if t.Status != "failed" && t.Status != "error" {
			continue
		}

		rerunCmd, rerunArgs := runner.RerunCommand(
			t.Name, t.Classname, t.FilePath,
			rerunCommand,
		)

		var attempts []result.RetryEntry
		unresolved := false
		for attempt := 1; attempt <= retries; attempt++ {
			var rerunExitCode int
			var timedOut bool
			var runErr error
			if rerunTimeout > 0 {
				rerunExitCode, timedOut, runErr = runner.RunWithTimeout(ctx, rerunTimeout, 0, rerunCmd, rerunArgs, os.Stdout, os.Stderr)
			} else {
				rerunExitCode, runErr = runner.Run(ctx, rerunCmd, rerunArgs, os.Stdout, os.Stderr)
			}
			if timedOut {
				rerunErrors = append(rerunErrors, fmt.Sprintf("Error [rerun]: rerun timed out after %s for '%s'", rerunTimeout, t.Name))
				unresolved = true
				break
			}
			// Treat two conditions as rerun infrastructure failures (not test failures):
			//  1. runErr != nil — the binary failed to start at all (ENOENT)
			//  2. rerunExitCode == 127 — the command ran but "command not found" (shell convention)
			rerunCrash := runErr != nil || (classifyRerunCrashAsUnresolved && rerunExitCode == 127)
			if rerunCrash {
				warnings = append(warnings, rerunFailureWarning(t.Name, rerunCmd))
				if classifyRerunCrashAsUnresolved {
					unresolved = true
				} else {
					attempts = append(attempts, result.RetryEntry{
						Attempt: attempt,
						Status:  "failed",
					})
				}
				break
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

		outcomes[t.TestID] = result.RetryOutcome{Attempts: attempts, Unresolved: unresolved}
	}

	return outcomes, warnings, rerunErrors
}

// rerunFailureWarning returns a warning message for a rerun command that
// failed to execute.
// This is a pure function — no I/O.
func rerunFailureWarning(testName, rerunCmd string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Rerun failed for %q (%s exited with error).\n", testName, rerunCmd)
	sb.WriteString("  Check that the rerun_command in your suite config is correct.\n")
	return sb.String()
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

// xmlFileExists reports whether a file exists at path (non-glob, exact path).
// Returns false for any stat error. This is an I/O helper used by crash detection.
func xmlFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resolveExitCode determines the process exit code for suite mode based on
// result priorities: genuine failure (1) > unresolved (2) > all pass (0).
// This is a pure function — no I/O.
func resolveExitCode(res result.Result) int {
	if res.Summary.Failed > 0 {
		return 1
	}
	if res.Summary.Unresolved > 0 {
		return 2
	}
	return 0
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
	// Ensure a non-nil empty slice is returned when at least one file
	// was successfully parsed (even if it contained 0 test cases), so
	// callers can distinguish "file found, 0 tests" from "no file found".
	if allResults == nil && succeeded > 0 {
		allResults = []parser.TestResult{}
	}
	return allResults, warnings
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
		SuiteName:  framework,
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

// degradedMsg formats a degraded-mode warning message based on the API error type.
// This is a pure function — no I/O.
func degradedMsg(err error) string {
	var apiErr *gh.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return "GitHub API returned 401 (unauthorized). Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."
		case http.StatusForbidden:
			if apiErr.RequestID != "" {
				return fmt.Sprintf("GitHub API returned 403 (forbidden). Request ID: %s. Ensure your token has 'repo' scope.", apiErr.RequestID)
			}
			return "GitHub API returned 403 (forbidden). Ensure your token has 'repo' scope."
		case http.StatusTooManyRequests:
			if apiErr.RetryAfter > 30 {
				return fmt.Sprintf("GitHub API rate limited (%ds wait exceeds 30s threshold). Running in degraded mode.", apiErr.RetryAfter)
			}
			return "GitHub API rate limited. Running in degraded mode."
		default:
			if apiErr.StatusCode >= 500 {
				return fmt.Sprintf("GitHub API server error (%d). Running in degraded mode.", apiErr.StatusCode)
			}
		}
	}
	if isTimeoutError(err) {
		return "GitHub API request timed out. Running in degraded mode."
	}
	return fmt.Sprintf("Unable to reach GitHub API and no cached quarantine state available. Running without quarantine exclusions. (reason: %v)", err)
}

// isTimeoutError returns true if the error is a network timeout.
// This is a pure function — no I/O.
func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// selectSuite resolves which suite to run based on the suites list and the
// optional name argument from the CLI. It handles four cases:
//
//  1. Zero suites configured → error with guidance.
//  2. One suite, no nameArg → auto-select (Scenario 118).
//  3. Multiple suites, no nameArg → error listing available suites (Scenario 119).
//  4. nameArg provided → delegate to findSuite.
//
// This is a pure function — no I/O.
func selectSuite(suites []config.TestSuite, nameArg string) (config.TestSuite, error) {
	if nameArg != "" {
		return findSuite(suites, nameArg)
	}
	switch len(suites) {
	case 0:
		return config.TestSuite{}, errors.New("Error [config]: No test suites configured. Edit .quarantine/config.yml to add one")
	case 1:
		return suites[0], nil
	default:
		var sb strings.Builder
		sb.WriteString("Error [config]: Multiple test suites configured. Specify a suite name:\n")
		for _, s := range suites {
			fmt.Fprintf(&sb, "\n  quarantine run %s", s.Name)
		}
		sb.WriteString("\n\nRun `quarantine suite list` to see all configured suites.")
		return config.TestSuite{}, fmt.Errorf("%s", sb.String())
	}
}

// findSuite returns the suite with the given name from the suites slice.
// Returns an error if no suite with that name is found.
// This is a pure function — no I/O.
func findSuite(suites []config.TestSuite, name string) (config.TestSuite, error) {
	for _, s := range suites {
		if s.Name == name {
			return s, nil
		}
	}
	return config.TestSuite{}, fmt.Errorf("suite %q not found; available suites: %s",
		name, suitesString(suites))
}

// suitesString returns a comma-separated list of suite names for error messages.
// This is a pure function — no I/O.
func suitesString(suites []config.TestSuite) string {
	if len(suites) == 0 {
		return "(none)"
	}
	names := make([]string, len(suites))
	for i, s := range suites {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}
