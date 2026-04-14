package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/spf13/cobra"
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

// newSuiteCmd returns the "suite" parent cobra command.
func newSuiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suite",
		Short: "Manage test suites",
	}
	cmd.AddCommand(newSuiteListCmd())
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
