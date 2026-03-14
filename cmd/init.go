package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gs in the current repository",
	Long: `Initialize gs in the current git repository by selecting a trunk branch.

The trunk branch is the main branch that stacks are based on (typically 'main' or 'master').
This command creates the necessary configuration files in .git/ directory.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Check if already initialized
	configPath := repo.GetConfigPath()
	if config.IsInitialized(configPath) {
		return fmt.Errorf("gs is already initialized in this repository\nConfig file: %s", configPath)
	}

	// Get list of branches
	branches, err := repo.ListBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	if len(branches) == 0 {
		return fmt.Errorf("no branches found in repository\nCreate at least one branch before running 'gs init'")
	}

	// Prompt user to select trunk branch
	var trunk string
	prompt := &survey.Select{
		Message: "Select trunk branch:",
		Options: branches,
		Description: func(value string, index int) string {
			// Try to get current branch
			current, err := repo.GetCurrentBranch()
			if err == nil && value == current {
				return "(current)"
			}
			return ""
		},
	}

	// Try to find common trunk names and set as default
	defaultIndex := 0
	for i, branch := range branches {
		if branch == "main" || branch == "master" {
			defaultIndex = i
			break
		}
	}
	prompt.Default = defaultIndex

	err = askOne(prompt, &trunk, survey.WithValidator(survey.Required))
	if err != nil {
		// Handle ESC/Ctrl+C gracefully
		if errors.Is(err, terminal.InterruptErr) {
			fmt.Println("Cancelled.")
			return nil
		}
		return fmt.Errorf("failed to get trunk selection: %w", err)
	}

	// Verify the selected branch exists
	if !repo.BranchExists(trunk) {
		return fmt.Errorf("selected branch %s does not exist", trunk)
	}

	// Create config
	cfg := config.NewConfig(trunk)
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create empty metadata
	metadata := &config.Metadata{
		Branches: make(map[string]*config.BranchMetadata),
	}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	fmt.Printf("✓ Initialized gs with trunk branch: %s\n", trunk)
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Printf("  Metadata: %s\n", repo.GetMetadataPath())
	fmt.Println("\nYou can now use 'gs track' to start tracking existing branches")
	fmt.Println("or 'gs create' to create new branches in your stack")

	return nil
}
