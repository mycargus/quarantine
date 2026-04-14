package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// suiteEntry holds the display fields for one test suite row.
// This is a pure data type — no I/O.
type suiteEntry struct {
	Name     string
	Command  string
	JUnitXML string
}

// formatSuiteRows formats suite entries as a tab-aligned table string with a
// header row. This is a pure function — no I/O, deterministic.
func formatSuiteRows(entries []suiteEntry) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SUITE\tCOMMAND\tJUNITXML")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Command, e.JUnitXML)
	}
	w.Flush()
	return buf.String()
}

// suiteNameEntry is a minimal named-entry type used by pure removal helpers.
// It holds just the name field so tests can assert on the result without
// depending on the full config.TestSuite struct.
type suiteNameEntry struct {
	name string
}

// suiteRemoveRamificationMessage returns the multi-line warning string shown
// to the user before they confirm a suite removal. This is a pure function —
// no I/O, deterministic.
func suiteRemoveRamificationMessage(suiteName string) string {
	return fmt.Sprintf(`Removing suite '%s':
  - The '%s' entry will be removed from .quarantine/config.yml
  - The state file (.quarantine/%s/state.json) on the quarantine/state
    branch will NOT be deleted — quarantined tests remain quarantined
  - GitHub issues for this suite's flaky tests will remain open but will no
    longer be updated by quarantine
  - If CI still runs `+"`"+`quarantine run %s`+"`"+`, it will error because the suite
    no longer exists in config — update your CI workflow first before removing

Are you sure? [y/N] `, suiteName, suiteName, suiteName, suiteName)
}

// suiteExists reports whether a suite with the given name is present in the
// config. This is a pure function — no I/O, deterministic.
func suiteExists(suites []config.TestSuite, name string) bool {
	for _, s := range suites {
		if s.Name == name {
			return true
		}
	}
	return false
}

// removeSuiteByName returns a new slice with all entries whose name matches
// the given name removed. This is a pure function — no I/O, deterministic.
func removeSuiteByName(entries []suiteNameEntry, name string) []suiteNameEntry {
	result := make([]suiteNameEntry, 0, len(entries))
	for _, e := range entries {
		if e.name != name {
			result = append(result, e)
		}
	}
	return result
}

// newSuiteCmd returns the "suite" parent cobra command.
func newSuiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suite",
		Short: "Manage test suites",
	}
	cmd.AddCommand(newSuiteListCmd())
	cmd.AddCommand(newSuiteRemoveCmd())
	return cmd
}

// newSuiteListCmd returns the "suite list" cobra subcommand.
func newSuiteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured test suites",
		RunE:  runSuiteList,
	}
}

// runSuiteList is the I/O shell for "suite list". It reads the config and
// writes the formatted suite table to stdout.
func runSuiteList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(".quarantine/config.yml")
	if err != nil {
		return err
	}

	entries := make([]suiteEntry, 0, len(cfg.TestSuites))
	for _, s := range cfg.TestSuites {
		entries = append(entries, suiteEntry{
			Name:     s.Name,
			Command:  strings.Join(s.Commands(), " "),
			JUnitXML: s.JUnitXML,
		})
	}

	cmd.Print(formatSuiteRows(entries))
	return nil
}

// newSuiteRemoveCmd returns the "suite remove" cobra subcommand.
func newSuiteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a test suite from the configuration",
		Args:  cobra.ExactArgs(1),
		RunE:  runSuiteRemove,
	}
}

// runSuiteRemove is the I/O shell for "suite remove". It reads the config,
// prints the ramification message, prompts for confirmation, and if confirmed
// writes the updated config back without the named suite.
func runSuiteRemove(cmd *cobra.Command, args []string) error {
	suiteName := args[0]

	cfg, err := config.Load(".quarantine/config.yml")
	if err != nil {
		return err
	}

	if !suiteExists(cfg.TestSuites, suiteName) {
		return fmt.Errorf("Error [config]: suite '%s' not found in .quarantine/config.yml", suiteName)
	}

	// Print ramifications and prompt.
	cmd.Print(suiteRemoveRamificationMessage(suiteName))

	// Read one line of confirmation from stdin.
	scanner := bufio.NewScanner(cmd.InOrStdin())
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())

	if answer != "y" && answer != "Y" {
		cmd.Println("Aborted. No changes made.")
		return nil
	}

	// Remove the suite from the slice.
	filtered := make([]config.TestSuite, 0, len(cfg.TestSuites))
	for _, s := range cfg.TestSuites {
		if s.Name != suiteName {
			filtered = append(filtered, s)
		}
	}
	cfg.TestSuites = filtered

	// Marshal and write config back.
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(".quarantine/config.yml", out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	cmd.Printf("Suite '%s' removed from .quarantine/config.yml.\n", suiteName)
	return nil
}
