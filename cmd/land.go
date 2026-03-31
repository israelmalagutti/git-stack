package cmd

import (
	"fmt"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/land"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	landStack          bool
	landNoDeleteRemote bool
	landForce          bool
)

var landCmd = &cobra.Command{
	Use:   "land [branch]",
	Short: "Land a merged branch and clean up",
	Long: `Land a branch whose PR has been merged (or whose commits are in trunk).

This command:
- Verifies the branch is merged (PR status or git merge-base)
- Reparents children to the landed branch's parent
- Updates children's PR base branches on the provider
- Deletes the branch locally and remotely
- Cleans up metadata refs
- Restacks children

Examples:
  gs land                      # land current branch
  gs land feat/auth            # land specific branch
  gs land --stack              # land all merged branches
  gs land --no-delete-remote   # keep remote branch`,
	RunE: runLand,
}

func init() {
	landCmd.Flags().BoolVar(&landStack, "stack", false, "Land all merged branches in the stack")
	landCmd.Flags().BoolVar(&landNoDeleteRemote, "no-delete-remote", false, "Don't delete remote branch")
	landCmd.Flags().BoolVarP(&landForce, "force", "f", false, "Skip confirmation prompts")
	rootCmd.AddCommand(landCmd)
}

func runLand(cmd *cobra.Command, args []string) error {
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Detect provider (best-effort — nil is OK for no-PR flow)
	var prov provider.Provider
	if p, err := detectProvider(repo); err == nil {
		if p.CLIAvailable() && p.CLIAuthenticated() {
			prov = p
		}
	}

	if landStack {
		return landStackBranches(repo, metadata, prov, cfg)
	}

	// Determine branch
	branchName := ""
	if len(args) > 0 {
		branchName = args[0]
	} else {
		current, err := repo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branchName = current
	}

	if branchName == cfg.Trunk {
		return fmt.Errorf("cannot land trunk branch")
	}

	// Confirm
	if !landForce {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Land branch '%s'?", branchName),
			Default: true,
		}
		if err := askOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(colors.Muted("Cancelled."))
			return nil
		}
	}

	return landSingleBranch(repo, metadata, prov, cfg, branchName)
}

func landSingleBranch(repo *git.Repo, metadata *config.Metadata, prov provider.Provider, cfg *config.Config, branchName string) error {
	currentBranch, _ := repo.GetCurrentBranch()

	result, err := land.Branch(repo, metadata, prov, cfg.Trunk, currentBranch, defaultRemote, land.Opts{
		Branch:         branchName,
		NoDeleteRemote: landNoDeleteRemote,
	})
	if err != nil {
		return err
	}

	// Save metadata and sync refs
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	deleteRemoteMetadataRef(repo, branchName)
	if len(result.ReparentedChildren) > 0 {
		pushMetadataRefs(repo, result.ReparentedChildren...)
	}

	// Restack children
	if len(result.ReparentedChildren) > 0 {
		s, err := stack.BuildStack(repo, cfg, metadata)
		if err == nil {
			parentNode := s.GetNode(result.NewParent)
			if parentNode != nil && len(parentNode.Children) > 0 {
				fmt.Println("Restacking children...")
				if err := restackChildren(repo, s, parentNode); err != nil {
					fmt.Printf("%s Could not restack children: %v\n", colors.Warning("⚠"), err)
				}
			}
		}
	}

	// Print results
	fmt.Printf("%s Landed '%s'\n", colors.Success("✓"), branchName)
	if result.CheckedOut != "" {
		fmt.Printf("  Checked out '%s'\n", result.CheckedOut)
	}
	for _, child := range result.ReparentedChildren {
		fmt.Printf("  Reparented '%s' to '%s'\n", child, result.NewParent)
	}
	for _, update := range result.UpdatedPRBases {
		fmt.Printf("  Updated PR #%d base to '%s'\n", update.PRNumber, update.NewBase)
	}

	return nil
}

func landStackBranches(repo *git.Repo, metadata *config.Metadata, prov provider.Provider, cfg *config.Config) error {
	merged, err := land.FindMergedBranches(repo, metadata, prov, cfg.Trunk)
	if err != nil {
		return fmt.Errorf("failed to find merged branches: %w", err)
	}

	if len(merged) == 0 {
		fmt.Println("No merged branches found.")
		return nil
	}

	// Build stack for depth sorting
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Sort deepest first to avoid reparenting conflicts
	sort.Slice(merged, func(i, j int) bool {
		return s.GetStackDepth(merged[i]) > s.GetStackDepth(merged[j])
	})

	fmt.Printf("Found %d merged branch(es):\n", len(merged))
	for _, b := range merged {
		fmt.Printf("  - %s\n", b)
	}

	if !landForce {
		var confirm bool
		prompt := &survey.Confirm{
			Message: "Land all merged branches?",
			Default: true,
		}
		if err := askOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(colors.Muted("Cancelled."))
			return nil
		}
	}

	fmt.Println()
	var landed int
	for _, branchName := range merged {
		if !metadata.IsTracked(branchName) {
			continue // already landed in a previous iteration
		}
		fmt.Printf("Landing '%s'...\n", branchName)
		if err := landSingleBranch(repo, metadata, prov, cfg, branchName); err != nil {
			fmt.Printf("  %s Failed: %v\n", colors.Warning("⚠"), err)
			continue
		}
		landed++
	}

	fmt.Printf("\n%s Landed %d branch(es)\n", colors.Success("✓"), landed)
	return nil
}
