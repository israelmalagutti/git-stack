package cmd

import (
	"fmt"
	"os"

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
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Override default version template to show more info
	rootCmd.SetVersionTemplate(`gs version {{.Version}}
`)
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() string {
	return fmt.Sprintf("gs version %s\ncommit: %s\nbuilt: %s", Version, Commit, BuildDate)
}
