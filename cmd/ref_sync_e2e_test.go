package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// setupE2ERepos creates a bare remote, "alice" clone, and "bob" clone.
// Alice's repo is gs-initialized with trunk=main.
// Bob's repo is a raw clone (no gs init yet).
func setupE2ERepos(t *testing.T) (aliceDir, bobDir string, cleanup func()) {
	t.Helper()

	// Reset the per-process refspec cache so each test is independent
	refspecConfigured = false

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	aliceDir = filepath.Join(base, "alice")
	bobDir = filepath.Join(base, "bob")

	// Create bare remote
	if err := exec.Command("git", "init", "--bare", remote).Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}
	// Set HEAD to main so clones get the right default branch
	if err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").Run(); err != nil {
		t.Fatalf("failed to set bare HEAD: %v", err)
	}

	// Create Alice's repo
	if err := exec.Command("git", "init", aliceDir).Run(); err != nil {
		t.Fatalf("failed to init alice: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", aliceDir, "config", "user.email", "alice@test.com"},
		{"git", "-C", aliceDir, "config", "user.name", "Alice"},
		{"git", "-C", aliceDir, "config", "commit.gpgsign", "false"},
		{"git", "-C", aliceDir, "remote", "add", "origin", remote},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("alice setup %v failed: %v", args, err)
		}
	}

	// Initial commit + push from Alice
	if err := os.WriteFile(filepath.Join(aliceDir, "README.md"), []byte("# Project\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", aliceDir, "add", "."},
		{"git", "-C", aliceDir, "commit", "-m", "initial commit"},
		{"git", "-C", aliceDir, "branch", "-M", "main"},
		{"git", "-C", aliceDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("alice init %v failed: %v", args, err)
		}
	}

	// Initialize gs in Alice's repo
	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir to alice: %v", err)
	}
	aliceRepo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (alice) failed: %v", err)
	}
	aliceCfg := config.NewConfig("main")
	if err := aliceCfg.Save(aliceRepo.GetConfigPath()); err != nil {
		t.Fatalf("failed to save alice config: %v", err)
	}
	_ = config.WriteRefConfig(aliceRepo, aliceCfg)
	aliceMeta := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := aliceMeta.SaveWithRefs(aliceRepo, aliceRepo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save alice metadata: %v", err)
	}
	configureGSRefspec(aliceRepo)

	// Clone for Bob
	if err := exec.Command("git", "clone", remote, bobDir).Run(); err != nil {
		t.Fatalf("failed to clone for bob: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", bobDir, "config", "user.email", "bob@test.com"},
		{"git", "-C", bobDir, "config", "user.name", "Bob"},
		{"git", "-C", bobDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("bob setup %v failed: %v", args, err)
		}
	}

	cleanup = func() {
		_ = os.Chdir(origDir)
	}

	return aliceDir, bobDir, cleanup
}

// aliceCreateBranch creates a branch with a commit in Alice's repo and tracks it.
func aliceCreateBranch(t *testing.T, aliceDir, name, parent, filename, content, message string) {
	t.Helper()

	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir to alice: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	if err := repo.CheckoutBranch(parent); err != nil {
		t.Fatalf("checkout %s failed: %v", parent, err)
	}
	if err := repo.CreateBranch(name); err != nil {
		t.Fatalf("create branch %s failed: %v", name, err)
	}
	if err := repo.CheckoutBranch(name); err != nil {
		t.Fatalf("checkout %s failed: %v", name, err)
	}

	// Create a commit so the branch diverges
	if err := os.WriteFile(filepath.Join(aliceDir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", filename); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", message); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Track in metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	parentSHA, _ := repo.GetBranchCommit(parent)
	metadata.TrackBranch(name, parent, parentSHA)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}
}

// alicePushEverything pushes branches and refs from Alice's repo.
func alicePushEverything(t *testing.T, aliceDir string, branches ...string) {
	t.Helper()

	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir to alice: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Push branches
	for _, branch := range branches {
		if _, err := repo.RunGitCommand("push", "-u", "origin", branch); err != nil {
			t.Fatalf("push branch %s failed: %v", branch, err)
		}
	}

	// Push all gs refs
	if err := config.PushAllRefs(repo, "origin"); err != nil {
		t.Fatalf("PushAllRefs failed: %v", err)
	}
	if err := config.PushConfig(repo, "origin"); err != nil {
		t.Fatalf("PushConfig failed: %v", err)
	}
}

// TestE2ERefSyncAliceToBob tests the full workflow:
// Alice creates a stack, pushes it. Bob clones, initializes gs,
// and sees the exact same stack structure.
func TestE2ERefSyncAliceToBob(t *testing.T) {
	aliceDir, bobDir, cleanup := setupE2ERepos(t)
	defer cleanup()

	// ─── Alice creates a stack: main → feat/auth → feat/auth-ui ───

	aliceCreateBranch(t, aliceDir, "feat/auth", "main",
		"auth.go", "package auth\n", "feat: add auth module")

	aliceCreateBranch(t, aliceDir, "feat/auth-ui", "feat/auth",
		"auth_ui.go", "package authui\n", "feat: add auth UI")

	// Also create an independent branch
	aliceCreateBranch(t, aliceDir, "feat/logging", "main",
		"logging.go", "package logging\n", "feat: add logging")

	// Verify Alice's local metadata is correct
	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir to alice: %v", err)
	}
	aliceRepo, _ := git.NewRepo()
	aliceMeta, err := loadMetadata(aliceRepo)
	if err != nil {
		t.Fatalf("loadMetadata (alice) failed: %v", err)
	}
	if len(aliceMeta.Branches) != 3 {
		t.Fatalf("alice should have 3 branches, got %d", len(aliceMeta.Branches))
	}

	// ─── Alice pushes everything ───

	alicePushEverything(t, aliceDir, "feat/auth", "feat/auth-ui", "feat/logging")

	// ─── Bob initializes gs ───

	if err := os.Chdir(bobDir); err != nil {
		t.Fatalf("failed to chdir to bob: %v", err)
	}

	// Bob runs gs init (with prompt mocked to select "main")
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit (bob) failed: %v", err)
		}
	})

	// ─── Bob should see Alice's full stack ───

	bobRepo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (bob) failed: %v", err)
	}

	bobMeta, err := loadMetadata(bobRepo)
	if err != nil {
		t.Fatalf("loadMetadata (bob) failed: %v", err)
	}

	// Verify all 3 branches were imported
	if len(bobMeta.Branches) != 3 {
		t.Fatalf("bob should have 3 branches, got %d: %v", len(bobMeta.Branches), branchNames(bobMeta))
	}

	// Verify parent relationships are correct
	tests := map[string]string{
		"feat/auth":    "main",
		"feat/auth-ui": "feat/auth",
		"feat/logging": "main",
	}
	for branch, expectedParent := range tests {
		parent, ok := bobMeta.GetParent(branch)
		if !ok {
			t.Errorf("bob: branch %s not tracked", branch)
			continue
		}
		if parent != expectedParent {
			t.Errorf("bob: %s parent = %q, want %q", branch, parent, expectedParent)
		}
	}

	// Verify config was imported correctly
	bobCfg, err := config.Load(bobRepo.GetConfigPath())
	if err != nil {
		t.Fatalf("Load config (bob) failed: %v", err)
	}
	if bobCfg.Trunk != "main" {
		t.Errorf("bob trunk = %q, want 'main'", bobCfg.Trunk)
	}
}

// TestE2ERefSyncBidirectional tests that Bob can add branches and Alice gets them.
func TestE2ERefSyncBidirectional(t *testing.T) {
	aliceDir, bobDir, cleanup := setupE2ERepos(t)
	defer cleanup()

	// Alice creates a branch and pushes
	aliceCreateBranch(t, aliceDir, "feat/auth", "main",
		"auth.go", "package auth\n", "feat: add auth")
	alicePushEverything(t, aliceDir, "feat/auth")

	// Bob initializes gs
	if err := os.Chdir(bobDir); err != nil {
		t.Fatalf("failed to chdir to bob: %v", err)
	}
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit (bob) failed: %v", err)
		}
	})

	// Bob creates a branch stacked on Alice's
	bobRepo, _ := git.NewRepo()
	// First fetch Alice's branch
	if _, err := bobRepo.RunGitCommand("fetch", "origin"); err != nil {
		t.Fatalf("bob fetch failed: %v", err)
	}
	if _, err := bobRepo.RunGitCommand("checkout", "-b", "feat/auth-tests", "origin/feat/auth"); err != nil {
		t.Fatalf("bob checkout feat/auth-tests failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(bobDir, "auth_test.go"), []byte("package auth_test\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := bobRepo.RunGitCommand("add", "auth_test.go"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := bobRepo.RunGitCommand("commit", "-m", "test: add auth tests"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Track it in gs
	bobMeta, _ := loadMetadata(bobRepo)
	parentSHA, _ := bobRepo.GetBranchCommit("feat/auth")
	bobMeta.TrackBranch("feat/auth-tests", "feat/auth", parentSHA)
	if err := bobMeta.SaveWithRefs(bobRepo, bobRepo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs (bob) failed: %v", err)
	}

	// Bob pushes his branch + refs
	if _, err := bobRepo.RunGitCommand("push", "-u", "origin", "feat/auth-tests"); err != nil {
		t.Fatalf("bob push branch failed: %v", err)
	}
	if err := config.PushAllRefs(bobRepo, "origin"); err != nil {
		t.Fatalf("bob PushAllRefs failed: %v", err)
	}

	// Alice fetches
	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir to alice: %v", err)
	}
	aliceRepo, _ := git.NewRepo()
	if _, err := aliceRepo.RunGitCommand("fetch", "origin"); err != nil {
		t.Fatalf("alice fetch failed: %v", err)
	}
	if err := config.FetchAllRefs(aliceRepo, "origin"); err != nil {
		t.Fatalf("alice FetchAllRefs failed: %v", err)
	}

	// Alice should see Bob's new branch
	aliceMeta, err := loadMetadata(aliceRepo)
	if err != nil {
		t.Fatalf("loadMetadata (alice after fetch) failed: %v", err)
	}

	if !aliceMeta.IsTracked("feat/auth-tests") {
		t.Error("alice should see feat/auth-tests after fetching Bob's refs")
	}

	parent, ok := aliceMeta.GetParent("feat/auth-tests")
	if !ok {
		t.Fatal("feat/auth-tests should have a parent")
	}
	if parent != "feat/auth" {
		t.Errorf("feat/auth-tests parent = %q, want 'feat/auth'", parent)
	}
}

// TestE2ERefsSurviveRebase tests that metadata refs are not affected by rebasing.
func TestE2ERefsSurviveRebase(t *testing.T) {
	aliceDir, _, cleanup := setupE2ERepos(t)
	defer cleanup()

	// Create a stack
	aliceCreateBranch(t, aliceDir, "feat/base", "main",
		"base.go", "package base\n", "feat: add base")
	aliceCreateBranch(t, aliceDir, "feat/child", "feat/base",
		"child.go", "package child\n", "feat: add child")

	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	repo, _ := git.NewRepo()

	// Record the child's commit SHA before rebase
	childSHABefore, _ := repo.GetBranchCommit("feat/child")

	// Modify main and rebase feat/base onto it
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aliceDir, "new.go"), []byte("package new\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "new.go"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "add new.go on main"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Rebase feat/base onto updated main (changes SHA)
	if err := repo.Rebase("feat/base", "main"); err != nil {
		t.Fatalf("rebase feat/base failed: %v", err)
	}
	// Rebase feat/child onto updated feat/base
	if err := repo.Rebase("feat/child", "feat/base"); err != nil {
		t.Fatalf("rebase feat/child failed: %v", err)
	}

	// Verify child's SHA changed (rebase happened)
	childSHAAfter, _ := repo.GetBranchCommit("feat/child")
	if childSHABefore == childSHAAfter {
		t.Fatal("rebase should have changed child's SHA")
	}

	// Metadata should still be correct — refs are keyed by branch name, not SHA
	meta, err := loadMetadata(repo)
	if err != nil {
		t.Fatalf("loadMetadata after rebase failed: %v", err)
	}

	parent, ok := meta.GetParent("feat/child")
	if !ok {
		t.Fatal("feat/child should still be tracked after rebase")
	}
	if parent != "feat/base" {
		t.Errorf("feat/child parent = %q, want 'feat/base'", parent)
	}

	parent, ok = meta.GetParent("feat/base")
	if !ok {
		t.Fatal("feat/base should still be tracked after rebase")
	}
	if parent != "main" {
		t.Errorf("feat/base parent = %q, want 'main'", parent)
	}
}

// TestE2EDeleteBranchCleansRefs tests that deleting a branch removes its metadata ref.
func TestE2EDeleteBranchCleansRefs(t *testing.T) {
	aliceDir, _, cleanup := setupE2ERepos(t)
	defer cleanup()

	aliceCreateBranch(t, aliceDir, "feat/temp", "main",
		"temp.go", "package temp\n", "feat: add temp")

	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	repo, _ := git.NewRepo()

	// Verify the ref exists
	if !repo.RefExists("meta/" + git.EncodeBranchRef("feat/temp")) {
		t.Fatal("ref should exist after tracking")
	}

	// Untrack and verify ref is cleaned up
	meta, _ := loadMetadata(repo)
	meta.UntrackBranch("feat/temp")
	if err := meta.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	if repo.RefExists("meta/" + git.EncodeBranchRef("feat/temp")) {
		t.Error("ref should be cleaned up after untracking")
	}
}

// TestE2ENoRemoteGraceful tests that everything works without a remote (offline mode).
func TestE2ENoRemoteGraceful(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "initial"},
		{"git", "-C", dir, "branch", "-M", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("init commit failed: %v", err)
		}
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Initialize gs (no remote)
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit failed: %v", err)
		}
	})

	repo, _ := git.NewRepo()

	// Create and track a branch — should work fine without remote
	if err := repo.CreateBranch("feat/offline"); err != nil {
		t.Fatalf("create branch failed: %v", err)
	}
	if err := repo.CheckoutBranch("feat/offline"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	meta, err := loadMetadata(repo)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	meta.TrackBranch("feat/offline", "main", "")
	if err := meta.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	// Ref should exist locally even without a remote
	if !repo.RefExists("meta/" + git.EncodeBranchRef("feat/offline")) {
		t.Error("ref should exist locally without a remote")
	}

	// loadMetadata should work
	meta2, err := loadMetadata(repo)
	if err != nil {
		t.Fatalf("loadMetadata (second) failed: %v", err)
	}
	if !meta2.IsTracked("feat/offline") {
		t.Error("feat/offline should be tracked")
	}
}

// TestE2EUpgradeFromPreRefVersion simulates a developer who initialized gs
// before refs existed, upgrades, and never runs gs init again. The refspec
// should be auto-configured on first loadMetadata call so that git fetch
// pulls refs/gs/* from the remote.
func TestE2EUpgradeFromPreRefVersion(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	refspecConfigured = false

	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	aliceDir := filepath.Join(base, "alice")
	bobDir := filepath.Join(base, "bob")

	// Create bare remote
	if err := exec.Command("git", "init", "--bare", remote).Run(); err != nil {
		t.Fatalf("init bare failed: %v", err)
	}
	if err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").Run(); err != nil {
		t.Fatalf("set bare HEAD failed: %v", err)
	}

	// Alice: full gs setup with refs
	if err := exec.Command("git", "init", aliceDir).Run(); err != nil {
		t.Fatalf("init alice failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", aliceDir, "config", "user.email", "alice@test.com"},
		{"git", "-C", aliceDir, "config", "user.name", "Alice"},
		{"git", "-C", aliceDir, "config", "commit.gpgsign", "false"},
		{"git", "-C", aliceDir, "remote", "add", "origin", remote},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("alice setup failed: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(aliceDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", aliceDir, "add", "."},
		{"git", "-C", aliceDir, "commit", "-m", "initial"},
		{"git", "-C", aliceDir, "branch", "-M", "main"},
		{"git", "-C", aliceDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("alice init failed: %v", err)
		}
	}

	// Alice: create a branch, track it, push refs
	if err := os.Chdir(aliceDir); err != nil {
		t.Fatalf("chdir alice failed: %v", err)
	}
	aliceRepo, _ := git.NewRepo()
	aliceCfg := config.NewConfig("main")
	_ = aliceCfg.Save(aliceRepo.GetConfigPath())
	_ = config.WriteRefConfig(aliceRepo, aliceCfg)
	aliceMeta := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	aliceMeta.TrackBranch("feat/new-stuff", "main", "")
	_ = aliceMeta.SaveWithRefs(aliceRepo, aliceRepo.GetMetadataPath())
	_ = config.PushAllRefs(aliceRepo, "origin")
	_ = config.PushConfig(aliceRepo, "origin")

	// Bob: simulate pre-ref gs installation
	// Clone the repo, set up gs config and JSON metadata manually (no refs, no refspec)
	if err := exec.Command("git", "clone", remote, bobDir).Run(); err != nil {
		t.Fatalf("clone for bob failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", bobDir, "config", "user.email", "bob@test.com"},
		{"git", "-C", bobDir, "config", "user.name", "Bob"},
		{"git", "-C", bobDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("bob setup failed: %v", err)
		}
	}

	if err := os.Chdir(bobDir); err != nil {
		t.Fatalf("chdir bob failed: %v", err)
	}
	bobRepo, _ := git.NewRepo()

	// Write ONLY JSON config and metadata — simulating old gs version
	bobCfg := config.NewConfig("main")
	_ = bobCfg.Save(bobRepo.GetConfigPath())
	bobMeta := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	bobMeta.TrackBranch("feat/old-branch", "main", "")
	_ = bobMeta.Save(bobRepo.GetMetadataPath()) // JSON only, no refs!

	// Verify: no refspec configured (simulating old gs)
	has, _ := bobRepo.HasRefspec("origin", "+refs/gs/*:refs/gs/*")
	if has {
		t.Fatal("bob should NOT have the gs refspec yet (simulating old version)")
	}

	// Now Bob "upgrades" gs and runs any command that calls loadMetadata
	refspecConfigured = false // Reset so loadMetadata will configure it
	meta, err := loadMetadata(bobRepo)
	if err != nil {
		t.Fatalf("loadMetadata (bob upgrade) failed: %v", err)
	}

	// loadMetadata should have auto-configured the refspec
	has, _ = bobRepo.HasRefspec("origin", "+refs/gs/*:refs/gs/*")
	if !has {
		t.Error("loadMetadata should have auto-configured the gs refspec on first call")
	}

	// Bob's old JSON branch should still be there (auto-migrated to refs)
	if !meta.IsTracked("feat/old-branch") {
		t.Error("Bob's old JSON-tracked branch should still be present")
	}

	// Now when Bob does a git fetch, refs/gs/* will be included
	if _, err := bobRepo.RunGitCommand("fetch", "origin"); err != nil {
		t.Fatalf("bob fetch failed: %v", err)
	}

	// After fetch, Alice's refs should be available locally
	// Reload metadata — should now see Alice's branch from refs
	refspecConfigured = false
	meta2, err := loadMetadata(bobRepo)
	if err != nil {
		t.Fatalf("loadMetadata (bob after fetch) failed: %v", err)
	}

	if !meta2.IsTracked("feat/new-stuff") {
		t.Error("Bob should see Alice's feat/new-stuff after fetch — the refspec auto-configure should have made this work")
	}
}

func branchNames(meta *config.Metadata) []string {
	names := make([]string, 0, len(meta.Branches))
	for name := range meta.Branches {
		names = append(names, name)
	}
	return names
}
