package cmd

import (
	"strings"
	"testing"
)

func TestRunSplitCommitMode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-run", "main")
	_ = repo.repo.CheckoutBranch("feat-split-run")
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

	splitByCommit = true
	splitName = "feat-base"

	logOutput, err := repo.repo.RunGitCommand("log", "--oneline", "--reverse", "main..feat-split-run")
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 commits to split")
	}

	withAskOne(t, []interface{}{[]string{lines[0]}}, func() {
		if err := runSplit(nil, nil); err != nil {
			t.Fatalf("runSplit commit failed: %v", err)
		}
	})
}

func TestRunSplitHunkMode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-hunk", "main")
	_ = repo.repo.CheckoutBranch("feat-split-hunk")
	repo.commitFile(t, "hunk.txt", "hunk", "hunk commit")

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

	splitByHunk = true
	splitName = "feat-hunk-base"

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit hunk failed: %v", err)
	}
}

func TestRunSplitFileMode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-file", "main")
	_ = repo.repo.CheckoutBranch("feat-split-file")
	repo.commitFile(t, "file.txt", "file", "file commit")

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

	splitByFile = []string{"file.txt"}
	splitName = "feat-file-base"

	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit file failed: %v", err)
	}
}

func TestRunSplitDefaultHunk(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-default", "main")
	_ = repo.repo.CheckoutBranch("feat-split-default")
	repo.commitFile(t, "default.txt", "data", "default commit")

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
	splitName = "feat-default-base"

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit default failed: %v", err)
	}
}
