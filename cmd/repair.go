package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/repair"
	"github.com/spf13/cobra"
)

var (
	repairDryRun bool
	repairForce  bool
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Validate and fix stack metadata",
	Long: `Scan all metadata for inconsistencies against the actual git state and fix them.

Checks for:
  - Orphaned refs (branch deleted but metadata remains)
  - Missing parents (parent branch no longer exists)
  - Circular parent chains
  - Ref/JSON metadata mismatches
  - Remote-deleted branches (local branch exists but remote was deleted)

Example:
  gs repair              # Interactive: report and prompt for each fix
  gs repair --dry-run    # Report issues without fixing
  gs repair --force      # Fix all issues without prompting`,
	RunE: runRepair,
}

func init() {
	repairCmd.Flags().BoolVar(&repairDryRun, "dry-run", false, "Report issues without fixing")
	repairCmd.Flags().BoolVarP(&repairForce, "force", "f", false, "Fix all issues without prompting")
	rootCmd.AddCommand(repairCmd)
}

func runRepair(cmd *cobra.Command, args []string) error {
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

	issues, err := repair.DetectIssues(repo, metadata, cfg)
	if err != nil {
		return fmt.Errorf("failed to scan for issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Printf("%s No issues found. Stack metadata is healthy.\n", colors.Success("✓"))
		return nil
	}

	fmt.Printf("Found %d issue(s):\n\n", len(issues))
	for i, iss := range issues {
		fmt.Printf("  %d. [%s] %s\n", i+1, iss.Kind, iss.Description)
		fmt.Printf("     Fix: %s\n\n", iss.Fix)
	}

	if repairDryRun {
		fmt.Println(colors.Muted("Dry run — no changes made."))
		return nil
	}

	var fixed int
	for _, iss := range issues {
		if !repairForce {
			var confirm bool
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Fix: %s?", iss.Fix),
				Default: true,
			}
			if err := askOne(prompt, &confirm); err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					fmt.Println(colors.Muted("Cancelled."))
					break
				}
				return err
			}
			if !confirm {
				continue
			}
		}

		if err := repair.ApplyFix(repo, metadata, cfg, iss); err != nil {
			fmt.Printf("  %s Failed to fix '%s': %v\n", colors.Warning("⚠"), iss.Branch, err)
			continue
		}
		fmt.Printf("  %s Fixed: %s\n", colors.Success("✓"), iss.Fix)
		fixed++
	}

	if fixed > 0 {
		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
		pushMetadataRefs(repo)
		fmt.Printf("\n%s Fixed %d issue(s).\n", colors.Success("✓"), fixed)
	} else {
		fmt.Println(colors.Muted("No fixes applied."))
	}

	return nil
}
