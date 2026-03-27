package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	splitByCommit bool
	splitByHunk   bool
	splitByFile   []string
	splitName     string
)

var splitCmd = &cobra.Command{
	Use:   "split",
	Short: "Split current branch into multiple branches",
	Long: `Split the current branch into two branches.

Three splitting methods available:
  --by-commit (-c): Split along commit boundaries
  --by-hunk (-u):   Interactively select hunks for new parent branch
  --by-file (-f):   Move files matching pattern to new parent branch

The new branch becomes the parent, and the current branch is rebased on top.

Example:
  gs split -c                    # Split by selecting commits
  gs split -u                    # Interactive hunk selection
  gs split -f "*.json"           # Split JSON files to parent
  gs split -f "src/**" -n base   # Split src/ to branch named 'base'`,
	RunE: runSplit,
}

func init() {
	splitCmd.Flags().BoolVarP(&splitByCommit, "by-commit", "c", false, "Split along commit boundaries")
	splitCmd.Flags().BoolVarP(&splitByHunk, "by-hunk", "u", false, "Interactively select hunks")
	splitCmd.Flags().StringArrayVarP(&splitByFile, "by-file", "f", nil, "Split files matching pattern")
	splitCmd.Flags().StringVarP(&splitName, "name", "n", "", "Name for the new parent branch")
	rootCmd.AddCommand(splitCmd)
}

func runSplit(cmd *cobra.Command, args []string) error {
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

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Cannot split trunk
	if currentBranch == cfg.Trunk {
		return fmt.Errorf("cannot split trunk branch")
	}

	// Must be tracked
	if !metadata.IsTracked(currentBranch) {
		return fmt.Errorf("branch '%s' is not tracked by gs", currentBranch)
	}

	// Get parent branch
	parentBranch, ok := metadata.GetParent(currentBranch)
	if !ok {
		return fmt.Errorf("branch has no parent")
	}

	// Count commits on this branch
	commitCount, err := countCommits(repo, currentBranch, parentBranch)
	if err != nil {
		return fmt.Errorf("failed to count commits: %w", err)
	}

	if commitCount == 0 {
		return fmt.Errorf("branch has no commits to split")
	}

	// Determine split mode
	modeCount := 0
	if splitByCommit {
		modeCount++
	}
	if splitByHunk {
		modeCount++
	}
	if len(splitByFile) > 0 {
		modeCount++
	}

	if modeCount > 1 {
		return fmt.Errorf("only one split mode can be specified")
	}

	// If no mode specified, prompt or default based on commit count
	if modeCount == 0 {
		if commitCount == 1 {
			// Single commit, default to hunk mode
			splitByHunk = true
			fmt.Println("Single commit detected, using hunk mode...")
		} else {
			// Multiple commits, prompt for mode
			mode, err := promptSplitMode()
			if err != nil {
				return err
			}
			switch mode {
			case "commit":
				splitByCommit = true
			case "hunk":
				splitByHunk = true
			case "file":
				return fmt.Errorf("file mode requires -f flag with pattern")
			}
		}
	}

	// Get new branch name
	newBranchName := splitName
	if newBranchName == "" {
		newBranchName, err = promptBranchName(currentBranch)
		if err != nil {
			return err
		}
	}

	// Validate new branch name
	if repo.BranchExists(newBranchName) {
		return fmt.Errorf("branch '%s' already exists", newBranchName)
	}

	// Execute split based on mode
	if splitByCommit {
		return splitByCommitMode(repo, cfg, metadata, currentBranch, parentBranch, newBranchName, commitCount)
	} else if splitByHunk {
		return splitByHunkMode(repo, cfg, metadata, currentBranch, parentBranch, newBranchName)
	} else if len(splitByFile) > 0 {
		return splitByFileMode(repo, cfg, metadata, currentBranch, parentBranch, newBranchName, splitByFile)
	}

	return fmt.Errorf("no split mode selected")
}

func countCommits(repo *git.Repo, branch, parent string) (int, error) {
	output, err := repo.RunGitCommand("rev-list", "--count", fmt.Sprintf("%s..%s", parent, branch))
	if err != nil {
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(output), "%d", &count)
	return count, err
}

func promptSplitMode() (string, error) {
	options := []string{
		"By commit - split along commit boundaries",
		"By hunk - interactively select changes",
	}

	prompt := &survey.Select{
		Message: "How would you like to split?",
		Options: options,
	}

	var selected string
	if err := askOne(prompt, &selected); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return "", fmt.Errorf("cancelled")
		}
		return "", err
	}

	if strings.HasPrefix(selected, "By commit") {
		return "commit", nil
	}
	return "hunk", nil
}

func promptBranchName(currentBranch string) (string, error) {
	prompt := &survey.Input{
		Message: "Name for new parent branch:",
		Default: currentBranch + "-base",
	}

	var name string
	if err := askOne(prompt, &name); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return "", fmt.Errorf("cancelled")
		}
		return "", err
	}

	return strings.TrimSpace(name), nil
}

func splitByCommitMode(repo *git.Repo, cfg *config.Config, metadata *config.Metadata, currentBranch, parentBranch, newBranchName string, commitCount int) error {
	// Get list of commits
	output, err := repo.RunGitCommand("log", "--oneline", "--reverse", fmt.Sprintf("%s..%s", parentBranch, currentBranch))
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	commits := strings.Split(strings.TrimSpace(output), "\n")
	if len(commits) < 2 {
		return fmt.Errorf("need at least 2 commits to split by commit")
	}

	// Let user select commits for new parent branch
	prompt := &survey.MultiSelect{
		Message: "Select commits for the NEW PARENT branch (remaining stay in current):",
		Options: commits,
	}

	var selected []string
	if err := askOne(prompt, &selected); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return fmt.Errorf("cancelled")
		}
		return err
	}

	if len(selected) == 0 {
		return fmt.Errorf("no commits selected")
	}

	if len(selected) == len(commits) {
		return fmt.Errorf("cannot move all commits to new branch")
	}

	// Get the SHA of the last selected commit (for the split point)
	lastSelectedCommit := selected[len(selected)-1]
	splitSHA := strings.Fields(lastSelectedCommit)[0]

	fmt.Printf("\nSplitting at commit %s...\n", splitSHA)

	// Create new branch at split point
	if _, err := repo.RunGitCommand("branch", newBranchName, splitSHA); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Track new branch with parent as its parent
	parentSHA, _ := repo.GetBranchCommit(parentBranch)
	metadata.TrackBranch(newBranchName, parentBranch, parentSHA)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Update current branch's parent to new branch
	if err := metadata.UpdateParent(currentBranch, newBranchName); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Rebase current branch onto new branch
	fmt.Printf("Rebasing '%s' onto '%s'...\n", currentBranch, newBranchName)
	if _, err := repo.RunGitCommand("rebase", "--onto", newBranchName, splitSHA, currentBranch); err != nil {
		return fmt.Errorf("rebase failed: %w\nResolve conflicts and run: gs stack restack", err)
	}

	fmt.Printf("\n✓ Created '%s' with %d commit(s)\n", newBranchName, len(selected))
	fmt.Printf("✓ '%s' now has %d commit(s) on top of '%s'\n", currentBranch, len(commits)-len(selected), newBranchName)

	return nil
}

func splitByHunkMode(repo *git.Repo, cfg *config.Config, metadata *config.Metadata, currentBranch, parentBranch, newBranchName string) error {
	// Save current HEAD
	currentHEAD, err := repo.RunGitCommand("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	currentHEAD = strings.TrimSpace(currentHEAD)

	// Create new branch from parent
	fmt.Printf("Creating '%s' from '%s'...\n", newBranchName, parentBranch)
	if _, err := repo.RunGitCommand("checkout", "-b", newBranchName, parentBranch); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Cherry-pick all changes from current branch as unstaged
	fmt.Println("Preparing changes for interactive staging...")
	if _, err := repo.RunGitCommand("cherry-pick", "-n", fmt.Sprintf("%s..%s", parentBranch, currentHEAD)); err != nil {
		// Reset if cherry-pick fails
		if _, abortErr := repo.RunGitCommand("cherry-pick", "--abort"); abortErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to abort cherry-pick: %v", err, abortErr)
		}
		if _, checkoutErr := repo.RunGitCommand("checkout", currentBranch); checkoutErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to checkout '%s': %v", err, currentBranch, checkoutErr)
		}
		if _, deleteErr := repo.RunGitCommand("branch", "-D", newBranchName); deleteErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to delete branch '%s': %v", err, newBranchName, deleteErr)
		}
		return fmt.Errorf("failed to prepare changes: %w", err)
	}

	// Reset to unstage everything
	if _, err := repo.RunGitCommand("reset", "HEAD"); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	// Interactive staging
	fmt.Println("\nSelect changes for the NEW PARENT branch:")
	fmt.Println("(y=stage, n=skip, s=split, q=quit staging)")
	if os.Getenv("GW_TEST_AUTO_STAGE") != "" {
		if _, err := repo.RunGitCommand("add", "-A"); err != nil {
			return fmt.Errorf("failed to auto-stage changes: %w", err)
		}
	} else if _, err := repo.RunGitCommand("add", "--patch"); err != nil {
		fmt.Printf("Interactive staging exited: %v\n", err)
	}

	// Check if anything was staged
	staged, err := hasStagedChanges(repo)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if !staged {
		// Nothing staged, abort
		fmt.Println("No changes staged, aborting split...")
		if _, checkoutErr := repo.RunGitCommand("checkout", currentBranch); checkoutErr != nil {
			return fmt.Errorf("failed to abort split; also failed to checkout '%s': %v", currentBranch, checkoutErr)
		}
		if _, deleteErr := repo.RunGitCommand("branch", "-D", newBranchName); deleteErr != nil {
			return fmt.Errorf("failed to abort split; also failed to delete branch '%s': %v", newBranchName, deleteErr)
		}
		return fmt.Errorf("no changes selected for new branch")
	}

	// Commit staged changes to new branch
	if _, err := repo.RunGitCommand("commit", "-m", fmt.Sprintf("Split from %s", currentBranch)); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Clean up unstaged changes
	if _, err := repo.RunGitCommand("checkout", "--", "."); err != nil {
		return fmt.Errorf("failed to discard unstaged changes: %w", err)
	}

	// Track new branch
	parentSHA, _ := repo.GetBranchCommit(parentBranch)
	metadata.TrackBranch(newBranchName, parentBranch, parentSHA)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Go back to current branch
	if err := repo.CheckoutBranch(currentBranch); err != nil {
		return fmt.Errorf("failed to checkout original branch: %w", err)
	}

	// Update current branch's parent
	if err := metadata.UpdateParent(currentBranch, newBranchName); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Rebase current branch onto new branch
	fmt.Printf("\nRebasing '%s' onto '%s'...\n", currentBranch, newBranchName)
	if _, err := repo.RunGitCommand("rebase", newBranchName); err != nil {
		return fmt.Errorf("rebase failed: %w\nResolve conflicts and run: gs stack restack", err)
	}

	// Build stack to restack children
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	node := s.GetNode(currentBranch)
	if node != nil && len(node.Children) > 0 {
		fmt.Println("\nRestacking children...")
		if err := restackChildren(repo, s, node); err != nil {
			return fmt.Errorf("failed to restack children: %w", err)
		}
	}

	fmt.Printf("\n✓ Created '%s' as new parent\n", newBranchName)
	fmt.Printf("✓ '%s' rebased on top\n", currentBranch)

	return nil
}

func splitByFileMode(repo *git.Repo, cfg *config.Config, metadata *config.Metadata, currentBranch, parentBranch, newBranchName string, patterns []string) error {
	// Save current HEAD
	currentHEAD, err := repo.RunGitCommand("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	currentHEAD = strings.TrimSpace(currentHEAD)

	// Create new branch from parent
	fmt.Printf("Creating '%s' from '%s'...\n", newBranchName, parentBranch)
	if _, err := repo.RunGitCommand("checkout", "-b", newBranchName, parentBranch); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Cherry-pick all changes as unstaged
	if _, err := repo.RunGitCommand("cherry-pick", "-n", fmt.Sprintf("%s..%s", parentBranch, currentHEAD)); err != nil {
		if _, abortErr := repo.RunGitCommand("cherry-pick", "--abort"); abortErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to abort cherry-pick: %v", err, abortErr)
		}
		if _, checkoutErr := repo.RunGitCommand("checkout", currentBranch); checkoutErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to checkout '%s': %v", err, currentBranch, checkoutErr)
		}
		if _, deleteErr := repo.RunGitCommand("branch", "-D", newBranchName); deleteErr != nil {
			return fmt.Errorf("failed to prepare changes: %w\nAlso failed to delete branch '%s': %v", err, newBranchName, deleteErr)
		}
		return fmt.Errorf("failed to prepare changes: %w", err)
	}

	// Reset to unstage
	if _, err := repo.RunGitCommand("reset", "HEAD"); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	// Stage only files matching patterns
	for _, pattern := range patterns {
		fmt.Printf("Adding files matching '%s'...\n", pattern)
		if _, err := repo.RunGitCommand("add", pattern); err != nil {
			return fmt.Errorf("failed to add pattern '%s': %w", pattern, err)
		}
	}

	// Check if anything was staged
	staged, err := hasStagedChanges(repo)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if !staged {
		fmt.Println("No files matched patterns, aborting split...")
		if _, checkoutErr := repo.RunGitCommand("checkout", currentBranch); checkoutErr != nil {
			return fmt.Errorf("failed to abort split; also failed to checkout '%s': %v", currentBranch, checkoutErr)
		}
		if _, deleteErr := repo.RunGitCommand("branch", "-D", newBranchName); deleteErr != nil {
			return fmt.Errorf("failed to abort split; also failed to delete branch '%s': %v", newBranchName, deleteErr)
		}
		return fmt.Errorf("no files matched the specified patterns")
	}

	// Commit staged changes
	patternStr := strings.Join(patterns, ", ")
	if _, err := repo.RunGitCommand("commit", "-m", fmt.Sprintf("Split files (%s) from %s", patternStr, currentBranch)); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Clean up unstaged changes
	if _, err := repo.RunGitCommand("checkout", "--", "."); err != nil {
		return fmt.Errorf("failed to discard unstaged changes: %w", err)
	}

	// Track new branch
	parentSHA, _ := repo.GetBranchCommit(parentBranch)
	metadata.TrackBranch(newBranchName, parentBranch, parentSHA)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Go back to current branch
	if err := repo.CheckoutBranch(currentBranch); err != nil {
		return fmt.Errorf("failed to checkout original branch: %w", err)
	}

	// Update parent
	if err := metadata.UpdateParent(currentBranch, newBranchName); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Rebase onto new branch
	fmt.Printf("\nRebasing '%s' onto '%s'...\n", currentBranch, newBranchName)
	if _, err := repo.RunGitCommand("rebase", newBranchName); err != nil {
		return fmt.Errorf("rebase failed: %w\nResolve conflicts and run: gs stack restack", err)
	}

	// Restack children
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	node := s.GetNode(currentBranch)
	if node != nil && len(node.Children) > 0 {
		fmt.Println("\nRestacking children...")
		if err := restackChildren(repo, s, node); err != nil {
			return fmt.Errorf("failed to restack children: %w", err)
		}
	}

	fmt.Printf("\n✓ Created '%s' with files matching: %s\n", newBranchName, patternStr)
	fmt.Printf("✓ '%s' rebased on top\n", currentBranch)

	return nil
}
