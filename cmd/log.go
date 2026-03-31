package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	logShort bool
	logLong  bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display a visual representation of the current stack",
	Long: `Display a visual representation of the stack structure.

Shows branches as a tree starting from the trunk branch, with
the current branch highlighted.

Modes:
  gs log         - Standard tree view (*branch = current)
  gs log --short - Compact indented view (● = current, ○ = other)
  gs log --long  - Detailed view with commit messages`,
	RunE: runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVar(&logShort, "short", false, "Show compact view")
	logCmd.Flags().BoolVar(&logLong, "long", false, "Show detailed view with commit messages")
}

func runLog(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	if err := rs.Stack.ValidateStack(); err != nil {
		return fmt.Errorf("invalid stack structure: %w", err)
	}

	// Populate PR URLs from remote (best-effort, non-fatal)
	if remoteURL, err := rs.Repo.GetRemoteURL("origin"); err == nil {
		if host, owner, repoName, err := provider.ParseRemoteURL(remoteURL); err == nil {
			rs.Stack.SetPRURLs(fmt.Sprintf("https://%s/%s/%s", host, owner, repoName))
		}
	}

	var output string
	if logShort {
		output = rs.Stack.RenderShort(rs.Repo)
	} else {
		opts := stack.TreeOptions{
			ShowCommitSHA: true,
			ShowCommitMsg: logLong,
			Detailed:      logLong,
		}
		output = rs.Stack.RenderTree(rs.Repo, opts)
	}

	fmt.Print(output)

	return nil
}
