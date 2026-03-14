package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunRenameTrunkAndCancel(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runRename(nil, nil); err == nil {
		t.Fatalf("expected trunk rename error")
	}

	repo.createBranch(t, "feat-rename-cancel", "main")
	if err := repo.repo.CheckoutBranch("feat-rename-cancel"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runRename(nil, nil); err != nil {
			t.Fatalf("runRename cancel failed: %v", err)
		}
	})
}

func TestRunRenameTrackedBranchUpdatesChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.createBranch(t, "feat-child", "feat-parent")
	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	if err := runRename(nil, []string{"feat-parent-renamed"}); err != nil {
		t.Fatalf("runRename tracked failed: %v", err)
	}

	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "feat-parent-renamed" {
		t.Fatalf("expected child parent updated, got %q", parent)
	}
}
