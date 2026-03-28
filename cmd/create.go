package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var (
	createMessage string
	createAll     bool
	createPatch   bool
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new branch stacked on the current branch",
	Long: `Create a new branch stacked on top of the current branch.

The new branch will be automatically tracked with the current branch as its parent.

Examples:
  gs create feat-auth                    # Create empty branch
  gs create feat-auth -m "Add login"     # Create and commit staged changes
  gs create feat-auth -am "Add login"    # Stage all changes and commit
  gs create feat-auth -pm "Add login"    # Interactive patch mode
  gs create -m "Add login"               # Auto-generate branch name from message`,
	Aliases: []string{"c"},
	RunE:    runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createMessage, "message", "m", "", "Commit staged changes with this message")
	createCmd.Flags().BoolVarP(&createAll, "all", "a", false, "Stage all unstaged changes before committing")
	createCmd.Flags().BoolVarP(&createPatch, "patch", "p", false, "Interactively select hunks to stage")
}

func runCreate(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Check if gs is initialized
	_, err = config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	// Get current branch (parent of new branch)
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Determine branch name
	branchName := resolveBranchName(args, createMessage)
	if branchName == "" {
		// Prompt for branch name if not provided
		prompt := &survey.Input{
			Message: "Branch name:",
		}
		err = askOne(prompt, &branchName, survey.WithValidator(survey.Required))
		if err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println(colors.Muted("Cancelled."))
				return nil
			}
			return fmt.Errorf("failed to get branch name: %w", err)
		}
		// Sanitize user input (replace spaces with dashes, etc.)
		branchName = sanitizeBranchName(branchName)
	}

	// Validate branch name
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Check if branch already exists
	if repo.BranchExists(branchName) {
		return fmt.Errorf("branch '%s' already exists", branchName)
	}

	// Create and checkout the new branch
	if err := repo.CreateBranch(branchName); err != nil {
		return err
	}

	if err := repo.CheckoutBranch(branchName); err != nil {
		return err
	}

	// Track the branch in metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	parentSHA, _ := repo.GetBranchCommit(currentBranch)
	metadata.TrackBranch(branchName, currentBranch, parentSHA)

	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Push new branch's metadata ref to remote
	pushMetadataRefs(repo, branchName)

	colors.PrintCreated(branchName, currentBranch)

	// Stage all changes if --all flag is set
	if createAll {
		if _, err := repo.RunGitCommand("add", "-A"); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
	}

	// Check for changes
	hasStaged := detectStagedChanges(repo)
	hasUnstaged := detectUnstagedChanges(repo)

	// Rollback helper
	rollback := func() {
		_ = repo.CheckoutBranch(currentBranch)
		_ = repo.DeleteBranch(branchName, true)
		metadata.UntrackBranch(branchName)
		_ = metadata.SaveWithRefs(repo, repo.GetMetadataPath())
		deleteRemoteMetadataRef(repo, branchName)
	}

	// Handle commit logic based on changes and flags
	if createMessage != "" {
		// Message provided via flag
		if hasStaged {
			// Staged changes exist - commit them
			if err := commitChanges(repo, createMessage, createPatch); err != nil {
				rollback()
				return fmt.Errorf("failed to commit: %w", err)
			}
			fmt.Println(colors.Success("✓") + " Committed changes")
		} else if hasUnstaged {
			// No staged but unstaged exist - prompt user
			action, err := promptNoStagedChanges()
			if err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					rollback()
					fmt.Println(colors.Muted("Aborted."))
					return nil
				}
				return err
			}

			switch action {
			case "all":
				if _, err := repo.RunGitCommand("add", "-A"); err != nil {
					return fmt.Errorf("failed to stage changes: %w", err)
				}
				if err := commitChanges(repo, createMessage, false); err != nil {
					rollback()
					return fmt.Errorf("failed to commit: %w", err)
				}
				fmt.Println(colors.Success("✓") + " Committed all changes")
			case "patch":
				// Prompt for untracked files before patch mode
				if err := promptTrackUntrackedFiles(repo); err != nil {
					if errors.Is(err, terminal.InterruptErr) {
						rollback()
						fmt.Println(colors.Muted("Aborted."))
						return nil
					}
					if errors.Is(err, errNoChangesToCommit) {
						rollback()
						printNoChangesInfo(repo)
						return nil
					}
					return err
				}
				if err := commitChanges(repo, createMessage, true); err != nil {
					rollback()
					return fmt.Errorf("failed to commit: %w", err)
				}
				fmt.Println(colors.Success("✓") + " Committed selected changes")
			case "no-commit":
				fmt.Println(colors.Muted("Created branch with no commit."))
			case "abort":
				rollback()
				fmt.Println(colors.Muted("Aborted."))
				return nil
			}
		} else {
			// No changes at all
			fmt.Println(colors.Muted("No changes to commit; created branch with no commit."))
		}
	} else {
		// No message provided
		if hasStaged || hasUnstaged {
			// Changes exist - prompt for what to do
			action, err := promptHasChanges(hasStaged)
			if err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					fmt.Println(colors.Muted("Cancelled."))
					return nil
				}
				return err
			}

			switch action {
			case "all":
				// Stage all and prompt for message
				if _, err := repo.RunGitCommand("add", "-A"); err != nil {
					return fmt.Errorf("failed to stage changes: %w", err)
				}
				msg, err := promptCommitMessage()
				if err != nil {
					if errors.Is(err, terminal.InterruptErr) {
						fmt.Println(colors.Muted("Cancelled."))
						return nil
					}
					return err
				}
				if err := commitChanges(repo, msg, false); err != nil {
					return fmt.Errorf("failed to commit: %w", err)
				}
				fmt.Println(colors.Success("✓") + " Committed all changes")
			case "patch":
				// Prompt for untracked files before patch mode
				if err := promptTrackUntrackedFiles(repo); err != nil {
					if errors.Is(err, terminal.InterruptErr) {
						fmt.Println(colors.Muted("Cancelled."))
						return nil
					}
					if errors.Is(err, errNoChangesToCommit) {
						rollback()
						printNoChangesInfo(repo)
						return nil
					}
					return err
				}
				msg, err := promptCommitMessage()
				if err != nil {
					if errors.Is(err, terminal.InterruptErr) {
						fmt.Println(colors.Muted("Cancelled."))
						return nil
					}
					return err
				}
				if err := commitChanges(repo, msg, true); err != nil {
					return fmt.Errorf("failed to commit: %w", err)
				}
				fmt.Println(colors.Success("✓") + " Committed selected changes")
			case "staged":
				// Commit only staged changes
				msg, err := promptCommitMessage()
				if err != nil {
					if errors.Is(err, terminal.InterruptErr) {
						fmt.Println(colors.Muted("Cancelled."))
						return nil
					}
					return err
				}
				if err := commitChanges(repo, msg, false); err != nil {
					return fmt.Errorf("failed to commit: %w", err)
				}
				fmt.Println(colors.Success("✓") + " Committed staged changes")
			case "no-commit":
				fmt.Println(colors.Muted("Created branch with no commit."))
			case "abort":
				rollback()
				fmt.Println(colors.Muted("Aborted."))
				return nil
			}
		} else {
			// No changes at all
			fmt.Println()
			fmt.Println(colors.Muted("Next steps:"))
			fmt.Println(colors.Muted("  Make your changes, then:"))
			fmt.Println(colors.Muted("  - gs modify -m \"message\"  to commit"))
			fmt.Println(colors.Muted("  - gs log                   to see your stack"))
		}
	}

	return nil
}

// promptNoStagedChanges prompts when message given but no staged changes
func promptNoStagedChanges() (string, error) {
	options := []string{
		"Commit all file changes (--all)",
		"Select changes to commit (--patch)",
		"Create a branch with no commit",
		"Abort this operation",
	}

	prompt := &survey.Select{
		Message: "You have no staged changes. What would you like to do?",
		Options: options,
	}

	var selected string
	if err := askOne(prompt, &selected); err != nil {
		return "", err
	}

	switch selected {
	case options[0]:
		return "all", nil
	case options[1]:
		return "patch", nil
	case options[2]:
		return "no-commit", nil
	default:
		return "abort", nil
	}
}

// promptHasChanges prompts when no message but changes exist
func promptHasChanges(hasStaged bool) (string, error) {
	var options []string

	if hasStaged {
		options = []string{
			"Commit staged changes",
			"Commit all file changes (--all)",
			"Select changes to commit (--patch)",
			"Create a branch with no commit",
			"Abort this operation",
		}
	} else {
		options = []string{
			"Commit all file changes (--all)",
			"Select changes to commit (--patch)",
			"Create a branch with no commit",
			"Abort this operation",
		}
	}

	prompt := &survey.Select{
		Message: "You have uncommitted changes. What would you like to do?",
		Options: options,
	}

	var selected string
	if err := askOne(prompt, &selected); err != nil {
		return "", err
	}

	if hasStaged {
		switch selected {
		case options[0]:
			return "staged", nil
		case options[1]:
			return "all", nil
		case options[2]:
			return "patch", nil
		case options[3]:
			return "no-commit", nil
		default:
			return "abort", nil
		}
	}

	switch selected {
	case options[0]:
		return "all", nil
	case options[1]:
		return "patch", nil
	case options[2]:
		return "no-commit", nil
	default:
		return "abort", nil
	}
}

// promptCommitMessage prompts for a commit message
func promptCommitMessage() (string, error) {
	var msg string
	prompt := &survey.Input{
		Message: "Commit message:",
	}
	if err := askOne(prompt, &msg, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}
	return msg, nil
}

// resolveBranchName determines the branch name from args or message
func resolveBranchName(args []string, message string) string {
	if len(args) > 0 {
		return sanitizeBranchName(args[0])
	}

	// Auto-generate from commit message if provided
	if message != "" {
		return generateBranchName(message)
	}

	return ""
}

// sanitizeBranchName cleans up a branch name
func sanitizeBranchName(name string) string {
	// Replace spaces and special chars with dashes
	replacer := strings.NewReplacer(
		" ", "-",
		":", "-",
		"~", "-",
		"^", "-",
		"?", "-",
		"*", "-",
		"[", "-",
		"]", "-",
		"\\", "-",
	)
	name = replacer.Replace(name)

	// Remove leading/trailing dashes and dots
	name = strings.Trim(name, "-.")

	// Collapse multiple dashes
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return strings.ToLower(name)
}

// generateBranchName creates a branch name from a commit message
func generateBranchName(message string) string {
	// Take first line only
	if idx := strings.Index(message, "\n"); idx != -1 {
		message = message[:idx]
	}

	// Truncate long messages
	if len(message) > 50 {
		message = message[:50]
	}

	return sanitizeBranchName(message)
}

// detectStagedChanges checks if there are staged changes
func detectStagedChanges(repo *git.Repo) bool {
	output, err := repo.RunGitCommand("diff", "--cached", "--shortstat")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) != ""
}

// hasStagedChanges is an alias for detectStagedChanges (used by other commands)
func hasStagedChanges(repo *git.Repo) (bool, error) {
	return detectStagedChanges(repo), nil
}

// detectUnstagedChanges checks if there are unstaged changes (modified or untracked)
func detectUnstagedChanges(repo *git.Repo) bool {
	// Check for modified tracked files
	output, err := repo.RunGitCommand("diff", "--shortstat")
	if err == nil && strings.TrimSpace(output) != "" {
		return true
	}

	// Check for untracked files
	output, err = repo.RunGitCommand("ls-files", "--others", "--exclude-standard")
	if err == nil && strings.TrimSpace(output) != "" {
		return true
	}

	return false
}

// commitChanges commits with the given message, optionally using patch mode
func commitChanges(repo *git.Repo, message string, patch bool) error {
	args := []string{"commit"}

	if patch {
		args = append(args, "-p")
	}

	args = append(args, "-m", message)

	_, err := repo.RunGitCommand(args...)
	return err
}

// getUntrackedFiles returns a list of untracked files
func getUntrackedFiles(repo *git.Repo) []string {
	output, err := repo.RunGitCommand("ls-files", "--others", "--exclude-standard")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	files := strings.Split(strings.TrimSpace(output), "\n")
	result := make([]string, 0, len(files))
	for _, f := range files {
		if f = strings.TrimSpace(f); f != "" {
			result = append(result, f)
		}
	}
	return result
}

// errNoChangesToCommit is returned when there are no changes available for patch mode
var errNoChangesToCommit = fmt.Errorf("no changes to commit")

// hasTrackedChanges checks if there are any staged or modified tracked files
func hasTrackedChanges(repo *git.Repo) bool {
	// Check for staged changes
	if detectStagedChanges(repo) {
		return true
	}

	// Check for modified tracked files (not untracked)
	output, err := repo.RunGitCommand("diff", "--shortstat")
	if err == nil && strings.TrimSpace(output) != "" {
		return true
	}

	return false
}

// printNoChangesInfo prints detailed info when there are no changes to commit
func printNoChangesInfo(repo *git.Repo) {
	fmt.Println()
	fmt.Println(colors.Warning("No changes."))

	// Get HEAD info
	headSHA, err := repo.RunGitCommand("rev-parse", "--short", "HEAD")
	if err == nil {
		headSHA = strings.TrimSpace(headSHA)
		branchName, branchErr := repo.GetCurrentBranch()
		if branchErr == nil && branchName != "" {
			fmt.Printf("On branch %s (%s)\n", colors.BranchCurrent(branchName), colors.Muted(headSHA))
		} else {
			fmt.Printf("HEAD detached at %s\n", colors.Muted(headSHA))
		}
	}

	// List untracked files
	untrackedFiles := getUntrackedFiles(repo)
	if len(untrackedFiles) > 0 {
		fmt.Println()
		fmt.Println(colors.Muted("Untracked files:"))
		for _, file := range untrackedFiles {
			fmt.Printf("  %s\n", colors.Muted(file))
		}
	}
	fmt.Println()
}

// promptTrackUntrackedFiles asks if user wants to track untracked files before patch
// Returns errNoChangesToCommit if user declines and there are no other tracked changes
func promptTrackUntrackedFiles(repo *git.Repo) error {
	untrackedFiles := getUntrackedFiles(repo)
	if len(untrackedFiles) == 0 {
		// No untracked files - check if there are tracked changes
		if !hasTrackedChanges(repo) {
			return errNoChangesToCommit
		}
		return nil
	}

	var trackThem bool
	prompt := &survey.Confirm{
		Message: "We detected untracked files in your working tree. Would you like to track any of them?",
		Default: false,
	}

	if err := askOne(prompt, &trackThem); err != nil {
		return err
	}

	if !trackThem {
		// User declined - check if there are tracked changes to commit
		if !hasTrackedChanges(repo) {
			return errNoChangesToCommit
		}
		return nil
	}

	// Let user select which files to track
	var selectedFiles []string
	selectPrompt := &survey.MultiSelect{
		Message: "Select files to track:",
		Options: untrackedFiles,
	}

	if err := askOne(selectPrompt, &selectedFiles); err != nil {
		return err
	}

	// User selected nothing - check if there are tracked changes
	if len(selectedFiles) == 0 {
		if !hasTrackedChanges(repo) {
			return errNoChangesToCommit
		}
		return nil
	}

	// Add selected files
	for _, file := range selectedFiles {
		if _, err := repo.RunGitCommand("add", file); err != nil {
			return fmt.Errorf("failed to add %s: %w", file, err)
		}
	}

	return nil
}
