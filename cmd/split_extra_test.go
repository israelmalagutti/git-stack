package cmd

import (
	"strings"
	"testing"
)

func TestSplitByCommitMode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split", "main")
	_ = repo.repo.CheckoutBranch("feat-split")
	repo.commitFile(t, "a.txt", "a", "commit a")
	repo.commitFile(t, "b.txt", "b", "commit b")

	output, err := repo.repo.RunGitCommand("log", "--oneline", "--reverse", "main..feat-split")
	if err != nil {
		t.Fatalf("failed to get commits: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 commits")
	}

	withAskOne(t, []interface{}{[]string{lines[0]}}, func() {
		if err := splitByCommitMode(repo.repo, repo.cfg, repo.metadata, "feat-split", "main", "feat-parent", len(lines)); err != nil {
			t.Fatalf("splitByCommitMode failed: %v", err)
		}
	})
}

func TestSplitPromptHelpers(t *testing.T) {
	withAskOne(t, []interface{}{"By commit - split along commit boundaries"}, func() {
		mode, err := promptSplitMode()
		if err != nil || mode != "commit" {
			t.Fatalf("promptSplitMode returned %q, %v", mode, err)
		}
	})
	withAskOne(t, []interface{}{"By hunk - interactively select changes"}, func() {
		mode, err := promptSplitMode()
		if err != nil || mode != "hunk" {
			t.Fatalf("promptSplitMode returned %q, %v", mode, err)
		}
	})

	withAskOne(t, []interface{}{"new-branch"}, func() {
		name, err := promptBranchName("current")
		if err != nil || name != "new-branch" {
			t.Fatalf("promptBranchName returned %q, %v", name, err)
		}
	})
}

func TestSplitByFileModeNoMatch(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-split-nomatch", "main")
	_ = repo.repo.CheckoutBranch("feat-split-nomatch")
	repo.commitFile(t, "match.txt", "data", "match commit")

	err := splitByFileMode(repo.repo, repo.cfg, repo.metadata, "feat-split-nomatch", "main", "feat-split-base", []string{"nope*.txt"})
	if err == nil {
		t.Fatalf("expected splitByFileMode to fail when no files match")
	}
}
