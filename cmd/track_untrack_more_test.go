package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunTrackErrorsAndCancel(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Missing branch
	if err := runTrack(nil, []string{"missing-track"}); err == nil {
		t.Fatalf("expected missing branch error")
	}

	// Already tracked
	repo.createBranch(t, "feat-track", "main")
	if err := runTrack(nil, []string{"feat-track"}); err == nil {
		t.Fatalf("expected already tracked error")
	}

	// Cancel prompt
	if err := repo.repo.CreateBranch("feat-track-cancel"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runTrack(nil, []string{"feat-track-cancel"}); err != nil {
			t.Fatalf("runTrack cancel failed: %v", err)
		}
	})
}

func TestRunTrackNoParentOptions(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runTrack(nil, []string{"main"}); err == nil {
		t.Fatalf("expected no parent options error")
	}
}

func TestRunTrackCurrentBranch(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := repo.repo.CreateBranch("feat-current"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.repo.CheckoutBranch("feat-current"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	withAskOne(t, []interface{}{"main"}, func() {
		if err := runTrack(nil, nil); err != nil {
			t.Fatalf("runTrack current failed: %v", err)
		}
	})
}

func TestRunUntrackErrorsAndCancel(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	prevForce := untrackForce
	defer func() { untrackForce = prevForce }()

	// Trunk cannot be untracked
	if err := runUntrack(nil, []string{"main"}); err == nil {
		t.Fatalf("expected trunk untrack error")
	}

	// Not tracked branch returns nil
	if err := repo.repo.CreateBranch("untracked-untrack"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := runUntrack(nil, []string{"untracked-untrack"}); err != nil {
		t.Fatalf("expected untracked branch to return nil, got %v", err)
	}

	// Cancel confirmation when children exist
	repo.createBranch(t, "feat-parent", "main")
	repo.createBranch(t, "feat-child", "feat-parent")
	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	untrackForce = false
	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runUntrack(nil, []string{"feat-parent"}); err != nil {
			t.Fatalf("runUntrack cancel failed: %v", err)
		}
	})
}

func TestRunUntrackReparentsChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	prevForce := untrackForce
	defer func() { untrackForce = prevForce }()

	repo.createBranch(t, "feat-parent", "main")
	repo.createBranch(t, "feat-child", "feat-parent")
	untrackForce = true

	if err := runUntrack(nil, []string{"feat-parent"}); err != nil {
		t.Fatalf("runUntrack reparent failed: %v", err)
	}

	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Fatalf("expected child reparented to main, got %q", parent)
	}
}

func TestRunUntrackCurrentNoChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	prevForce := untrackForce
	defer func() { untrackForce = prevForce }()

	repo.createBranch(t, "feat-no-children", "main")
	if err := repo.repo.CheckoutBranch("feat-no-children"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	untrackForce = true

	if err := runUntrack(nil, nil); err != nil {
		t.Fatalf("runUntrack current failed: %v", err)
	}
}
