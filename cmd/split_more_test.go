package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestSplitByCommitModeSelectionErrors(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-errors", "main")
	_ = repo.repo.CheckoutBranch("feat-split-errors")
	repo.commitFile(t, "a.txt", "a", "commit a")
	repo.commitFile(t, "b.txt", "b", "commit b")

	output, err := repo.repo.RunGitCommand("log", "--oneline", "--reverse", "main..feat-split-errors")
	if err != nil {
		t.Fatalf("failed to get commits: %v", err)
	}
	commits := strings.Split(strings.TrimSpace(output), "\n")
	if len(commits) < 2 {
		t.Fatalf("expected at least 2 commits")
	}

	withAskOne(t, []interface{}{[]string{}}, func() {
		if err := splitByCommitMode(repo.repo, repo.cfg, repo.metadata, "feat-split-errors", "main", "feat-parent-none", len(commits)); err == nil {
			t.Fatalf("expected no selection error")
		}
	})

	withAskOne(t, []interface{}{commits}, func() {
		if err := splitByCommitMode(repo.repo, repo.cfg, repo.metadata, "feat-split-errors", "main", "feat-parent-all", len(commits)); err == nil {
			t.Fatalf("expected all selected error")
		}
	})
}

func TestPromptBranchNameCancelled(t *testing.T) {
	withAskOneError(t, terminal.InterruptErr, func() {
		if _, err := promptBranchName("current"); err == nil {
			t.Fatalf("expected promptBranchName cancel error")
		}
	})
}

func TestPromptSplitModeCancelled(t *testing.T) {
	withAskOneError(t, terminal.InterruptErr, func() {
		if _, err := promptSplitMode(); err == nil {
			t.Fatalf("expected promptSplitMode cancel error")
		}
	})
}

func TestSplitByHunkModeNoChanges(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-empty", "main")
	if err := repo.repo.CheckoutBranch("feat-split-empty"); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "--allow-empty", "-m", "empty"); err != nil {
		t.Fatalf("failed to create empty commit: %v", err)
	}

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := splitByHunkMode(repo.repo, repo.cfg, repo.metadata, "feat-split-empty", "main", "feat-split-base"); err == nil {
		t.Fatalf("expected splitByHunkMode no changes error")
	}
}

func TestRunSplitPromptCommitMode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-prompt", "main")
	_ = repo.repo.CheckoutBranch("feat-split-prompt")
	repo.commitFile(t, "a.txt", "a", "commit a")
	repo.commitFile(t, "b.txt", "b", "commit b")

	prevCommit := splitByCommit
	prevHunk := splitByHunk
	prevFile := splitByFile
	prevName := splitName
	defer func() {
		splitByCommit = prevCommit
		splitByHunk = prevHunk
		splitByFile = prevFile
		splitName = prevName
	}()

	splitByCommit = false
	splitByHunk = false
	splitByFile = nil
	splitName = ""

	output, err := repo.repo.RunGitCommand("log", "--oneline", "--reverse", "main..feat-split-prompt")
	if err != nil {
		t.Fatalf("failed to get commits: %v", err)
	}
	commits := strings.Split(strings.TrimSpace(output), "\n")
	if len(commits) < 2 {
		t.Fatalf("expected commits")
	}

	withAskOne(t, []interface{}{
		"By commit - split along commit boundaries",
		"feat-prompt-base",
		[]string{commits[0]},
	}, func() {
		if err := runSplit(nil, nil); err != nil {
			t.Fatalf("runSplit prompt commit failed: %v", err)
		}
	})
}

func TestRunSplitBranchExists(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-exists", "main")
	_ = repo.repo.CheckoutBranch("feat-split-exists")
	repo.commitFile(t, "a.txt", "a", "commit a")
	repo.commitFile(t, "b.txt", "b", "commit b")

	prevCommit := splitByCommit
	prevName := splitName
	defer func() {
		splitByCommit = prevCommit
		splitName = prevName
	}()

	splitByCommit = true
	splitName = "main"
	if err := runSplit(nil, nil); err == nil {
		t.Fatalf("expected branch exists error")
	}
}

func TestSplitByCommitModeCancelled(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-cancel", "main")
	_ = repo.repo.CheckoutBranch("feat-split-cancel")
	repo.commitFile(t, "a.txt", "a", "commit a")
	repo.commitFile(t, "b.txt", "b", "commit b")

	output, err := repo.repo.RunGitCommand("log", "--oneline", "--reverse", "main..feat-split-cancel")
	if err != nil {
		t.Fatalf("failed to get commits: %v", err)
	}
	commits := strings.Split(strings.TrimSpace(output), "\n")
	if len(commits) < 2 {
		t.Fatalf("expected commits")
	}

	withAskOneSequence(t, []interface{}{terminal.InterruptErr}, func() {
		if err := splitByCommitMode(repo.repo, repo.cfg, repo.metadata, "feat-split-cancel", "main", "feat-parent", len(commits)); err == nil {
			t.Fatalf("expected splitByCommitMode cancel error")
		}
	})
}

func TestSplitByFileModeErrors(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Invalid parent branch should fail to create new branch
	if err := splitByFileMode(repo.repo, repo.cfg, repo.metadata, "main", "missing", "feat-split-base", []string{"*.txt"}); err == nil {
		t.Fatalf("expected splitByFileMode parent error")
	}

	repo.createBranch(t, "feat-split-file-error", "main")
	_ = repo.repo.CheckoutBranch("feat-split-file-error")
	repo.commitFile(t, "file.txt", "data", "file commit")

	// Invalid pattern should fail to add
	if err := splitByFileMode(repo.repo, repo.cfg, repo.metadata, "feat-split-file-error", "main", "feat-split-base2", []string{"["}); err == nil {
		t.Fatalf("expected splitByFileMode add error")
	}
}

func TestSplitByHunkModeInvalidParent(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := splitByHunkMode(repo.repo, repo.cfg, repo.metadata, "main", "missing", "feat-split-base"); err == nil {
		t.Fatalf("expected splitByHunkMode parent error")
	}
}

func TestSplitByFileModeRestacksChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "keep.txt", "data", "parent commit")
	repo.createBranch(t, "feat-child", "feat-parent")
	repo.commitFile(t, "child.txt", "data", "child commit")

	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	repo.commitFile(t, "move.txt", "data", "move commit")

	if err := splitByFileMode(repo.repo, repo.cfg, repo.metadata, "feat-parent", "main", "feat-base-file", []string{"move.txt", "keep.txt"}); err != nil {
		t.Fatalf("splitByFileMode restack failed: %v", err)
	}
}

func TestSplitByHunkModeRestacksChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "keep.txt", "data", "parent commit")
	repo.createBranch(t, "feat-child", "feat-parent")
	repo.commitFile(t, "child.txt", "data", "child commit")

	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "hunk.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "hunk.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "hunk commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := splitByHunkMode(repo.repo, repo.cfg, repo.metadata, "feat-parent", "main", "feat-base-hunk"); err != nil {
		t.Fatalf("splitByHunkMode restack failed: %v", err)
	}
}

func TestSplitByFileModeCheckoutError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "keep.txt", "data", "parent commit")

	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	repo.commitFile(t, "move.txt", "data", "move commit")

	if err := splitByFileMode(repo.repo, repo.cfg, repo.metadata, "feat-parent", "main", "feat-base-file-error", []string{"move.txt"}); err == nil {
		t.Fatalf("expected splitByFileMode checkout error")
	}
}

func TestSplitByHunkModeUpdateParentError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := repo.repo.CreateBranch("feat-untracked"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.repo.CheckoutBranch("feat-untracked"); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}
	repo.commitFile(t, "hunk.txt", "data", "hunk commit")

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := splitByHunkMode(repo.repo, repo.cfg, repo.metadata, "feat-untracked", "main", "feat-base-hunk-error"); err == nil {
		t.Fatalf("expected splitByHunkMode update parent error")
	}
}
