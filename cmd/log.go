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
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	if err := rs.Stack.ValidateStack(); err != nil {
		return fmt.Errorf("invalid stack structure: %w", err)
	}

	// Discover PRs for tracked branches that don't have PR metadata yet.
	// Best-effort: if the provider can't be detected or gh isn't available, skip silently.
	prov, _ := detectProviderBestEffort(rs.Repo)
	if prov != nil {
		discoverPRs(rs.Repo, rs.Metadata, rs.Stack, prov)
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

// discoverPRs queries the hosting provider for open PRs on tracked branches
// that don't already have PR metadata. Discovered PRs are persisted so
// subsequent runs don't re-query. This is best-effort: any failure is
// silently ignored.
func discoverPRs(repo *git.Repo, metadata *config.Metadata, s *stack.Stack, prov provider.Provider) {
	// Collect branches that need discovery (tracked, non-trunk, no PR metadata).
	var missing []string
	for branchName := range metadata.Branches {
		if branchName == s.TrunkName {
			continue
		}
		if metadata.GetPR(branchName) != nil {
			continue
		}
		// Only query branches that actually exist in git (they have nodes in the stack).
		if s.GetNode(branchName) == nil {
			continue
		}
		missing = append(missing, branchName)
	}
	if len(missing) == 0 {
		return
	}

	// Query each missing branch. Track whether we discovered any.
	discovered := false
	for _, branch := range missing {
		result, err := prov.FindExistingPR(branch)
		if err != nil || result == nil {
			continue
		}
		_ = metadata.SetPR(branch, &config.PRInfo{
			Number:   result.Number,
			Provider: prov.Name(),
		})
		discovered = true
	}

	// Persist if we found anything new.
	if discovered {
		_ = metadata.SaveWithRefs(repo, repo.GetMetadataPath())
	}
}

// detectProviderBestEffort returns a Provider if the remote is recognized
// and the CLI is available and authenticated. Returns nil on any failure.
func detectProviderBestEffort(repo *git.Repo) (provider.Provider, error) {
	remoteURL, err := repo.GetRemoteURL(defaultRemote)
	if err != nil {
		return nil, err
	}
	prov, err := provider.DetectFromRemoteURL(remoteURL)
	if err != nil {
		return nil, err
	}
	if !prov.CLIAvailable() || !prov.CLIAuthenticated() {
		return nil, nil
	}
	return prov, nil
}
