package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestContinueRestackChildrenPaths(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "parent.txt", "parent", "parent commit")
	repo.createBranch(t, "feat-child-behind", "feat-parent")
	repo.commitFile(t, "child.txt", "child", "child commit")
	repo.createBranch(t, "feat-grandchild", "feat-child-behind")
	repo.commitFile(t, "grand.txt", "grand", "grand commit")

	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "parent2.txt"), []byte("parent2"), 0644); err != nil {
		t.Fatalf("failed to write parent file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "parent2.txt"); err != nil {
		t.Fatalf("failed to add parent file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "parent commit 2"); err != nil {
		t.Fatalf("failed to commit parent: %v", err)
	}

	repo.createBranch(t, "feat-child-up", "feat-parent")

	cfg, err := config.Load(repo.repo.GetConfigPath())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(repo.repo, cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	parentNode := s.GetNode("feat-parent")
	if parentNode == nil {
		t.Fatalf("expected parent node")
	}

	if err := continueRestackChildren(repo.repo, s, parentNode); err != nil {
		t.Fatalf("continueRestackChildren failed: %v", err)
	}
}

func TestRunContinueRebaseContinueFailure(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	rebaseDir := filepath.Join(repo.repo.GetGitDir(), "rebase-merge")
	if err := os.MkdirAll(rebaseDir, 0755); err != nil {
		t.Fatalf("failed to create rebase dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(rebaseDir) }()

	if err := runContinue(nil, nil); err == nil {
		t.Fatalf("expected rebase continue error")
	}
}

func TestChildNeedsRebaseError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if _, err := childNeedsRebase(repo.repo, "missing-branch", "main"); err == nil {
		t.Fatalf("expected childNeedsRebase error")
	}
}

func TestContinueRestackChildrenConflict(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-conflict", "main")
	if err := repo.repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "conflict.txt"), []byte("feat"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "conflict.txt"), []byte("main"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "main commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	cfg, err := config.Load(repo.repo.GetConfigPath())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(repo.repo, cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}
	parent := s.GetNode("main")
	if parent == nil {
		t.Fatalf("expected trunk node")
	}

	if err := continueRestackChildren(repo.repo, s, parent); err == nil {
		t.Fatalf("expected conflict error")
	}
	_, _ = repo.repo.RunGitCommand("rebase", "--abort")
}

func TestRunContinueMissingConfig(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")

	repo.createBranch(t, "feat-continue-config", "main")

	// Create conflicting commits
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := os.WriteFile(repo.dir+"/conflict.txt", []byte("main"), 0644); err != nil {
		t.Fatalf("failed to write main conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "main change"); err != nil {
		t.Fatalf("failed to commit main change: %v", err)
	}

	if err := repo.repo.CheckoutBranch("feat-continue-config"); err != nil {
		t.Fatalf("failed to checkout feat: %v", err)
	}
	if err := os.WriteFile(repo.dir+"/conflict.txt", []byte("feat"), 0644); err != nil {
		t.Fatalf("failed to write feat conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add feat conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "feat change"); err != nil {
		t.Fatalf("failed to commit feat change: %v", err)
	}

	// Start rebase to create conflict
	if _, err := repo.repo.RunGitCommand("rebase", "main"); err == nil {
		t.Fatalf("expected rebase conflict")
	}

	// Resolve conflict and stage
	if err := os.WriteFile(repo.dir+"/conflict.txt", []byte("resolved"), 0644); err != nil {
		t.Fatalf("failed to resolve conflict: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add resolved conflict: %v", err)
	}

	// Remove config to trigger error after rebase continues
	if err := os.Remove(repo.repo.GetConfigPath()); err != nil {
		t.Fatalf("failed to remove config: %v", err)
	}

	if err := runContinue(nil, nil); err == nil {
		t.Fatalf("expected runContinue missing config error")
	}
}
