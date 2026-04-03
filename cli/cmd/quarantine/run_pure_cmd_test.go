package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/mycargus/quarantine/cli/internal/parser"
	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)

// --- Command construction: flag defaults and error messages ---

func TestNewRunCmdFlagDefaults(t *testing.T) {
	cmd := newRunCmd()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --config flag to quarantine.yml",
		Actual:   cmd.Flags().Lookup("config").DefValue,
		Expected: "quarantine.yml",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --retries flag to 0",
		Actual:   cmd.Flags().Lookup("retries").DefValue,
		Expected: "0",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newRunCmd with no arguments",
		Should:   "default --output flag to .quarantine/results.json",
		Actual:   cmd.Flags().Lookup("output").DefValue,
		Expected: ".quarantine/results.json",
	})
}

func TestNewDoctorCmdFlagDefaults(t *testing.T) {
	cmd := newDoctorCmd()

	riteway.Assert(t, riteway.Case[string]{
		Given:    "newDoctorCmd with no arguments",
		Should:   "default --config flag to quarantine.yml",
		Actual:   cmd.Flags().Lookup("config").DefValue,
		Expected: "quarantine.yml",
	})
}

func TestNewRunCmdMissingSeparatorError(t *testing.T) {
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"run", "--unknown-flag"})

	err := rootCmd.Execute()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with an unknown flag and no -- separator",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "run called with an unknown flag and no -- separator",
		Should:   "error message contains missing separator",
		Actual:   strings.Contains(err.Error(), "missing separator"),
		Expected: true,
	})
}

func TestNewVersionCmdOutput(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	// version is a package-level var; capture stdout via os.Pipe since
	// fmt.Printf bypasses cmd.OutOrStdout().
	// Instead, redirect stdout at the process level for this call.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine version command",
		Should:   "output quarantine v prefix",
		Actual:   strings.HasPrefix(out.String(), "quarantine v"),
		Expected: true,
	})
}

// --- Mutation guard: line 653 len(matches) == 0 ---

// TestParseJUnitXMLNoMatchesReturnsNil kills the mutation len(matches) > 0 on
// line 653. When the glob matches no files, parseJUnitXML must return nil
// results (not an empty non-nil slice). The caller uses testResults == nil to
// decide whether to emit the "no JUnit XML found" warning.
func TestParseJUnitXMLNoMatchesReturnsNil(t *testing.T) {
	dir := t.TempDir()
	// A glob that matches nothing.
	pattern := filepath.Join(dir, "*.xml")

	results, warnings := parseJUnitXML(pattern)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "glob pattern that matches no files",
		Should:   "return nil results (not empty slice)",
		Actual:   results == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "glob pattern that matches no files",
		Should:   "return no warnings",
		Actual:   len(warnings),
		Expected: 0,
	})
}

// TestParseJUnitXMLWithMatchReturnsNonNil is the counterpart: a valid XML file
// must produce non-nil results so the caller does NOT falsely emit the warning.
func TestParseJUnitXMLWithMatchReturnsNonNil(t *testing.T) {
	dir := t.TempDir()
	xmlContent := `<?xml version="1.0"?>
<testsuites tests="1">
  <testsuite name="s.test.js" tests="1">
    <testcase classname="S" name="passes" file="s.test.js" time="0.01"/>
  </testsuite>
</testsuites>`
	xmlPath := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	results, _ := parseJUnitXML(xmlPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "glob pattern matching one valid XML file",
		Should:   "return non-nil results",
		Actual:   results != nil,
		Expected: true,
	})
}

// --- Mutation guard: line 694 succeeded < total ---

// TestMergeParseResultsSummaryWarningIsEmitted kills the mutation
// succeeded > total on line 694. When 1 of 2 files fails, succeeded=1,
// total=2. With the mutation: 1 > 2 = false, so no summary warning would be
// appended. This test checks that the summary warning IS present.
func TestMergeParseResultsSummaryWarningIsEmitted(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}

	_, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{warning: "Failed to parse shard-2.xml: unexpected EOF. Skipping."},
	})

	var hasSummaryWarning bool
	for _, w := range warnings {
		if strings.Contains(w, "Parsed 1/2") {
			hasSummaryWarning = true
		}
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "1 of 2 parse attempts failed (succeeded=1 < total=2)",
		Should:   "include a 'Parsed 1/2' summary warning",
		Actual:   hasSummaryWarning,
		Expected: true,
	})
}

// TestMergeParseResultsAllSucceededNoSummaryWarning is the counterpart:
// when all files parse successfully, no summary warning should appear.
func TestMergeParseResultsAllSucceededNoSummaryWarning(t *testing.T) {
	r1 := []parser.TestResult{{TestID: "a", Status: "passed"}}
	r2 := []parser.TestResult{{TestID: "b", Status: "passed"}}

	_, warnings := mergeParseResults([]parseAttempt{
		{results: r1},
		{results: r2},
	})

	var hasSummaryWarning bool
	for _, w := range warnings {
		if strings.Contains(w, "Parsed") && strings.Contains(w, "JUnit XML") {
			hasSummaryWarning = true
		}
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all 2 parse attempts succeeded (succeeded=2, not < total=2)",
		Should:   "not include a summary warning",
		Actual:   hasSummaryWarning,
		Expected: false,
	})
}

// --- Mutation guard: line 837 res.Summary.Total == 0 ---

// TestAllTestsQuarantinedTotalZeroReturnsTrue kills the mutation
// res.Summary.Total != 0 on line 837. When Jest excludes all tests via
// exclusion flags, Total==0. allTestsQuarantined must return true so the
// "All tests are quarantined" warning is emitted.
func TestAllTestsQuarantinedTotalZeroReturnsTrue(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 0},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=0 (Jest excluded all tests)",
		Should:   "return true (all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: true,
	})
}

// TestAllTestsQuarantinedTotalNonZeroAllQuarantined tests the RSpec path:
// Total > 0 and Quarantined == Total.
func TestAllTestsQuarantinedTotalNonZeroAllQuarantined(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 2, Quarantined: 2},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=2 and Quarantined=2 (RSpec post-filtering)",
		Should:   "return true (all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: true,
	})
}

// TestAllTestsQuarantinedPartialReturnsFalse ensures a partial quarantine
// (some tests still ran) does not trigger the warning.
func TestAllTestsQuarantinedPartialReturnsFalse(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 3, Quarantined: 1, Passed: 2},
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "result with Total=3 but only 1 quarantined",
		Should:   "return false (not all tests quarantined)",
		Actual:   allTestsQuarantined(res),
		Expected: false,
	})
}

// --- Mutation guard: line 710 runID == "" ---

// TestBuildMetadataUsesEnvRunID kills the mutation runID != "" on line 710.
// When GITHUB_RUN_ID is set, buildMetadata must use it as-is (not generate a
// local fallback). With the mutation the condition is flipped: a non-empty env
// value would be overwritten by the local-... fallback.
func TestBuildMetadataUsesEnvRunID(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "12345678")
	// Unset branch / SHA env vars so they fall back to git (or empty) — we
	// only care about RunID here.
	t.Setenv("GITHUB_REF_NAME", "")
	t.Setenv("GITHUB_SHA", "")

	cfg := &config.Config{Framework: "jest", Retries: 3}
	meta := buildMetadata(cfg)

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GITHUB_RUN_ID=12345678 is set in the environment",
		Should:   "use the env value as RunID without generating a local fallback",
		Actual:   meta.RunID,
		Expected: "12345678",
	})
}

// TestBuildMetadataGeneratesFallbackWhenRunIDNotSet kills the same mutation
// from the other direction: when GITHUB_RUN_ID is empty, a local-... fallback
// must be generated. With the mutation the condition is `runID != ""`, so an
// empty env value would leave RunID as "" instead of generating the fallback.
func TestBuildMetadataGeneratesFallbackWhenRunIDNotSet(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "")

	cfg := &config.Config{Framework: "jest", Retries: 3}
	meta := buildMetadata(cfg)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GITHUB_RUN_ID is not set",
		Should:   "generate a local-... fallback run ID",
		Actual:   strings.HasPrefix(meta.RunID, "local-"),
		Expected: true,
	})
}
