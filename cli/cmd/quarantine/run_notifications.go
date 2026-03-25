package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	gh "github.com/mycargus/quarantine/internal/github"
	qstate "github.com/mycargus/quarantine/internal/quarantine"
	"github.com/mycargus/quarantine/internal/result"
	"github.com/spf13/cobra"
)

// testHash returns the first 8 hex characters of the SHA-256 hash of testID.
// This is a pure function — no I/O, deterministic.
func testHash(testID string) string {
	h := sha256.Sum256([]byte(testID))
	return fmt.Sprintf("%x", h[:4])
}

// detectPRNumber resolves the PR number from the --pr flag or GITHUB_EVENT_PATH.
// Returns 0 with no error when neither source is available.
// This is a pure function in terms of logic; it reads a file when prFlag==0 and
// eventPath is set — so it's an I/O function that wraps pure parsing.
func detectPRNumber(prFlag int, eventPath string) (int, error) {
	if prFlag > 0 {
		return prFlag, nil
	}
	if eventPath == "" {
		return 0, nil
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return 0, nil
	}
	return parsePRNumberFromEvent(data), nil
}

// parsePRNumberFromEvent extracts the PR number from a GitHub event JSON payload.
// Handles both pull_request events (.pull_request.number) and
// pull_request_target events (.number).
// This is a pure function — no I/O.
func parsePRNumberFromEvent(data []byte) int {
	var evt struct {
		PullRequest *struct {
			Number int `json:"number"`
		} `json:"pull_request"`
		Number int `json:"number"`
	}
	if err := json.Unmarshal(data, &evt); err != nil {
		return 0
	}
	if evt.PullRequest != nil && evt.PullRequest.Number > 0 {
		return evt.PullRequest.Number
	}
	return evt.Number
}

// IssueBodyData holds the data needed to render a GitHub issue body.
type IssueBodyData struct {
	TestID    string
	Suite     string
	Name      string
	Timestamp string
	Branch    string
	CommitSHA string
	PRNumber  int
}

// renderIssueBody renders the body of a GitHub issue for a flaky test.
// This is a pure function — no I/O.
func renderIssueBody(data IssueBodyData) string {
	var sb strings.Builder
	sb.WriteString("## Flaky Test Detected\n\n")
	fmt.Fprintf(&sb, "**Test ID:** `%s`\n", data.TestID)
	fmt.Fprintf(&sb, "**Suite:** `%s`\n", data.Suite)
	fmt.Fprintf(&sb, "**Name:** `%s`\n", data.Name)
	fmt.Fprintf(&sb, "**First detected:** %s\n", data.Timestamp)
	fmt.Fprintf(&sb, "**Detected in:** %s @ %s\n", data.Branch, data.CommitSHA)
	if data.PRNumber > 0 {
		fmt.Fprintf(&sb, "**PR:** #%d\n", data.PRNumber)
	}
	sb.WriteString("\n---\n\n")
	sb.WriteString("*This issue was automatically created by [Quarantine](https://github.com/mycargus/quarantine). Close this issue to unquarantine the test.*")
	return sb.String()
}

// FlakyEntry holds data for a single flaky test in the PR comment.
type FlakyEntry struct {
	Name     string
	IssueURL string
	IssueNum int
}

// QuarantinedEntry holds data for a single already-quarantined (excluded) test in the PR comment.
type QuarantinedEntry struct {
	Name     string
	IssueURL string
	IssueNum int
	Since    string
}

// UnquarantinedEntry holds data for a single unquarantined test in the PR comment.
type UnquarantinedEntry struct {
	Name     string
	IssueURL string
	IssueNum int
}

// FailureEntry holds data for a single genuine failure in the PR comment.
type FailureEntry struct {
	Name    string
	Message string
}

// PRCommentData holds the data needed to render the PR comment.
type PRCommentData struct {
	Total             int
	Passed            int
	Failed            int
	Flaky             int
	Quarantined       int
	Unquarantined     int
	Version           string
	NewlyFlaky        []FlakyEntry
	QuarantinedTests  []QuarantinedEntry
	UnquarantinedTests []UnquarantinedEntry
	Failures          []FailureEntry
}

// renderPRComment renders the PR comment body from the given data.
// The first line MUST be <!-- quarantine-bot --> per spec.
// This is a pure function — no I/O.
func renderPRComment(data PRCommentData) string {
	var sb strings.Builder

	sb.WriteString("<!-- quarantine-bot -->\n")
	sb.WriteString("## Quarantine Summary\n\n")

	sb.WriteString("| Metric | Count |\n")
	sb.WriteString("|--------|-------|\n")
	fmt.Fprintf(&sb, "| Tests run | %d |\n", data.Total)
	fmt.Fprintf(&sb, "| Passed | %d |\n", data.Passed)
	fmt.Fprintf(&sb, "| Failed (genuine) | %d |\n", data.Failed)
	fmt.Fprintf(&sb, "| Flaky (newly detected) | %d |\n", data.Flaky)
	fmt.Fprintf(&sb, "| Quarantined (excluded) | %d |\n", data.Quarantined)
	fmt.Fprintf(&sb, "| Unquarantined | %d |\n", data.Unquarantined)

	if len(data.NewlyFlaky) > 0 {
		sb.WriteString("\n### New Flaky Tests Detected\n\n")
		sb.WriteString("| Test | Issue |\n")
		sb.WriteString("|------|-------|\n")
		for _, f := range data.NewlyFlaky {
			fmt.Fprintf(&sb, "| `%s` | [#%d](%s) |\n", f.Name, f.IssueNum, f.IssueURL)
		}
		sb.WriteString("\nThese tests failed initially but passed on retry. They have been quarantined\n")
		sb.WriteString("and will be excluded from future runs until their issues are resolved.\n")
	}

	if len(data.QuarantinedTests) > 0 {
		fmt.Fprintf(&sb, "\n### Quarantined Tests (Excluded)\n\n%d quarantined tests were excluded from this run:\n\n", len(data.QuarantinedTests))
		sb.WriteString("<details>\n<summary>View excluded tests</summary>\n\n")
		sb.WriteString("| Test | Issue | Quarantined since |\n")
		sb.WriteString("|------|-------|-------------------|\n")
		for _, q := range data.QuarantinedTests {
			fmt.Fprintf(&sb, "| `%s` | [#%d](%s) | %s |\n", q.Name, q.IssueNum, q.IssueURL, q.Since)
		}
		sb.WriteString("\n</details>\n")
	}

	if len(data.UnquarantinedTests) > 0 {
		sb.WriteString("\n### Unquarantined Tests\n\nThe following tests have been unquarantined because their tracking issues were closed:\n\n")
		sb.WriteString("| Test | Issue |\n")
		sb.WriteString("|------|-------|\n")
		for _, u := range data.UnquarantinedTests {
			fmt.Fprintf(&sb, "| `%s` | [#%d](%s) (closed) |\n", u.Name, u.IssueNum, u.IssueURL)
		}
		sb.WriteString("\nThese tests are now running normally again. If they fail, they will be treated\nas genuine failures.\n")
	}

	if len(data.Failures) > 0 {
		sb.WriteString("\n### Real Failures\n\nThe following tests failed and are NOT flaky (failed all retries):\n\n")
		sb.WriteString("| Test | Failure message |\n")
		sb.WriteString("|------|------------------|\n")
		for _, f := range data.Failures {
			fmt.Fprintf(&sb, "| `%s` | `%s` |\n", f.Name, f.Message)
		}
	}

	sb.WriteString("\n---\n")
	fmt.Fprintf(&sb, "<sub>Posted by [quarantine](https://github.com/mycargus/quarantine) v%s</sub>", data.Version)

	return sb.String()
}

// postOrUpdatePRComment posts or updates the quarantine-bot PR comment.
// This is an I/O function — best-effort, never breaks the build.
func postOrUpdatePRComment(ctx context.Context, cmd *cobra.Command, client *gh.Client, prNumber int, body string) {
	if client == nil || prNumber == 0 {
		return
	}

	// List existing comments to find the quarantine-bot comment.
	comments, err := client.ListPRComments(ctx, prNumber)
	if err != nil {
		cmd.PrintErrf("[quarantine] WARNING: Could not list PR comments: %v\n", err)
		// Fall through to create a new comment.
	}

	const marker = "<!-- quarantine-bot -->"
	for _, c := range comments {
		if strings.HasPrefix(c.Body, marker) {
			// Found existing comment — update it.
			if updateErr := client.UpdatePRComment(ctx, c.ID, body); updateErr != nil {
				cmd.PrintErrf("[quarantine] WARNING: Could not update PR comment: %v\n", updateErr)
			}
			return
		}
	}

	// No existing comment — create a new one.
	if createErr := client.CreatePRComment(ctx, prNumber, body); createErr != nil {
		cmd.PrintErrf("[quarantine] WARNING: Could not post PR comment: %v\n", createErr)
	}
}

// createIssuesForNewFlakyTests creates GitHub issues for newly detected flaky
// tests, using dedup search to avoid duplicates.
// Returns a map of testID -> (issueNumber, issueURL) for tests where an issue
// was found or created.
// This is an I/O orchestrator — best-effort, never breaks the build.
func createIssuesForNewFlakyTests(
	ctx context.Context,
	cmd *cobra.Command,
	client *gh.Client,
	res result.Result,
	excludePatterns []string,
	branch, commitSHA string,
	prNumber int,
) map[string]issueRef {
	refs := make(map[string]issueRef)
	if client == nil {
		return refs
	}

	for _, t := range res.Tests {
		if t.Status != "flaky" {
			continue
		}

		hash := testHash(t.TestID)

		// Dedup: search for open issue first.
		existingNum, existingURL, found, searchErr := client.SearchOpenIssue(ctx, hash)
		if searchErr != nil {
			cmd.PrintErrf("[quarantine] WARNING: dedup search failed for %q: %v. Proceeding to create issue.\n", t.Name, searchErr)
		}
		if found {
			refs[t.TestID] = issueRef{Number: existingNum, URL: existingURL}
			continue
		}

		// Create a new issue.
		title := fmt.Sprintf("[Quarantine] %s", t.Name)
		body := renderIssueBody(IssueBodyData{
			TestID:    t.TestID,
			Suite:     t.FilePath,
			Name:      t.Name,
			Timestamp: res.Timestamp,
			Branch:    branch,
			CommitSHA: commitSHA,
			PRNumber:  prNumber,
		})
		labels := []string{"quarantine", fmt.Sprintf("quarantine:%s", hash)}

		issueNum, issueURL, createErr := client.CreateIssue(ctx, title, body, labels)
		if createErr != nil {
			cmd.PrintErrf("[quarantine] WARNING: Could not create issue for %q: %v\n", t.Name, createErr)
			continue
		}
		refs[t.TestID] = issueRef{Number: issueNum, URL: issueURL}
	}

	return refs
}

// issueRef holds the number and URL for a GitHub Issue.
type issueRef struct {
	Number int
	URL    string
}

// buildFlakyEntries converts result tests + issue refs to PR comment flaky entries.
// This is a pure function — no I/O.
func buildFlakyEntries(res result.Result, issueRefs map[string]issueRef) []FlakyEntry {
	var entries []FlakyEntry
	for _, t := range res.Tests {
		if t.Status != "flaky" {
			continue
		}
		ref := issueRefs[t.TestID]
		entries = append(entries, FlakyEntry{
			Name:     t.Name,
			IssueURL: ref.URL,
			IssueNum: ref.Number,
		})
	}
	return entries
}

// buildQuarantinedEntries converts quarantine state entries to PR comment quarantined entries.
// This is a pure function — no I/O.
func buildQuarantinedEntries(state *qstate.State) []QuarantinedEntry {
	if state == nil {
		return nil
	}
	var entries []QuarantinedEntry
	for _, e := range state.Tests {
		num := 0
		if e.IssueNumber != nil {
			num = *e.IssueNumber
		}
		entries = append(entries, QuarantinedEntry{
			Name:     e.Name,
			IssueURL: e.IssueURL,
			IssueNum: num,
			Since:    e.QuarantinedAt,
		})
	}
	return entries
}

// buildUnquarantinedEntries converts removed quarantine entries to PR comment unquarantined entries.
// This is a pure function — no I/O.
func buildUnquarantinedEntries(removed []qstate.Entry) []UnquarantinedEntry {
	var entries []UnquarantinedEntry
	for _, e := range removed {
		num := 0
		if e.IssueNumber != nil {
			num = *e.IssueNumber
		}
		entries = append(entries, UnquarantinedEntry{
			Name:     e.Name,
			IssueURL: e.IssueURL,
			IssueNum: num,
		})
	}
	return entries
}

// buildFailureEntries converts result tests to PR comment failure entries.
// This is a pure function — no I/O.
func buildFailureEntries(res result.Result) []FailureEntry {
	var entries []FailureEntry
	for _, t := range res.Tests {
		if t.Status != "failed" {
			continue
		}
		msg := ""
		if t.FailureMessage != nil {
			msg = *t.FailureMessage
		}
		entries = append(entries, FailureEntry{
			Name:    t.Name,
			Message: msg,
		})
	}
	return entries
}
