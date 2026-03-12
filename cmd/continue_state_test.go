package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-wrapper/internal/config"
)

func TestContinueWithSavedState(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")

	// Create a chain: main -> feat-a -> feat-b
	repo.createBranch(t, "feat-a", "main")
	repo.commitFile(t, "a.txt", "a content", "a commit")
	repo.createBranch(t, "feat-b", "feat-a")
	repo.commitFile(t, "b.txt", "b content", "b commit")

	// Add commit to main to make both behind
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit")

	// Create a conflict on feat-a
	if err := repo.repo.CheckoutBranch("feat-a"); err != nil {
		t.Fatalf("failed to checkout feat-a: %v", err)
	}

	// Start a rebase that will conflict
	if _, err := repo.repo.RunGitCommand("rebase", "main"); err != nil {
		// Expected - no conflict in this case since files don't overlap
		// Let's just create a saved state to test gw continue
		if _, abortErr := repo.repo.RunGitCommand("rebase", "--abort"); abortErr != nil {
			// Already completed, fine
		}
	}

	// Simulate saved continue state with feat-b remaining
	state := &config.ContinueState{
		RemainingBranches: []string{"feat-b"},
		OriginalBranch:    "main",
	}
	if err := state.Save(repo.repo.GetContinueStatePath()); err != nil {
		t.Fatalf("failed to save continue state: %v", err)
	}

	// Run continue - no rebase in progress, but state exists
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue with saved state failed: %v", err)
	}

	// Verify state was cleared
	loaded, _ := config.LoadContinueState(repo.repo.GetContinueStatePath())
	if loaded != nil {
		t.Error("expected continue state to be cleared after completion")
	}
}

func TestContinueNoRebaseNoState(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// No rebase in progress, no saved state
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRestackSavesStateOnConflict(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create branch with conflicting content
	repo.createBranch(t, "feat-conflict", "main")
	repo.commitFile(t, "conflict.txt", "feat content", "feat commit")

	// Add conflicting commit to main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "conflict.txt", "main content", "main commit")

	// Try to restack - should fail with conflict
	if err := repo.repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	resetRestackFlags()
	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected rebase conflict error")
	}

	// Verify continue state was saved
	state, loadErr := config.LoadContinueState(repo.repo.GetContinueStatePath())
	if loadErr != nil {
		t.Fatalf("failed to load continue state: %v", loadErr)
	}
	if state == nil {
		t.Fatal("expected continue state to be saved on conflict")
	}
	if len(state.RemainingBranches) == 0 {
		t.Error("expected remaining branches in state")
	}
	if state.OriginalBranch != "feat-conflict" {
		t.Errorf("expected original branch 'feat-conflict', got '%s'", state.OriginalBranch)
	}

	// Cleanup
	if _, err := repo.repo.RunGitCommand("rebase", "--abort"); err != nil {
		// May fail if not in rebase, that's ok
	}
	_ = config.ClearContinueState(repo.repo.GetContinueStatePath())
}

func TestRestackClearsStateOnSuccess(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-clear", "main")
	repo.commitFile(t, "feat.txt", "feat", "feat commit")

	// Add commit to main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit")

	// Pre-create a stale continue state
	state := &config.ContinueState{
		RemainingBranches: []string{"stale-branch"},
		OriginalBranch:    "main",
	}
	_ = state.Save(repo.repo.GetContinueStatePath())

	if err := repo.repo.CheckoutBranch("feat-clear"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	resetRestackFlags()
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack failed: %v", err)
	}

	// State should be cleared
	loaded, _ := config.LoadContinueState(repo.repo.GetContinueStatePath())
	if loaded != nil {
		t.Error("expected continue state to be cleared after successful restack")
	}
}

func TestContinueWithRebaseAndState(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")

	// Create chain: main -> feat-1 -> feat-2
	repo.createBranch(t, "feat-1", "main")
	repo.commitFile(t, "f1.txt", "f1 content", "f1 commit")
	repo.createBranch(t, "feat-2", "feat-1")
	repo.commitFile(t, "f2.txt", "f2 content", "f2 commit")

	// Add commit to main (non-conflicting)
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	repo.commitFile(t, "main-new.txt", "main new", "main new commit")

	// Create conflict on feat-1
	if err := repo.repo.CheckoutBranch("feat-1"); err != nil {
		t.Fatalf("checkout feat-1: %v", err)
	}

	// Start rebase - should succeed since no file overlap
	if _, err := repo.repo.RunGitCommand("rebase", "main"); err != nil {
		// Unexpected conflict, abort
		if _, abortErr := repo.repo.RunGitCommand("rebase", "--abort"); abortErr != nil {
			t.Fatalf("failed to abort: %v", abortErr)
		}
		t.Skipf("unexpected conflict, skipping test")
	}

	// Now simulate: we just finished rebasing feat-1, feat-2 remains
	state := &config.ContinueState{
		RemainingBranches: []string{"feat-2"},
		OriginalBranch:    "main",
	}
	if err := state.Save(repo.repo.GetContinueStatePath()); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// No rebase in progress, but state exists - should continue with feat-2
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue failed: %v", err)
	}

	// Verify feat-2 is now up to date with feat-1
	mergeBase, err := repo.repo.RunGitCommand("merge-base", "feat-2", "feat-1")
	if err != nil {
		t.Fatalf("merge-base failed: %v", err)
	}
	feat1SHA, _ := repo.repo.GetBranchCommit("feat-1")
	if mergeBase != feat1SHA {
		t.Error("expected feat-2 to be rebased onto feat-1 tip")
	}

	// State should be cleared
	loaded, _ := config.LoadContinueState(repo.repo.GetContinueStatePath())
	if loaded != nil {
		t.Error("expected state to be cleared")
	}
}

func TestContinueUpdatesParentRevision(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")

	repo.createBranch(t, "feat-rev-update", "main")
	repo.commitFile(t, "conflict.txt", "feat content", "feat commit")

	// Advance main with conflicting content
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	repo.commitFile(t, "conflict.txt", "main content", "main commit")

	// Start rebase to trigger conflict
	if err := repo.repo.CheckoutBranch("feat-rev-update"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("rebase", "main"); err == nil {
		t.Skipf("expected conflict but rebase succeeded")
	}

	// Resolve the conflict
	if err := os.WriteFile(filepath.Join(repo.dir, "conflict.txt"), []byte("resolved"), 0644); err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add resolved: %v", err)
	}

	// Run continue — should finish rebase and update parentRevision
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue: %v", err)
	}

	// Check that parentRevision was updated
	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}

	mainSHA, _ := repo.repo.GetBranchCommit("main")
	rev := metadata.GetParentRevision("feat-rev-update")
	if rev != mainSHA {
		t.Errorf("expected parentRevision '%s', got '%s'", mainSHA, rev)
	}
}

func TestGetContinueStatePath(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	path := repo.repo.GetContinueStatePath()
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	// Should be in git common dir
	expected := filepath.Join(repo.repo.GetCommonDir(), ".gw_continue_state")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}
}

func TestRebaseOnto(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main -> feat-base (commit A) -> feat-top (commit B)
	repo.createBranch(t, "feat-base", "main")
	repo.commitFile(t, "base.txt", "base", "base commit")
	baseSHA, _ := repo.repo.GetBranchCommit("feat-base")

	repo.createBranch(t, "feat-top", "feat-base")
	repo.commitFile(t, "top.txt", "top", "top commit")

	// Advance feat-base
	if err := repo.repo.CheckoutBranch("feat-base"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "base2.txt", "base2", "base commit 2")

	// RebaseOnto: replay feat-top's commits (after baseSHA) onto feat-base tip
	if err := repo.repo.RebaseOnto("feat-top", "feat-base", baseSHA); err != nil {
		t.Fatalf("RebaseOnto failed: %v", err)
	}

	// Verify feat-top is now on top of feat-base
	mergeBase, err := repo.repo.RunGitCommand("merge-base", "feat-top", "feat-base")
	if err != nil {
		t.Fatalf("merge-base: %v", err)
	}
	newBaseSHA, _ := repo.repo.GetBranchCommit("feat-base")
	if mergeBase != newBaseSHA {
		t.Error("expected feat-top to be on top of feat-base after RebaseOnto")
	}

	// Verify top.txt exists (commit was replayed)
	if err := repo.repo.CheckoutBranch("feat-top"); err != nil {
		t.Fatalf("checkout feat-top: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.dir, "top.txt")); os.IsNotExist(err) {
		t.Error("expected top.txt to exist after RebaseOnto")
	}
}
