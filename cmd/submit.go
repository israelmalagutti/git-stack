package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/israelmalagutti/git-stack/internal/submit"
	"github.com/spf13/cobra"
)

var (
	submitDraft  bool
	submitStack  bool
	submitTitle  string
	submitNoPush bool
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Create or update a pull request",
	Long: `Submit the current branch as a pull request.

If a PR already exists for the branch, it is updated (push is sufficient).
If no PR exists, one is created with the correct base branch from metadata.
The PR number is stored in metadata refs so teammates can see it.

Examples:
  gs submit                    # submit current branch
  gs submit --draft            # submit as draft PR
  gs submit --stack            # submit all branches in stack
  gs submit -m "PR title"      # custom title`,
	Aliases: []string{"s"},
	RunE:    runSubmit,
}

func init() {
	submitCmd.Flags().BoolVarP(&submitDraft, "draft", "d", false, "Submit as draft PR")
	submitCmd.Flags().BoolVar(&submitStack, "stack", false, "Submit all branches in current stack")
	submitCmd.Flags().StringVarP(&submitTitle, "message", "m", "", "PR title (default: first commit or branch name)")
	submitCmd.Flags().BoolVar(&submitNoPush, "no-push", false, "Don't push before submitting")
	rootCmd.AddCommand(submitCmd)
}

func runSubmit(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	prov, err := detectProvider(rs.Repo)
	if err != nil {
		return err
	}

	if !prov.CLIAvailable() {
		return fmt.Errorf("provider CLI not found.\nInstall gh from https://cli.github.com/")
	}
	if !prov.CLIAuthenticated() {
		return fmt.Errorf("provider CLI not authenticated.\nRun: gh auth login")
	}

	if submitStack {
		return submitStackBranches(rs.Repo, rs.Metadata, prov, rs.Stack)
	}

	return submitCurrentBranch(rs.Repo, rs.Metadata, prov, rs.Stack)
}

func submitCurrentBranch(repo *git.Repo, metadata *config.Metadata, prov provider.Provider, s *stack.Stack) error {
	node := s.GetNode(s.Current)
	if node == nil {
		return fmt.Errorf("current branch is not tracked by gs")
	}
	if node.IsTrunk {
		return fmt.Errorf("cannot submit trunk branch")
	}
	if node.Parent == nil {
		return fmt.Errorf("branch has no parent")
	}

	parentBranch := node.Parent.Name

	// Ensure parent is pushed if it's not trunk
	if !node.Parent.IsTrunk && !repo.HasRemoteBranch(parentBranch, defaultRemote) {
		fmt.Printf("Pushing parent '%s' to %s...\n", parentBranch, defaultRemote)
		if _, err := repo.RunGitCommand("push", "-u", defaultRemote, parentBranch); err != nil {
			return fmt.Errorf("failed to push parent '%s': %w", parentBranch, err)
		}
	}

	result, err := submit.Branch(repo, metadata, prov, defaultRemote, submit.Opts{
		Branch: s.Current,
		Parent: parentBranch,
		Draft:  submitDraft,
		Title:  submitTitle,
		NoPush: submitNoPush,
	})
	if err != nil {
		return err
	}

	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	pushMetadataRefs(repo, s.Current)

	printSubmitResult(result)
	return nil
}

func submitStackBranches(repo *git.Repo, metadata *config.Metadata, prov provider.Provider, s *stack.Stack) error {
	path := s.FindPath(s.Current)
	if path == nil {
		return fmt.Errorf("could not find stack path for current branch")
	}

	var results []*submit.Result
	for _, node := range path {
		if node.IsTrunk {
			continue
		}
		if node.Parent == nil {
			continue
		}

		parentBranch := node.Parent.Name

		fmt.Printf("Submitting %s...\n", colors.BranchCurrent(node.Name))

		result, err := submit.Branch(repo, metadata, prov, defaultRemote, submit.Opts{
			Branch: node.Name,
			Parent: parentBranch,
			Draft:  submitDraft,
			NoPush: submitNoPush,
		})
		if err != nil {
			fmt.Printf("  %s Failed: %v\n", colors.Warning("⚠"), err)
			continue
		}

		results = append(results, result)
		printSubmitResult(result)
	}

	if len(results) > 0 {
		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
		pushMetadataRefs(repo)
	}

	fmt.Printf("\n%s Submitted %d branch(es)\n", colors.Success("✓"), len(results))
	return nil
}

func printSubmitResult(r *submit.Result) {
	action := colors.Success("Created")
	if r.Action == "updated" {
		action = colors.Muted("Updated")
	}
	fmt.Printf("  %s #%d %s\n", action, r.PRNumber, r.PRURL)
}

func detectProvider(repo *git.Repo) (provider.Provider, error) {
	remoteURL, err := repo.GetRemoteURL(defaultRemote)
	if err != nil {
		return nil, fmt.Errorf("no remote '%s' configured: %w", defaultRemote, err)
	}
	prov, err := provider.DetectFromRemoteURL(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("could not detect provider: %w", err)
	}
	return prov, nil
}
