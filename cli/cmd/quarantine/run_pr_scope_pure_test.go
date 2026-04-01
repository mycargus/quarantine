package main

import (
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Unit tests for defaultCheckPRScopeForTests ---
// These tests call defaultCheckPRScopeForTests directly (without injection) to
// exercise the git command logic. They use a two-repo setup so origin/main
// stays at the base commit while the working tree has PR-specific changes.

// setupTwoRepoGit creates an origin repo with an initial commit, then clones
// it into a working dir. Returns working dir path. The working dir's origin/main
// tracks the origin's initial commit, so new commits in working dir are "ahead
// of origin/main" — exactly the condition a PR run would see.
func setupTwoRepoGit(t *testing.T) (workDir string) {
	t.Helper()

	// 1. Init origin repo with a base commit.
	originDir := t.TempDir()
	runCmd(t, originDir, "git", "init", "-b", "main")
	runCmd(t, originDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, originDir, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(originDir, "existing.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write existing.go: %v", err)
	}
	runCmd(t, originDir, "git", "add", ".")
	runCmd(t, originDir, "git", "commit", "-m", "initial")

	// 2. Clone origin into working dir.
	workDir = t.TempDir()
	runCmd(t, workDir, "git", "clone", originDir, ".")
	runCmd(t, workDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, workDir, "git", "config", "user.name", "Test")
	return workDir
}

// cdTo changes the process working directory and registers a cleanup to restore it.
func cdTo(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

// TestDefaultCheckPRScopeEmptyBaseRef verifies that an empty baseRef triggers
// the early return (line 46 first condition).
func TestDefaultCheckPRScopeEmptyBaseRef(t *testing.T) {
	inputs := []prScopeInput{{TestID: "t1", FilePath: "foo.go", Name: "test"}}
	reasons := defaultCheckPRScopeForTests("", inputs)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "baseRef is empty",
		Should:   "return empty map immediately (no git commands run)",
		Actual:   len(reasons),
		Expected: 0,
	})
}

// TestDefaultCheckPRScopeEmptyInputs verifies that empty inputs triggers the
// early return (line 46 second condition).
func TestDefaultCheckPRScopeEmptyInputs(t *testing.T) {
	reasons := defaultCheckPRScopeForTests("main", []prScopeInput{})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "inputs list is empty",
		Should:   "return empty map immediately (no git commands run)",
		Actual:   len(reasons),
		Expected: 0,
	})
}

// TestDefaultCheckPRScopeNewFileDetected verifies the full git-command path
// when a test file is new to the PR.
// Kills mutations on lines 57 (err == nil), 59 (line != ""), 70 (f == filePath),
// and line 46 second condition.
func TestDefaultCheckPRScopeNewFileDetected(t *testing.T) {
	workDir := setupTwoRepoGit(t)

	// Add a new file that did not exist in origin/main.
	newFile := filepath.Join(workDir, "newtest.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write newtest.go: %v", err)
	}
	runCmd(t, workDir, "git", "add", ".")
	runCmd(t, workDir, "git", "commit", "-m", "add new test file")

	cdTo(t, workDir)

	inputs := []prScopeInput{{
		TestID:   "newtest.go::Foo::bar",
		FilePath: "newtest.go",
		Name:     "bar",
	}}
	reasons := defaultCheckPRScopeForTests("main", inputs)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newtest.go was added to the PR (not in origin/main)",
		Should:   "classify as 'new_file_in_pr'",
		Actual:   reasons["newtest.go::Foo::bar"],
		Expected: "new_file_in_pr",
	})
}

// TestDefaultCheckPRScopeNewTestInExistingFile verifies that when a new test
// is added to a pre-existing file, the per-file diff is run and the test is
// classified as "new_test_in_pr".
// Kills mutations on lines 75 (!isNewFile guard), 75 (inp.FilePath != ""),
// and 77 (diffErr == nil).
func TestDefaultCheckPRScopeNewTestInExistingFile(t *testing.T) {
	workDir := setupTwoRepoGit(t)

	// Modify the pre-existing file to add a line mentioning the test name.
	existingFile := filepath.Join(workDir, "existing.go")
	if err := os.WriteFile(existingFile, []byte("package main\n// TestMyNewScenario\n"), 0644); err != nil {
		t.Fatalf("write existing.go: %v", err)
	}
	runCmd(t, workDir, "git", "add", ".")
	runCmd(t, workDir, "git", "commit", "-m", "add new test in existing file")

	cdTo(t, workDir)

	inputs := []prScopeInput{{
		TestID:   "existing.go::Suite::TestMyNewScenario",
		FilePath: "existing.go",
		Name:     "TestMyNewScenario",
	}}
	reasons := defaultCheckPRScopeForTests("main", inputs)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "TestMyNewScenario was added in a diff line of existing.go",
		Should:   "classify as 'new_test_in_pr'",
		Actual:   reasons["existing.go::Suite::TestMyNewScenario"],
		Expected: "new_test_in_pr",
	})
}
