package cmd

import (
	"fmt"
	"os"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/spf13/cobra"
)

// Version information - injected at build time via ldflags
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "gs",
	Short: "gs - blazing fast git stack management",
	Long: `gs (git-stack) is a fast, simple git stack management tool.

It helps you work with stacked diffs (stacked PRs) efficiently,
maintaining parent-child relationships between branches.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmdutil.JSONMode(cmd) {
			colors.SetEnabled(false)
		}
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if cmdutil.JSONMode(rootCmd) {
			cmdName := ""
			if sub, _, _ := rootCmd.Find(os.Args[1:]); sub != nil && sub != rootCmd {
				cmdName = sub.Name()
			}
			cmdutil.PrintJSONError(cmdName, err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output result as JSON to stdout")
	rootCmd.PersistentFlags().Bool("debug", false, "Print timestamped debug lines to stderr")
	rootCmd.PersistentFlags().Bool("no-interactive", false, "Disable interactive prompts")

	rootCmd.SetVersionTemplate(`gs version {{.Version}}
`)
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() string {
	return fmt.Sprintf("gs version %s\ncommit: %s\nbuilt: %s", Version, Commit, BuildDate)
}
