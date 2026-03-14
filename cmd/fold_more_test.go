package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunFoldTrunkAndUntrackedErrors(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runFold(nil, nil); err == nil {
		t.Fatalf("expected trunk fold error")
	}

	if err := repo.repo.CreateBranch("untracked-fold"); err != nil {
		t.Fatalf("failed to create untracked branch: %v", err)
	}
	if err := repo.repo.CheckoutBranch("untracked-fold"); err != nil {
		t.Fatalf("failed to checkout untracked: %v", err)
	}
	if err := runFold(nil, nil); err == nil {
		t.Fatalf("expected untracked fold error")
	}
}

func TestRunFoldDeleteBranch(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-fold-delete", "main")
	repo.commitFile(t, "fold.txt", "data", "fold commit")
	repo.createBranch(t, "feat-fold-child", "feat-fold-delete")

	if err := repo.repo.CheckoutBranch("feat-fold-delete"); err != nil {
		t.Fatalf("failed to checkout fold branch: %v", err)
	}

	prevKeep := foldKeep
	prevForce := foldForce
	defer func() {
		foldKeep = prevKeep
		foldForce = prevForce
	}()

	foldKeep = false
	foldForce = true
	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold delete failed: %v", err)
	}

	if repo.repo.BranchExists("feat-fold-delete") {
		t.Fatalf("expected folded branch to be deleted")
	}
}

func TestRunFoldNoParentError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-noparent", "main")
	if err := repo.repo.CheckoutBranch("feat-noparent"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	meta := metadata.Branches["feat-noparent"]
	meta.Parent = ""
	metadata.Branches["feat-noparent"] = meta
	if err := metadata.Save(repo.repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	if err := runFold(nil, nil); err == nil {
		t.Fatalf("expected no parent error")
	}
}
