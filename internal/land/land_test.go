package land

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func setupLandTestRepo(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-land-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to get cwd: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to chdir: %v", err)
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
		t.Fatalf("write failed: %v", err)
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

// createBranchWithCommit creates a branch off current and adds a commit.
func createBranchWithCommit(t *testing.T, repo *git.Repo, name, file, content string) {
	t.Helper()
	if err := repo.CreateBranch(name); err != nil {
		t.Fatalf("CreateBranch %s failed: %v", name, err)
	}
	if err := repo.CheckoutBranch(name); err != nil {
		t.Fatalf("CheckoutBranch %s failed: %v", name, err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Add "+file); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func TestLandMergedBranch(t *testing.T) {
	repo, dir, cleanup := setupLandTestRepo(t)
	defer cleanup()

	// Create a feature branch
	createBranchWithCommit(t, repo, "feat-a", filepath.Join(dir, "a.txt"), "a")

	// Merge it into main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main failed: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-a"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Set up metadata
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-a", "main", "")

	// Land it (no provider — merge-base check)
	result, err := Branch(repo, metadata, nil, "main", "main", "", Opts{
		Branch:         "feat-a",
		NoDeleteRemote: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.Landed != "feat-a" {
		t.Errorf("expected landed 'feat-a', got %q", result.Landed)
	}
	if result.NewParent != "main" {
		t.Errorf("expected parent 'main', got %q", result.NewParent)
	}
	if metadata.IsTracked("feat-a") {
		t.Error("expected feat-a to be untracked after landing")
	}
	if repo.BranchExists("feat-a") {
		t.Error("expected feat-a to be deleted after landing")
	}
}

func TestLandUnmergedBranchFails(t *testing.T) {
	repo, dir, cleanup := setupLandTestRepo(t)
	defer cleanup()

	// Create feature branch but don't merge
	createBranchWithCommit(t, repo, "feat-unmerged", filepath.Join(dir, "u.txt"), "u")
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-unmerged", "main", "")

	_, err := Branch(repo, metadata, nil, "main", "main", "", Opts{
		Branch:         "feat-unmerged",
		NoDeleteRemote: true,
	})
	if err == nil {
		t.Fatal("expected error for unmerged branch")
	}
}

func TestLandWithChildrenReparents(t *testing.T) {
	repo, dir, cleanup := setupLandTestRepo(t)
	defer cleanup()

	// Create parent and child branches
	createBranchWithCommit(t, repo, "feat-parent", filepath.Join(dir, "p.txt"), "p")
	createBranchWithCommit(t, repo, "feat-child", filepath.Join(dir, "c.txt"), "c")

	// Merge parent into main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-parent"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Set up metadata
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-parent", "main", "")
	metadata.TrackBranch("feat-child", "feat-parent", "")

	result, err := Branch(repo, metadata, nil, "main", "main", "", Opts{
		Branch:         "feat-parent",
		NoDeleteRemote: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if len(result.ReparentedChildren) != 1 || result.ReparentedChildren[0] != "feat-child" {
		t.Errorf("expected [feat-child] reparented, got %v", result.ReparentedChildren)
	}

	// Verify child's new parent
	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Errorf("expected child parent 'main', got %q", parent)
	}
}

func TestLandCurrentBranchChecksOutParent(t *testing.T) {
	repo, dir, cleanup := setupLandTestRepo(t)
	defer cleanup()

	createBranchWithCommit(t, repo, "feat-current", filepath.Join(dir, "cur.txt"), "cur")

	// Merge into main from main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-current"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Switch back to feat-current (simulating "I'm on the branch I want to land")
	if err := repo.CheckoutBranch("feat-current"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-current", "main", "")

	result, err := Branch(repo, metadata, nil, "main", "feat-current", "", Opts{
		Branch:         "feat-current",
		NoDeleteRemote: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.CheckedOut != "main" {
		t.Errorf("expected checkout to 'main', got %q", result.CheckedOut)
	}

	current, _ := repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected current branch 'main', got %q", current)
	}
}

func TestFindMergedBranches(t *testing.T) {
	repo, dir, cleanup := setupLandTestRepo(t)
	defer cleanup()

	// Create two branches, merge only one
	createBranchWithCommit(t, repo, "feat-merged", filepath.Join(dir, "m.txt"), "m")
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-merged"); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	createBranchWithCommit(t, repo, "feat-open", filepath.Join(dir, "o.txt"), "o")
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-merged", "main", "")
	metadata.TrackBranch("feat-open", "main", "")

	merged, err := FindMergedBranches(repo, metadata, nil, "main")
	if err != nil {
		t.Fatalf("FindMergedBranches failed: %v", err)
	}

	if len(merged) != 1 || merged[0] != "feat-merged" {
		t.Errorf("expected [feat-merged], got %v", merged)
	}
}
