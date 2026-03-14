package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var (
	commitMessage string
	commitAll     bool
	commitPatch   bool
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Create a commit on the current branch",
	Long: `Create a commit on the current branch.

Similar to git commit but with interactive prompts for staging changes.

Examples:
  gs commit -m "Add feature"     # Commit staged changes
  gs commit -am "Add feature"    # Stage all and commit
  gs commit -pm "Add feature"    # Interactive patch mode
  gs commit                      # Interactive mode`,
	Aliases: []string{"ci"},
	RunE:    runCommit,
}

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "Commit message")
	commitCmd.Flags().BoolVarP(&commitAll, "all", "a", false, "Stage all changes before committing")
	commitCmd.Flags().BoolVarP(&commitPatch, "patch", "p", false, "Interactively select hunks to stage")
}

func runCommit(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Check if gs is initialized
	if _, err := config.Load(repo.GetConfigPath()); err != nil {
		return err
	}

	// Stage all changes if --all flag is set
	if commitAll {
		if _, err := repo.RunGitCommand("add", "-A"); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
	}

	// Check for changes
	hasStaged := detectStagedChanges(repo)
	hasUnstaged := detectUnstagedChanges(repo)

	// Handle based on message and changes
	if commitMessage != "" {
		// Message provided
		if hasStaged {
			// Commit staged changes
			if err := doCommit(repo, commitMessage, commitPatch); err != nil {
				return err
			}
			fmt.Printf("%s Committed changes\n", colors.Success("✓"))
		} else if hasUnstaged {
			// No staged changes but have unstaged - auto-stage all (gt cli behavior)
			if _, err := repo.RunGitCommand("add", "-A"); err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}
			if err := doCommit(repo, commitMessage, commitPatch); err != nil {
				return err
			}
			fmt.Printf("%s Committed all changes\n", colors.Success("✓"))
		} else {
			printNoChangesInfo(repo)
		}
	} else {
		// No message provided
		if !hasStaged && !hasUnstaged {
			printNoChangesInfo(repo)
			return nil
		}

		// Prompt for what to do
		action, err := promptCommitActionNoMessage(hasStaged)
		if err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println(colors.Muted("Cancelled."))
				return nil
			}
			return err
		}

		switch action {
		case "staged":
			msg, err := promptCommitMessage()
			if err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					fmt.Println(colors.Muted("Cancelled."))
					return nil
				}
				return err
			}
			if err := doCommit(repo, msg, false); err != nil {
				return err
			}
			fmt.Printf("%s Committed staged changes\n", colors.Success("✓"))
		case "all":
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
			if err := doCommit(repo, msg, false); err != nil {
				return err
			}
			fmt.Printf("%s Committed all changes\n", colors.Success("✓"))
		case "patch":
			if err := promptTrackUntrackedFiles(repo); err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					fmt.Println(colors.Muted("Cancelled."))
					return nil
				}
				if errors.Is(err, errNoChangesToCommit) {
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
			if err := doCommit(repo, msg, true); err != nil {
				return err
			}
			fmt.Printf("%s Committed selected changes\n", colors.Success("✓"))
		case "abort":
			fmt.Println(colors.Muted("Cancelled."))
			return nil
		}
	}

	return nil
}

// doCommit performs the actual commit
func doCommit(repo *git.Repo, message string, patch bool) error {
	args := []string{"commit"}

	if patch {
		args = append(args, "-p")
	}

	args = append(args, "-m", message)

	if _, err := repo.RunGitCommand(args...); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

// promptCommitActionNoMessage prompts when no message and changes exist
func promptCommitActionNoMessage(hasStaged bool) (string, error) {
	var options []string

	if hasStaged {
		options = []string{
			"Commit staged changes",
			"Stage all changes and commit (--all)",
			"Select changes to commit (--patch)",
			"Abort",
		}
	} else {
		options = []string{
			"Stage all changes and commit (--all)",
			"Select changes to commit (--patch)",
			"Abort",
		}
	}

	prompt := &survey.Select{
		Message: "What would you like to commit?",
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
		default:
			return "abort", nil
		}
	}

	switch selected {
	case options[0]:
		return "all", nil
	case options[1]:
		return "patch", nil
	default:
		return "abort", nil
	}
}
