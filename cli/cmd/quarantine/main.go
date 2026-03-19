// Package main is the entry point for the quarantine CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags.
var version = "0.1.0"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(2)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "quarantine",
		Short: "Detect, quarantine, and track flaky tests in CI",
		Long: `Quarantine automatically detects, quarantines, and tracks flaky tests
in CI pipelines. It wraps your test command, retries failures, and manages
quarantine state on GitHub.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Override cobra's default flag error handling to print a clean
	// error message and exit 2 (quarantine error). Exit 1 is reserved
	// exclusively for test failures, per docs/cli-spec.md.
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErrln("Error:", err)
		cmd.PrintErrln(cmd.UsageString())
		os.Exit(2)
		return nil
	})

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize quarantine for a repository",
		Long: `Initialize quarantine for a repository. Creates quarantine.yml,
validates GitHub token/permissions, and creates the quarantine/state branch.`,
		RunE: runInit,
	}
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [flags] -- <test command>",
		Short: "Run tests with quarantine detection and enforcement",
		Long: `Run tests with quarantine detection and enforcement. Wraps your test
command, parses JUnit XML output, retries failures, and manages quarantine
state on GitHub.

The -- separator is required. Everything after -- is treated as the test
command and its arguments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement in M2.
			fmt.Println("[quarantine] run is not yet implemented.")
			return nil
		},
	}

	// Flags will be added in M2-M5 as features are implemented.
	cmd.Flags().String("config", "quarantine.yml", "Path to configuration file")
	cmd.Flags().String("junitxml", "", "Glob pattern for JUnit XML output files")
	cmd.Flags().Int("retries", 0, "Number of retries for failing tests (1-10)")
	cmd.Flags().Bool("verbose", false, "Detailed output")
	cmd.Flags().Bool("quiet", false, "Minimal output")
	cmd.Flags().Bool("strict", false, "Exit 2 on infrastructure errors")
	cmd.Flags().Bool("dry-run", false, "Show what would happen without making changes")
	cmd.Flags().Int("pr", 0, "Override PR number for comments")
	cmd.Flags().StringArray("exclude", nil, "Exclude patterns (repeatable)")
	cmd.Flags().String("output", ".quarantine/results.json", "Path for results JSON output")

	return cmd
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate quarantine.yml configuration",
		Long: `Validate quarantine.yml configuration. Reads and validates all fields
against the schema, prints the resolved configuration, and reports errors
and warnings.`,
		RunE: runDoctor,
	}

	cmd.Flags().String("config", "quarantine.yml", "Path to configuration file")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("quarantine v%s\n", version)
		},
	}
}
