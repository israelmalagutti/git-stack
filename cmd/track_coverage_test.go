package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestRunTrackWithChildren(t *testing.T) {
	// Tests the "show children" path at the end of runTrack
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main -> parent -> child (child already tracked)
	repo.createBranch(t, "parent", "main")
	repo.createBranch(t, "child", "parent")

	// Untrack parent so we can re-track it
	repo.metadata.UntrackBranch("parent")
	if err := repo.metadata.Save(repo.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Track parent again - child is already tracked with parent as its parent
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runTrack(nil, []string{"parent"}); err != nil {
			t.Fatalf("runTrack with children failed: %v", err)
		}
	})
}

func TestRunTrackDescriptionCallback(t *testing.T) {
	// This tests the prompt description callback that shows "(trunk)" and "(parent: X)"
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create several branches so the description callback has material
	repo.createBranch(t, "feat-a", "main")
	repo.createBranch(t, "feat-b", "feat-a")

	// Create an untracked branch to track
	if err := repo.repo.CreateBranch("feat-new"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Track it, selecting feat-a as parent
	withAskOne(t, []interface{}{"feat-a"}, func() {
		if err := runTrack(nil, []string{"feat-new"}); err != nil {
			t.Fatalf("runTrack failed: %v", err)
		}
	})

	// Reload metadata from disk to verify (runTrack creates its own metadata instance)
	metadata, err := loadMetadata(repo.repo)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	parent, ok := metadata.GetParent("feat-new")
	if !ok || parent != "feat-a" {
		t.Fatalf("expected parent feat-a, got %q", parent)
	}
}

func TestRunTrackGenericError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create branch to track
	if err := repo.repo.CreateBranch("feat-err"); err != nil {
		t.Fatalf("create: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		// InterruptErr path returns nil (not error)
		err := runTrack(nil, []string{"feat-err"})
		if err != nil {
			t.Fatalf("runTrack cancel should return nil, got: %v", err)
		}
	})
}

func TestRunTrackSingleBranch(t *testing.T) {
	// When there's only main in the repo and we create+track a new branch,
	// the only parent option is "main"
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := repo.repo.CreateBranch("single-track"); err != nil {
		t.Fatalf("create: %v", err)
	}

	withAskOne(t, []interface{}{"main"}, func() {
		if err := runTrack(nil, []string{"single-track"}); err != nil {
			t.Fatalf("runTrack single branch failed: %v", err)
		}
	})
}
