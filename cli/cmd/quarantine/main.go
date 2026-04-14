// Package main is the entry point for the quarantine CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags (-X main.version=...).
// Local dev builds default to "dev"; GoReleaser sets the real version.
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Exit code 1 for test failures, 2 for quarantine errors.
		if code, ok := err.(exitCodeError); ok {
			os.Exit(int(code))
		}
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

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newSuiteCmd())
	rootCmd.AddCommand(newStatusCmd())

	return rootCmd
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize quarantine for a repository",
		Long: `Initialize quarantine for a repository. Creates quarantine.yml,
validates GitHub token/permissions, and creates the quarantine/state branch.`,
		RunE: runInit,
	}
	return cmd
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [suite-name]",
		Short: "Run tests with quarantine detection and enforcement",
		Long: `Run tests with quarantine detection and enforcement. Reads the suite
configuration from .quarantine/config.yml, executes the named suite command,
parses JUnit XML output, retries failures, and manages quarantine state on GitHub.`,
		RunE: runRun,
	}

	cmd.Flags().Bool("verbose", false, "Detailed output")
	cmd.Flags().Bool("quiet", false, "Minimal output")
	cmd.Flags().Bool("strict", false, "Exit 2 on infrastructure errors")
	cmd.Flags().Bool("dry-run", false, "Show what would happen without making changes")
	cmd.Flags().Int("pr", 0, "Override PR number for comments")

	return cmd
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate .quarantine/config.yml configuration",
		Long: `Validate .quarantine/config.yml configuration. Reads and validates all
fields against the schema, prints the resolved configuration, and reports errors
and warnings.`,
		RunE: runDoctor,
	}

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
