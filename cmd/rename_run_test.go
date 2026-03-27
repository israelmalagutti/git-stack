package cmd

import "testing"

func TestRunRenamePaths(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-rename", "main")
	_ = repo.repo.CheckoutBranch("feat-rename")

	// Same name should be no-op
	if err := runRename(nil, []string{"feat-rename"}); err != nil {
		t.Fatalf("runRename same name failed: %v", err)
	}

	// Existing branch name error
	if err := repo.repo.CreateBranch("feat-existing"); err != nil {
		t.Fatalf("failed to create existing branch: %v", err)
	}
	if err := runRename(nil, []string{"feat-existing"}); err == nil {
		t.Fatalf("expected error for existing branch name")
	}

	// Prompt for name
	withAskOne(t, []interface{}{"feat-renamed"}, func() {
		if err := runRename(nil, nil); err != nil {
			t.Fatalf("runRename prompt failed: %v", err)
		}
	})
}
