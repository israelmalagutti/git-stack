package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteBranchNames_Empty(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	names, directive := completeBranchNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
	// Should at least contain trunk
	if len(names) == 0 {
		t.Error("expected at least one branch name (trunk)")
	}
	found := false
	for _, n := range names {
		if n == "main" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'main' in branch names")
	}
}

func TestCompleteBranchNames_WithTrackedBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-a", "main")
	tr.createBranch(t, "feat-b", "main")

	names, directive := completeBranchNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive")
	}
	if len(names) < 3 {
		t.Errorf("expected at least 3 branch names (main, feat-a, feat-b), got %d", len(names))
	}
}

func TestCompleteBranchNames_AlreadyHasArg(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// When args already has an element, should return nil (no more completions)
	names, directive := completeBranchNames(nil, []string{"existing"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive")
	}
	if names != nil {
		t.Errorf("expected nil names when arg already provided, got %v", names)
	}
}

func TestCompleteBranchNamesFunc(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-comp", "main")

	names, directive := completeBranchNamesFunc(nil, nil, "feat")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive")
	}
	if len(names) == 0 {
		t.Error("expected some branch names")
	}
}
