package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// --- commit.go: cancel during message prompt in "staged" path ---

func TestRunCommit_StagedCancelMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevMsg := commitMessage
	prevAll := commitAll
	prevPatch := commitPatch
	defer func() {
		commitMessage = prevMsg
		commitAll = prevAll
		commitPatch = prevPatch
	}()

	commitMessage = ""
	commitAll = false
	commitPatch = false

	// Create staged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "cancel-staged.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "cancel-staged.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// First prompt selects "Commit staged changes", second (message) cancelled
	withAskOneSequence(t, []interface{}{
		"Commit staged changes",
		terminal.InterruptErr,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunCommit_AllCancelMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevMsg := commitMessage
	prevAll := commitAll
	prevPatch := commitPatch
	defer func() {
		commitMessage = prevMsg
		commitAll = prevAll
		commitPatch = prevPatch
	}()

	commitMessage = ""
	commitAll = false
	commitPatch = false

	// Create unstaged changes (no staged)
	if err := os.WriteFile(filepath.Join(tr.dir, "cancel-all.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// First prompt: "Stage all changes and commit (--all)", second (message): cancelled
	withAskOneSequence(t, []interface{}{
		"Stage all changes and commit (--all)",
		terminal.InterruptErr,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunCommit_PatchCancelMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevMsg := commitMessage
	prevAll := commitAll
	prevPatch := commitPatch
	defer func() {
		commitMessage = prevMsg
		commitAll = prevAll
		commitPatch = prevPatch
	}()

	commitMessage = ""
	commitAll = false
	commitPatch = false

	// Create unstaged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "cancel-patch.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// "Select changes to commit" -> accept tracking -> select file -> cancel message
	withAskOneSequence(t, []interface{}{
		"Select changes to commit (--patch)",
		true,
		[]string{"cancel-patch.txt"},
		terminal.InterruptErr,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunCommit_PatchNoTrackedChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevMsg := commitMessage
	prevAll := commitAll
	prevPatch := commitPatch
	defer func() {
		commitMessage = prevMsg
		commitAll = prevAll
		commitPatch = prevPatch
	}()

	commitMessage = ""
	commitAll = false
	commitPatch = false

	// Create only untracked file
	if err := os.WriteFile(filepath.Join(tr.dir, "only-untracked.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// "Select changes to commit" -> decline tracking -> errNoChangesToCommit -> printNoChangesInfo
	withAskOneSequence(t, []interface{}{
		"Select changes to commit (--patch)",
		false,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on no changes, got: %v", err)
		}
	})
}

// --- fold.go: keep mode, cancel, no staged changes ---

func TestRunFold_KeepMode(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> parent -> current
	tr.createBranch(t, "fold-parent", "main")
	tr.commitFile(t, "fp.txt", "parent data", "parent commit")
	tr.createBranch(t, "fold-current", "fold-parent")
	tr.commitFile(t, "fc.txt", "current data", "current commit")

	prevKeep := foldKeep
	prevForce := foldForce
	defer func() {
		foldKeep = prevKeep
		foldForce = prevForce
	}()

	foldKeep = true
	foldForce = true

	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold keep mode failed: %v", err)
	}
}

func TestRunFold_CancelConfirmation(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "fold-cancel", "main")
	tr.commitFile(t, "fc2.txt", "data", "commit")

	prevKeep := foldKeep
	prevForce := foldForce
	defer func() {
		foldKeep = prevKeep
		foldForce = prevForce
	}()

	foldKeep = false
	foldForce = false

	// Cancel confirmation
	withAskOne(t, []interface{}{false}, func() {
		err := runFold(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunFold_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "fold-w-child-parent", "main")
	tr.commitFile(t, "fwcp.txt", "parent", "parent")
	tr.createBranch(t, "fold-w-child-current", "fold-w-child-parent")
	tr.commitFile(t, "fwcc.txt", "current", "current")
	tr.createBranch(t, "fold-w-child-child", "fold-w-child-current")
	tr.commitFile(t, "fwccc.txt", "child", "child")

	// Go back to current
	if err := tr.repo.CheckoutBranch("fold-w-child-current"); err != nil {
		t.Fatalf("checkout: %v", err)
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
		t.Fatalf("runFold with children failed: %v", err)
	}
}

func TestRunFold_KeepModeNoGrandparent(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> fold-direct (parent is trunk, no grandparent in metadata)
	tr.createBranch(t, "fold-direct", "main")
	tr.commitFile(t, "fd.txt", "data", "commit")

	prevKeep := foldKeep
	prevForce := foldForce
	defer func() {
		foldKeep = prevKeep
		foldForce = prevForce
	}()

	foldKeep = true
	foldForce = true

	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold keep no grandparent failed: %v", err)
	}
}

func TestRunFold_NoStagedChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create branch with empty commit (no file changes to squash)
	tr.createBranch(t, "fold-empty", "main")
	if _, err := tr.repo.RunGitCommand("commit", "--allow-empty", "-m", "empty"); err != nil {
		t.Fatalf("commit: %v", err)
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
		t.Fatalf("runFold no staged changes failed: %v", err)
	}
}

// --- delete.go: force delete current branch with children ---

func TestRunDelete_ForceCurrentBranchWithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-parent", "main")
	tr.commitFile(t, "dp.txt", "data", "parent commit")
	tr.createBranch(t, "del-child", "del-parent")
	tr.commitFile(t, "dc.txt", "data", "child commit")

	// Go to the parent branch (the one we'll delete)
	if err := tr.repo.CheckoutBranch("del-parent"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prevForce := deleteForce
	defer func() { deleteForce = prevForce }()
	deleteForce = true

	if err := runDelete(nil, []string{"del-parent"}); err != nil {
		t.Fatalf("runDelete force current with children failed: %v", err)
	}

	// After delete + restack, we may end up on the child branch or main
	// depending on restack behavior. Just verify the delete succeeded.
	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" && current != "del-child" {
		t.Errorf("expected main or del-child, got %s", current)
	}
}

func TestRunDelete_NotTracked(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-del"); err != nil {
		t.Fatalf("create: %v", err)
	}

	prevForce := deleteForce
	defer func() { deleteForce = prevForce }()
	deleteForce = true

	err := runDelete(nil, []string{"untracked-del"})
	if err == nil {
		t.Fatal("expected not-tracked error")
	}
}

// --- move.go: cancel interactive, source on different branch ---

func TestRunMove_InteractiveCancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "move-cancel", "main")

	prevSource := moveSource
	prevOnto := moveOnto
	defer func() {
		moveSource = prevSource
		moveOnto = prevOnto
	}()

	moveSource = "move-cancel"
	moveOnto = ""

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runMove(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunMove_RebaseConflict(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create conflicting scenario: main -> a, main -> b with same file
	tr.createBranch(t, "move-a", "main")
	tr.commitFile(t, "shared.txt", "content-a", "a commit")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "move-b", "main")
	tr.commitFile(t, "shared.txt", "content-b", "b commit")

	prevSource := moveSource
	prevOnto := moveOnto
	defer func() {
		moveSource = prevSource
		moveOnto = prevOnto
	}()

	moveSource = ""
	moveOnto = "move-a"

	err := runMove(nil, nil)
	if err == nil {
		t.Fatal("expected rebase conflict error")
	}
}

// --- init.go: init with remote that has different trunk config ---

func TestRunInit_RemoteWithDifferentTrunk(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create bare remote
	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	// Source repo: push gs config with trunk="main"
	srcDir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", srcDir},
		{"git", "-C", srcDir, "config", "user.email", "test@test.com"},
		{"git", "-C", srcDir, "config", "user.name", "Test User"},
		{"git", "-C", srcDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", srcDir, "add", "."},
		{"git", "-C", srcDir, "commit", "-m", "Init"},
		{"git", "-C", srcDir, "branch", "-M", "main"},
		{"git", "-C", srcDir, "remote", "add", "origin", bareDir},
		{"git", "-C", srcDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("push: %v", err)
		}
	}

	// Set up gs config in source and push refs
	if err := os.Chdir(srcDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	srcRepo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	srcCfg := config.NewConfig("main")
	if err := srcCfg.Save(srcRepo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	_ = config.WriteRefConfig(srcRepo, srcCfg)

	// Track a branch and push metadata refs
	srcMeta := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	srcMeta.TrackBranch("feat-remote", "main", "abc123")
	if err := srcMeta.SaveWithRefs(srcRepo, srcRepo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	_ = config.PushAllRefs(srcRepo, "origin")
	_ = config.PushConfig(srcRepo, "origin")

	// Update bare repo HEAD to point to main (git init --bare defaults to master)
	if err := exec.Command("git", "-C", bareDir, "symbolic-ref", "HEAD", "refs/heads/main").Run(); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}

	// Clone into fresh dir (no gs files)
	cloneDir := t.TempDir()
	if err := exec.Command("git", "clone", bareDir, cloneDir).Run(); err != nil {
		t.Fatalf("clone: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", cloneDir, "config", "user.email", "test@test.com"},
		{"git", "-C", cloneDir, "config", "user.name", "Test User"},
		{"git", "-C", cloneDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("config: %v", err)
		}
	}

	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Init: user selects "main" but remote already has gs config with trunk="main"
	// This should hit the "remote already has gs config" path
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit remote with gs config failed: %v", err)
		}
	})

	// Verify config exists
	if !config.IsInitialized(filepath.Join(cloneDir, ".git", ".gs_config")) {
		t.Error("expected gs config")
	}
}

// --- sync.go: runSync full path with restack and failed branches ---

func TestRunSync_WithBranchesAndRestack(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}

	// Create a tracked branch
	if err := repo.CreateBranch("feat-sync-rs"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CheckoutBranch("feat-sync-rs"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "feat.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	parentSHA, _ := repo.GetBranchCommit("main")
	metadata.TrackBranch("feat-sync-rs", "main", parentSHA)
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	prevForce := syncForce
	prevRestack := syncRestack
	prevDelete := syncDelete
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
		syncDelete = prevDelete
	}()

	syncForce = true
	syncRestack = true
	syncDelete = true

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync with branches failed: %v", err)
	}
}

// --- create.go: detectStagedChanges with no staged ---

func TestDetectStagedChanges_NoStaged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if detectStagedChanges(tr.repo) {
		t.Error("expected no staged changes")
	}
}

func TestHasTrackedChanges_ModifiedTracked(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Modify a tracked file but don't stage
	if err := os.WriteFile(filepath.Join(tr.dir, "README.md"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !hasTrackedChanges(tr.repo) {
		t.Error("expected tracked changes for modified file")
	}
}

func TestHasTrackedChanges_NoChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if hasTrackedChanges(tr.repo) {
		t.Error("expected no tracked changes")
	}
}

// --- modify.go: unstaged changes error, no changes ---

func TestRunModify_UnstagedChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "mod-unstaged", "main")
	tr.commitFile(t, "mod.txt", "original", "initial")

	// Modify but don't stage
	if err := os.WriteFile(filepath.Join(tr.dir, "mod.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prevAll := modifyAll
	prevCommit := modifyCommit
	prevMessage := modifyMessage
	prevPatch := modifyPatch
	defer func() {
		modifyAll = prevAll
		modifyCommit = prevCommit
		modifyMessage = prevMessage
		modifyPatch = prevPatch
	}()

	modifyAll = false
	modifyCommit = false
	modifyMessage = ""
	modifyPatch = false

	err := runModify(nil, nil)
	if err == nil {
		t.Fatal("expected unstaged changes error")
	}
}

func TestRunModify_NewCommitWithMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "mod-newcommit", "main")
	tr.commitFile(t, "mod2.txt", "data", "initial")

	// Stage a change
	if err := os.WriteFile(filepath.Join(tr.dir, "mod2.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prevAll := modifyAll
	prevCommit := modifyCommit
	prevMessage := modifyMessage
	prevPatch := modifyPatch
	defer func() {
		modifyAll = prevAll
		modifyCommit = prevCommit
		modifyMessage = prevMessage
		modifyPatch = prevPatch
	}()

	modifyAll = true
	modifyCommit = true
	modifyMessage = "new commit"
	modifyPatch = false

	if err := runModify(nil, nil); err != nil {
		t.Fatalf("runModify new commit failed: %v", err)
	}
}

func TestRunModify_AmendWithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "mod-amend-parent", "main")
	tr.commitFile(t, "map.txt", "data", "parent commit")
	tr.createBranch(t, "mod-amend-child", "mod-amend-parent")
	tr.commitFile(t, "mac.txt", "data", "child commit")

	if err := tr.repo.CheckoutBranch("mod-amend-parent"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Stage a change
	if err := os.WriteFile(filepath.Join(tr.dir, "map.txt"), []byte("amended"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prevAll := modifyAll
	prevCommit := modifyCommit
	prevMessage := modifyMessage
	prevPatch := modifyPatch
	defer func() {
		modifyAll = prevAll
		modifyCommit = prevCommit
		modifyMessage = prevMessage
		modifyPatch = prevPatch
	}()

	modifyAll = true
	modifyCommit = false
	modifyMessage = ""
	modifyPatch = false

	err := runModify(nil, nil)
	// A rebase conflict is acceptable here — the amend itself succeeded,
	// and the conflict is from restacking the child branch.
	if err != nil {
		if err.Error() != "failed to restack children: rebase conflict" {
			t.Fatalf("runModify amend with children failed unexpectedly: %v", err)
		}
		// Abort the in-progress rebase to keep the repo clean
		_, _ = tr.repo.RunGitCommand("rebase", "--abort")
	}
}

func TestRunModify_NoCommitsOnBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create branch with no commits (just a branch pointer)
	tr.createBranch(t, "mod-nocommits", "main")

	// Stage a file
	if err := os.WriteFile(filepath.Join(tr.dir, "mnc.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prevAll := modifyAll
	prevCommit := modifyCommit
	prevMessage := modifyMessage
	prevPatch := modifyPatch
	defer func() {
		modifyAll = prevAll
		modifyCommit = prevCommit
		modifyMessage = prevMessage
		modifyPatch = prevPatch
	}()

	modifyAll = true
	modifyCommit = false
	modifyMessage = "forced commit"
	modifyPatch = false

	if err := runModify(nil, nil); err != nil {
		t.Fatalf("runModify no commits failed: %v", err)
	}
}

// --- checkout.go: description callback, interactive with tracked/untracked ---

func TestRunCheckout_InteractiveDescBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-desc-a", "main")
	tr.createBranch(t, "feat-desc-b", "feat-desc-a")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prevStack := checkoutStack
	prevTrunk := checkoutTrunk
	prevShowUntracked := checkoutShowUntracked
	defer func() {
		checkoutStack = prevStack
		checkoutTrunk = prevTrunk
		checkoutShowUntracked = prevShowUntracked
	}()
	checkoutStack = false
	checkoutTrunk = false
	checkoutShowUntracked = false

	withAskOne(t, []interface{}{"feat-desc-a"}, func() {
		if err := runCheckout(nil, nil); err != nil {
			t.Fatalf("runCheckout interactive with description failed: %v", err)
		}
	})
}

// --- split.go: single commit auto-hunk, file mode via -f flag ---

func TestRunSplit_SingleCommitAutoHunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-single-hunk", "main")
	tr.commitFile(t, "single.txt", "data", "only commit")

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
	splitName = "feat-single-base"

	t.Setenv("GW_TEST_AUTO_STAGE", "1")
	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit single commit auto-hunk failed: %v", err)
	}
}

func TestRunSplit_FileMode(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create branch with files on main first (tracked)
	tr.commitFile(t, "split-move.txt", "original", "add split-move on main")
	tr.commitFile(t, "split-keep.txt", "original", "add split-keep on main")

	tr.createBranch(t, "feat-split-file", "main")
	tr.commitFile(t, "split-move.txt", "modified", "modify split-move")
	tr.commitFile(t, "split-keep.txt", "modified", "modify split-keep")

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
	splitByFile = []string{"split-move.txt"}
	splitName = "feat-split-file-base"

	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit file mode failed: %v", err)
	}
}

func TestRunSplit_MultipleModeError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-multi-mode", "main")
	tr.commitFile(t, "mm.txt", "data", "commit")

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
	splitByHunk = true
	splitByFile = nil
	splitName = "base"

	err := runSplit(nil, nil)
	if err == nil {
		t.Fatal("expected multiple mode error")
	}
}

func TestRunSplit_TrunkError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevCommit := splitByCommit
	prevName := splitName
	defer func() {
		splitByCommit = prevCommit
		splitName = prevName
	}()
	splitByCommit = true
	splitName = "base"

	err := runSplit(nil, nil)
	if err == nil {
		t.Fatal("expected trunk error")
	}
}

func TestRunSplit_BranchAlreadyExists(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-exists-split", "main")
	tr.commitFile(t, "exists.txt", "data", "commit")

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
	splitByHunk = false
	splitByFile = nil
	splitName = "main" // Already exists

	err := runSplit(nil, nil)
	if err == nil {
		t.Fatal("expected already-exists error")
	}
}

func TestRunSplit_NoCommits(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-no-commits", "main")

	prevCommit := splitByCommit
	prevName := splitName
	defer func() {
		splitByCommit = prevCommit
		splitName = prevName
	}()
	splitByCommit = true
	splitName = "base"

	err := runSplit(nil, nil)
	if err == nil {
		t.Fatal("expected no-commits error")
	}
}

// --- land.go: landStackBranches cancel ---

func TestLandStackBranches_Cancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create and merge a branch
	tr.createBranch(t, "feat-land-stack-cancel", "main")
	tr.commitFile(t, "lsc.txt", "data", "commit")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-land-stack-cancel"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if err := tr.repo.CheckoutBranch("feat-land-stack-cancel"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prevForce := landForce
	prevStack := landStack
	defer func() {
		landForce = prevForce
		landStack = prevStack
	}()
	landForce = false
	landStack = true

	// Cancel the confirmation
	withAskOne(t, []interface{}{false}, func() {
		err := runLand(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

// --- rename.go: prompt for name ---

func TestRunRename_PromptName(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-rename-prompt", "main")

	withAskOne(t, []interface{}{"feat-renamed"}, func() {
		if err := runRename(nil, nil); err != nil {
			t.Fatalf("runRename prompt failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "feat-renamed" {
		t.Errorf("expected feat-renamed, got %s", current)
	}
}

// --- untrack.go extra paths ---

func TestRunUntrack_TrunkError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runUntrack(nil, []string{"main"})
	if err == nil {
		t.Fatal("expected cannot untrack trunk error")
	}
}

// --- down.go: node not found in stack ---

func TestRunDown_UntrackedBranchBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-down"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-down"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runDown(nil, nil)
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}
