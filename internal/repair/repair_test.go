package repair

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func setupRepairTestRepo(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-repair-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to get current dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			_ = os.Chdir(origDir)
			_ = os.RemoveAll(tmpDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644); err != nil {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to write test file: %v", err)
	}

	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
		{"branch", "-M", "main"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			_ = os.Chdir(origDir)
			_ = os.RemoveAll(tmpDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	repo, err := git.NewRepo()
	if err != nil {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("NewRepo failed: %v", err)
	}

	return repo, tmpDir, func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}
}

func newTestConfig() *config.Config {
	return config.NewConfig("main")
}

func TestDetectNoIssues(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Create a branch and track it properly
	if err := repo.CreateBranch("feat-a"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-a", "main", "")
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

func TestDetectOrphanedRef(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Create branch, track it, write ref, then delete the git branch
	if err := repo.CreateBranch("feat-orphan"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-orphan", "main", "")
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	// Delete the git branch (but leave refs and metadata)
	if err := repo.DeleteBranch("feat-orphan", true); err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Kind != OrphanedRef {
		t.Errorf("expected OrphanedRef, got %s", issues[0].Kind)
	}
	if issues[0].Branch != "feat-orphan" {
		t.Errorf("expected branch 'feat-orphan', got %s", issues[0].Branch)
	}

	// Fix it
	if err := ApplyFix(repo, metadata, newTestConfig(), issues[0]); err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}

	if metadata.IsTracked("feat-orphan") {
		t.Error("expected feat-orphan to be untracked after fix")
	}
}

func TestDetectMissingParent(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Create a branch tracked with a parent that doesn't exist
	if err := repo.CreateBranch("feat-child"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-child", "feat-gone", "")
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.Kind == MissingParent && iss.Branch == "feat-child" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected MissingParent for feat-child, got: %+v", issues)
	}

	// Fix it
	for _, iss := range issues {
		if iss.Kind == MissingParent {
			if err := ApplyFix(repo, metadata, newTestConfig(), iss); err != nil {
				t.Fatalf("ApplyFix failed: %v", err)
			}
		}
	}

	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Errorf("expected parent to be 'main' after fix, got %q", parent)
	}
}

func TestDetectCircularParent(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Create two branches with a circular parent chain
	if err := repo.CreateBranch("feat-a"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := repo.CreateBranch("feat-b"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.Branches["feat-a"] = &config.BranchMetadata{Parent: "feat-b", Tracked: true, Created: time.Now()}
	metadata.Branches["feat-b"] = &config.BranchMetadata{Parent: "feat-a", Tracked: true, Created: time.Now()}

	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.Kind == CircularParent {
			found = true
			// Fix it
			if err := ApplyFix(repo, metadata, newTestConfig(), iss); err != nil {
				t.Fatalf("ApplyFix failed: %v", err)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected CircularParent issue, got: %+v", issues)
	}

	// After fix, the cycle should be broken
	newIssues, _ := DetectIssues(repo, metadata, newTestConfig())
	for _, iss := range newIssues {
		if iss.Kind == CircularParent {
			t.Error("cycle should be broken after fix")
		}
	}
}

func TestDetectRefJSONMismatch(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	if err := repo.CreateBranch("feat-mismatch"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := repo.CreateBranch("develop"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Write ref with parent "main"
	refMeta := &config.BranchMetadata{Parent: "main", Tracked: true, Created: time.Now()}
	if err := config.WriteRefBranchMeta(repo, "feat-mismatch", refMeta); err != nil {
		t.Fatalf("WriteRefBranchMeta failed: %v", err)
	}

	// Set JSON metadata with different parent "develop"
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.Branches["feat-mismatch"] = &config.BranchMetadata{
		Parent:  "develop",
		Tracked: true,
		Created: time.Now(),
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.Kind == RefJSONMismatch && iss.Branch == "feat-mismatch" {
			found = true

			// Fix it (refs win)
			if err := ApplyFix(repo, metadata, newTestConfig(), iss); err != nil {
				t.Fatalf("ApplyFix failed: %v", err)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected RefJSONMismatch, got: %+v", issues)
	}

	// After fix, metadata should match ref (parent = "main")
	parent, _ := metadata.GetParent("feat-mismatch")
	if parent != "main" {
		t.Errorf("expected parent 'main' after fix (refs win), got %q", parent)
	}
}

func TestDetectOrphanedMetadataWithoutRef(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Track a branch in metadata only (no ref, no git branch)
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.Branches["feat-ghost"] = &config.BranchMetadata{
		Parent:  "main",
		Tracked: true,
		Created: time.Now(),
	}
	// Don't call SaveWithRefs — only JSON metadata, no ref

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.Kind == OrphanedRef && iss.Branch == "feat-ghost" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected orphaned metadata entry for feat-ghost, got: %+v", issues)
	}
}

func setupRepairTestRepoWithRemote(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	base := t.TempDir()
	remoteDir := filepath.Join(base, "remote.git")
	localDir := filepath.Join(base, "local")

	if err := exec.Command("git", "init", "--bare", remoteDir).Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}
	if err := exec.Command("git", "init", localDir).Run(); err != nil {
		t.Fatalf("failed to init local: %v", err)
	}

	for _, args := range [][]string{
		{"git", "-C", localDir, "config", "user.email", "test@test.com"},
		{"git", "-C", localDir, "config", "user.name", "Test User"},
		{"git", "-C", localDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "add", "."},
		{"git", "-C", localDir, "commit", "-m", "Initial commit"},
		{"git", "-C", localDir, "branch", "-M", "main"},
		{"git", "-C", localDir, "remote", "add", "origin", remoteDir},
		{"git", "-C", localDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Set bare repo HEAD so HasRemoteBranch works
	if err := exec.Command("git", "-C", remoteDir, "symbolic-ref", "HEAD", "refs/heads/main").Run(); err != nil {
		t.Fatalf("failed to set bare HEAD: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	return repo, localDir, func() {
		_ = os.Chdir(origDir)
	}
}

func TestDetectRemoteDeleted(t *testing.T) {
	repo, localDir, cleanup := setupRepairTestRepoWithRemote(t)
	defer cleanup()

	// Create a branch, push it, then delete the remote branch
	if err := exec.Command("git", "-C", localDir, "checkout", "-b", "feat-remote-gone").Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "feat.txt"), []byte("feature"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "add", "."},
		{"git", "-C", localDir, "commit", "-m", "feat commit"},
		{"git", "-C", localDir, "push", "-u", "origin", "feat-remote-gone"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	// Track it in metadata
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-remote-gone", "main", "")

	// Delete remote branch and prune
	if err := exec.Command("git", "-C", localDir, "push", "origin", "--delete", "feat-remote-gone").Run(); err != nil {
		t.Fatalf("failed to delete remote branch: %v", err)
	}
	if err := exec.Command("git", "-C", localDir, "fetch", "--prune").Run(); err != nil {
		t.Fatalf("failed to fetch --prune: %v", err)
	}

	// Go back to main so the branch can be deleted by ApplyFix
	if err := exec.Command("git", "-C", localDir, "checkout", "main").Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.Kind == RemoteDeleted && iss.Branch == "feat-remote-gone" {
			found = true

			// Fix it
			if err := ApplyFix(repo, metadata, newTestConfig(), iss); err != nil {
				t.Fatalf("ApplyFix failed: %v", err)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected RemoteDeleted for feat-remote-gone, got: %+v", issues)
	}

	if metadata.IsTracked("feat-remote-gone") {
		t.Error("expected feat-remote-gone to be untracked after fix")
	}
	if repo.BranchExists("feat-remote-gone") {
		t.Error("expected feat-remote-gone git branch to be deleted after fix")
	}
}

func TestDetectRemoteDeletedSkippedWithoutRemote(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	// Track a branch with no remote configured
	if err := repo.CreateBranch("feat-noremote"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-noremote", "main", "")

	issues, err := DetectIssues(repo, metadata, newTestConfig())
	if err != nil {
		t.Fatalf("DetectIssues failed: %v", err)
	}

	for _, iss := range issues {
		if iss.Kind == RemoteDeleted {
			t.Errorf("should not detect RemoteDeleted when no remote is configured, got: %+v", iss)
		}
	}
}

func TestApplyFixUnknownKind(t *testing.T) {
	repo, _, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}

	err := ApplyFix(repo, metadata, newTestConfig(), Issue{Kind: "unknown_kind"})
	if err == nil {
		t.Error("expected error for unknown issue kind")
	}
}
