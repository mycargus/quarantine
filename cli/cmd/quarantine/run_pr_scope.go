package main

import (
	"os/exec"
	"strings"

	"github.com/mycargus/quarantine/internal/result"
)

// prScopeInput holds the fields needed to classify a single flaky test by PR scope.
type prScopeInput struct {
	TestID   string
	FilePath string
	Name     string
}

// checkPRScopeForTests classifies flaky tests by their relationship to the current PR.
// Returns a map of testID -> skipReason ("new_file_in_pr" or "new_test_in_pr").
// Tests not in the map are pre-existing and should follow the normal issue-creation path.
// Injectable for testing — default implementation runs actual git commands.
var checkPRScopeForTests = defaultCheckPRScopeForTests

// classifyPRScope determines whether a test is new to the current PR.
// newFiles is the list of file paths added in this PR (from git diff --diff-filter=A).
// diffLines is the added lines (+) from git diff for the test's file.
// Returns "" (pre-existing), "new_file_in_pr", or "new_test_in_pr".
// This is a pure function — no I/O.
func classifyPRScope(newFiles []string, filePath, testName string, diffLines []string) string {
	for _, f := range newFiles {
		if f == filePath {
			return "new_file_in_pr"
		}
	}
	for _, line := range diffLines {
		if strings.HasPrefix(line, "+") && strings.Contains(line, testName) {
			return "new_test_in_pr"
		}
	}
	return ""
}

// defaultCheckPRScopeForTests runs git commands to determine which flaky tests are new
// to the current PR. Falls back to empty map (treat all as pre-existing) on any error.
func defaultCheckPRScopeForTests(baseRef string, inputs []prScopeInput) map[string]string {
	reasons := make(map[string]string)
	if baseRef == "" || len(inputs) == 0 {
		return reasons
	}

	// Fetch the base ref so it's available in shallow clones.
	// Failure is non-fatal — fall back to treating all tests as pre-existing.
	_ = exec.Command("git", "fetch", "origin", baseRef, "--depth=1").Run()

	// Get the list of files added in this PR.
	out, err := exec.Command("git", "diff", "--name-only", "--diff-filter=A", "origin/"+baseRef).Output()
	var newFiles []string
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				newFiles = append(newFiles, line)
			}
		}
	}

	for _, inp := range inputs {
		var diffLines []string
		// Only fetch per-file diff if the file is NOT in the new-files list.
		isNewFile := false
		for _, f := range newFiles {
			if f == inp.FilePath {
				isNewFile = true
				break
			}
		}
		if !isNewFile && inp.FilePath != "" {
			diffOut, diffErr := exec.Command("git", "diff", "origin/"+baseRef, "--", inp.FilePath).Output()
			if diffErr == nil {
				diffLines = strings.Split(string(diffOut), "\n")
			}
		}
		reason := classifyPRScope(newFiles, inp.FilePath, inp.Name, diffLines)
		if reason != "" {
			reasons[inp.TestID] = reason
		}
	}

	return reasons
}

// buildPRScopeInputs collects flaky test inputs for the PR scope check.
// This is a pure function — no I/O.
func buildPRScopeInputs(res result.Result) []prScopeInput {
	var inputs []prScopeInput
	for _, t := range res.Tests {
		if t.Status == "flaky" {
			inputs = append(inputs, prScopeInput{
				TestID:   t.TestID,
				FilePath: t.FilePath,
				Name:     t.Name,
			})
		}
	}
	return inputs
}
