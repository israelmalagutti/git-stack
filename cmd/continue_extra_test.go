package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestContinueHelpers(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue failed: %v", err)
	}

	// isRebaseInProgress should be false by default
	if isRebaseInProgress(repo.repo) {
		t.Fatalf("expected no rebase in progress")
	}

	// Simulate rebase in progress
	rebaseDir := filepath.Join(repo.repo.GetGitDir(), "rebase-merge")
	if err := os.MkdirAll(rebaseDir, 0755); err != nil {
		t.Fatalf("failed to create rebase dir: %v", err)
	}
	if !isRebaseInProgress(repo.repo) {
		t.Fatalf("expected rebase in progress")
	}
	if err := os.RemoveAll(rebaseDir); err != nil {
		t.Fatalf("failed to remove rebase dir: %v", err)
	}

	rebaseApplyDir := filepath.Join(repo.repo.GetGitDir(), "rebase-apply")
	if err := os.MkdirAll(rebaseApplyDir, 0755); err != nil {
		t.Fatalf("failed to create rebase apply dir: %v", err)
	}
	if !isRebaseInProgress(repo.repo) {
		t.Fatalf("expected rebase apply in progress")
	}
	if err := os.RemoveAll(rebaseApplyDir); err != nil {
		t.Fatalf("failed to remove rebase apply dir: %v", err)
	}

	// childNeedsRebase should be false for fresh branch
	repo.createBranch(t, "feat-child", "main")
	needs, err := childNeedsRebase(repo.repo, "feat-child", "main")
	if err != nil {
		t.Fatalf("childNeedsRebase failed: %v", err)
	}
	if needs {
		t.Fatalf("expected no rebase needed")
	}

	// continueRestackChildren with no rebase needed
	parent := &stack.Node{Name: "main"}
	child := &stack.Node{Name: "feat-child", Parent: parent}
	parent.Children = []*stack.Node{child}
	s := &stack.Stack{
		Trunk: parent,
		Nodes: map[string]*stack.Node{
			"main":       parent,
			"feat-child": child,
		},
	}

	if err := continueRestackChildren(repo.repo, s, parent); err != nil {
		t.Fatalf("continueRestackChildren failed: %v", err)
	}

	// Rebase needed path
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := os.WriteFile(repo.dir+"/rebase.txt", []byte("main"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "rebase.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "main update"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	s = &stack.Stack{
		Trunk: parent,
		Nodes: map[string]*stack.Node{
			"main":       parent,
			"feat-child": child,
		},
	}
	if err := continueRestackChildren(repo.repo, s, parent); err != nil {
		t.Fatalf("continueRestackChildren rebase failed: %v", err)
	}
}

func TestRunContinueSuccess(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_SEQUENCE_EDITOR", "true")

	repo.createBranch(t, "feat-continue", "main")

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

	if err := repo.repo.CheckoutBranch("feat-continue"); err != nil {
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

	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue failed: %v", err)
	}
}
