package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var initReset bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gs in the current repository",
	Long: `Initialize gs in the current git repository by selecting a trunk branch.

The trunk branch is the main branch that stacks are based on (typically 'main' or 'master').
This command creates the necessary configuration files in .git/ directory.

Use --reset to wipe all gs (and legacy gw) config files and start fresh.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initReset, "reset", false, "Remove all gs/gw config and reinitialize from scratch")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// --reset: wipe all gs and gw config files
	if initReset {
		removeConfigFiles(repo)
		fmt.Println("✓ Removed all gs/gw config files")
	}

	// Auto-migrate from gw if legacy files exist
	migrated, err := migrateFromGW(repo)
	if err != nil {
		return fmt.Errorf("failed to migrate from gw: %w", err)
	}
	if migrated {
		fmt.Println("✓ Migrated config from gw → gs")
	}

	// Check if already initialized (either natively or after migration)
	configPath := repo.GetConfigPath()
	if config.IsInitialized(configPath) {
		if migrated {
			fmt.Printf("  Config: %s\n", configPath)
			fmt.Printf("  Metadata: %s\n", repo.GetMetadataPath())
			return nil
		}
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

	// Configure fetch refspec for refs/gs/* so future fetches include metadata
	configureGSRefspec(repo)

	// Try to fetch existing metadata from remote (newcomer experience)
	remoteHasRefs := fetchMetadataRefs(repo)

	// Check if remote already has a config ref (team already uses gs)
	if remoteHasRefs {
		remoteCfg, err := config.ReadRefConfig(repo)
		if err == nil && remoteCfg.Trunk != "" {
			// Use the remote's trunk if it differs from selection
			if remoteCfg.Trunk != trunk {
				fmt.Printf("ℹ Remote already has gs config with trunk '%s', using it\n", remoteCfg.Trunk)
				trunk = remoteCfg.Trunk
			}
		}
	}

	// Create config (JSON + ref)
	cfg := config.NewConfig(trunk)
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	// Best-effort: also write config to git refs for team sync
	_ = config.WriteRefConfig(repo, cfg)

	// If remote had metadata refs, load them; otherwise create empty metadata
	var metadata *config.Metadata
	if remoteHasRefs {
		meta, source, loadErr := config.LoadMetadataWithRefs(repo, repo.GetMetadataPath())
		if loadErr == nil && source == config.SourceRefs && len(meta.Branches) > 0 {
			metadata = meta
			fmt.Printf("✓ Imported %d tracked branch(es) from remote\n", len(meta.Branches))
		}
	}
	if metadata == nil {
		metadata = &config.Metadata{
			Branches: make(map[string]*config.BranchMetadata),
		}
	}

	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Push config ref to remote so teammates can discover gs
	pushConfigRef(repo)

	fmt.Printf("✓ Initialized gs with trunk branch: %s\n", trunk)
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Printf("  Metadata: %s\n", repo.GetMetadataPath())
	fmt.Println("\nYou can now use 'gs track' to start tracking existing branches")
	fmt.Println("or 'gs create' to create new branches in your stack")

	return nil
}

// removeConfigFiles deletes all gs and legacy gw config files.
func removeConfigFiles(repo *git.Repo) {
	commonDir := repo.GetCommonDir()
	files := []string{
		".gs_config", ".gs_stack_metadata", ".gs_continue_state",
		".gw_config", ".gw_stack_metadata", ".gw_continue_state",
	}
	for _, f := range files {
		_ = os.Remove(filepath.Join(commonDir, f))
	}
}

// migrateFromGW renames legacy .gw_* files to .gs_* and removes the old files.
// Returns true if migration occurred.
func migrateFromGW(repo *git.Repo) (bool, error) {
	commonDir := repo.GetCommonDir()

	migrations := []struct{ old, new string }{
		{".gw_config", ".gs_config"},
		{".gw_stack_metadata", ".gs_stack_metadata"},
		{".gw_continue_state", ".gs_continue_state"},
	}

	migrated := false
	for _, m := range migrations {
		oldPath := filepath.Join(commonDir, m.old)
		newPath := filepath.Join(commonDir, m.new)

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue
		}

		// Don't overwrite existing gs files
		if _, err := os.Stat(newPath); err == nil {
			// gs file already exists, just remove the old one
			_ = os.Remove(oldPath)
			migrated = true
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			return false, fmt.Errorf("failed to rename %s → %s: %w", m.old, m.new, err)
		}
		migrated = true
	}

	return migrated, nil
}
