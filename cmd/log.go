package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
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
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Load config
	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	// Load metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Build stack
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Validate stack
	if err := s.ValidateStack(); err != nil {
		return fmt.Errorf("invalid stack structure: %w", err)
	}

	// Populate PR URLs from remote (best-effort, non-fatal)
	if remoteURL, err := repo.GetRemoteURL("origin"); err == nil {
		if host, owner, repoName, err := provider.ParseRemoteURL(remoteURL); err == nil {
			s.SetPRURLs(fmt.Sprintf("https://%s/%s/%s", host, owner, repoName))
		}
	}

	// Render based on flags
	var output string
	if logShort {
		output = s.RenderShort(repo)
	} else {
		opts := stack.TreeOptions{
			ShowCommitSHA: true,
			ShowCommitMsg: logLong,
			Detailed:      logLong,
		}
		output = s.RenderTree(repo, opts)
	}

	fmt.Print(output)

	return nil
}
