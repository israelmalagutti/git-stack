package submit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/provider"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name string

	// Return values
	createPRResult   *provider.PRResult
	createPRErr      error
	getPRStatusResult *provider.PRStatus
	getPRStatusErr   error
	findExistingResult *provider.PRResult
	findExistingErr  error
	updatePRBaseErr  error

	// Call tracking
	createPRCalls    []provider.PRCreateOpts
	getPRStatusCalls []int
	findExistingCalls []string
	updatePRBaseCalls []struct {
		Number  int
		NewBase string
	}
}

func (m *mockProvider) Name() string            { return m.name }
func (m *mockProvider) CLIAvailable() bool       { return true }
func (m *mockProvider) CLIAuthenticated() bool   { return true }

func (m *mockProvider) CreatePR(opts provider.PRCreateOpts) (*provider.PRResult, error) {
	m.createPRCalls = append(m.createPRCalls, opts)
	return m.createPRResult, m.createPRErr
}

func (m *mockProvider) UpdatePR(number int, opts provider.PRUpdateOpts) error {
	return nil
}

func (m *mockProvider) GetPRStatus(number int) (*provider.PRStatus, error) {
	m.getPRStatusCalls = append(m.getPRStatusCalls, number)
	return m.getPRStatusResult, m.getPRStatusErr
}

func (m *mockProvider) MergePR(number int, opts provider.PRMergeOpts) error {
	return nil
}

func (m *mockProvider) UpdatePRBase(number int, newBase string) error {
	m.updatePRBaseCalls = append(m.updatePRBaseCalls, struct {
		Number  int
		NewBase string
	}{number, newBase})
	return m.updatePRBaseErr
}

func (m *mockProvider) FindExistingPR(head string) (*provider.PRResult, error) {
	m.findExistingCalls = append(m.findExistingCalls, head)
	return m.findExistingResult, m.findExistingErr
}

// setupTestRepo creates a git repo with an initial commit on main.
func setupTestRepo(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-submit-test-*")
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

// setupTestRepoWithRemote creates a repo with a bare remote for push tests.
func setupTestRepoWithRemote(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-submit-remote-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to get cwd: %v", err)
	}

	bareDir := filepath.Join(tmpDir, "bare.git")
	workDir := filepath.Join(tmpDir, "work")

	// Create bare remote
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("mkdir bare failed: %v", err)
	}
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("git init --bare failed: %v", err)
	}

	// Clone bare into work dir
	if err := exec.Command("git", "clone", bareDir, workDir).Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("git clone failed: %v", err)
	}

	if err := os.Chdir(workDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("chdir failed: %v", err)
	}

	for _, args := range [][]string{
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

	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test"), 0644); err != nil {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("write failed: %v", err)
	}

	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
		{"branch", "-M", "main"},
		{"push", "-u", "origin", "main"},
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

	return repo, workDir, func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}
}

func TestHumanizeBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat/auth-ui", "Auth ui"},
		{"fix/login-bug", "Login bug"},
		{"auth", "Auth"},
		{"my-feature", "My feature"},
		{"feat/a", "A"},
		{"release/v1.0/hotfix", "Hotfix"},
	}

	for _, tt := range tests {
		got := HumanizeBranchName(tt.input)
		if got != tt.want {
			t.Errorf("HumanizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Branch() tests ---

func TestBranchCreateNewPR(t *testing.T) {
	repo, dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a feature branch with a commit
	if err := exec.Command("git", "checkout", "-b", "feat-new").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Add new feature"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-new", "main", "")

	prov := &mockProvider{
		name: "github",
		findExistingResult: nil, // no existing PR
		createPRResult: &provider.PRResult{
			Number: 42,
			URL:    "https://github.com/test/repo/pull/42",
			Action: "created",
		},
	}

	result, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-new",
		Parent: "main",
		NoPush: true, // skip push (no remote)
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.Action != "created" {
		t.Errorf("expected action 'created', got %q", result.Action)
	}
	if result.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", result.PRNumber)
	}
	if result.PRURL != "https://github.com/test/repo/pull/42" {
		t.Errorf("unexpected PR URL: %q", result.PRURL)
	}
	if result.Provider != "github" {
		t.Errorf("expected provider 'github', got %q", result.Provider)
	}
	if result.Branch != "feat-new" {
		t.Errorf("expected branch 'feat-new', got %q", result.Branch)
	}
	if result.Parent != "main" {
		t.Errorf("expected parent 'main', got %q", result.Parent)
	}

	// Verify CreatePR was called with correct args
	if len(prov.createPRCalls) != 1 {
		t.Fatalf("expected 1 CreatePR call, got %d", len(prov.createPRCalls))
	}
	call := prov.createPRCalls[0]
	if call.Base != "main" || call.Head != "feat-new" {
		t.Errorf("CreatePR called with Base=%q Head=%q, want main/feat-new", call.Base, call.Head)
	}
	if call.Title != "Add new feature" {
		t.Errorf("expected derived title 'Add new feature', got %q", call.Title)
	}

	// Verify PR metadata was stored
	pr := metadata.GetPR("feat-new")
	if pr == nil {
		t.Fatal("expected PR metadata to be stored")
	}
	if pr.Number != 42 {
		t.Errorf("expected stored PR number 42, got %d", pr.Number)
	}
	if pr.Provider != "github" {
		t.Errorf("expected stored provider 'github', got %q", pr.Provider)
	}
}

func TestBranchUpdateExistingPR(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-existing", "main", "")
	_ = metadata.SetPR("feat-existing", &config.PRInfo{Number: 10, Provider: "github"})

	prov := &mockProvider{
		name: "github",
		getPRStatusResult: &provider.PRStatus{
			Number: 10,
			State:  "open",
			URL:    "https://github.com/test/repo/pull/10",
		},
	}

	result, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-existing",
		Parent: "main",
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.Action != "updated" {
		t.Errorf("expected action 'updated', got %q", result.Action)
	}
	if result.PRNumber != 10 {
		t.Errorf("expected PR number 10, got %d", result.PRNumber)
	}

	// Verify GetPRStatus was called
	if len(prov.getPRStatusCalls) != 1 || prov.getPRStatusCalls[0] != 10 {
		t.Errorf("expected GetPRStatus(10), got calls: %v", prov.getPRStatusCalls)
	}

	// CreatePR should NOT have been called
	if len(prov.createPRCalls) != 0 {
		t.Error("CreatePR should not have been called for existing open PR")
	}
}

func TestBranchFindExistingPRViaProvider(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-found", "main", "")
	// No PR in metadata

	prov := &mockProvider{
		name: "github",
		findExistingResult: &provider.PRResult{
			Number: 55,
			URL:    "https://github.com/test/repo/pull/55",
			Action: "updated",
		},
	}

	result, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-found",
		Parent: "main",
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.PRNumber != 55 {
		t.Errorf("expected PR number 55, got %d", result.PRNumber)
	}
	if result.Action != "updated" {
		t.Errorf("expected action 'updated', got %q", result.Action)
	}

	// FindExistingPR should have been called
	if len(prov.findExistingCalls) != 1 || prov.findExistingCalls[0] != "feat-found" {
		t.Errorf("expected FindExistingPR('feat-found'), got calls: %v", prov.findExistingCalls)
	}

	// CreatePR should NOT have been called
	if len(prov.createPRCalls) != 0 {
		t.Error("CreatePR should not have been called when FindExistingPR returns a result")
	}
}

func TestBranchClosedPRCreatesNewOne(t *testing.T) {
	repo, dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create branch with commit so DerivePRTitle works
	if err := exec.Command("git", "checkout", "-b", "feat-closed").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "closed.txt"), []byte("closed"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Closed feature"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-closed", "main", "")
	_ = metadata.SetPR("feat-closed", &config.PRInfo{Number: 5, Provider: "github"})

	prov := &mockProvider{
		name: "github",
		// PR is closed
		getPRStatusResult: &provider.PRStatus{
			Number: 5,
			State:  "closed",
		},
		// No existing open PR found
		findExistingResult: nil,
		// New PR created
		createPRResult: &provider.PRResult{
			Number: 99,
			URL:    "https://github.com/test/repo/pull/99",
			Action: "created",
		},
	}

	result, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-closed",
		Parent: "main",
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if result.Action != "created" {
		t.Errorf("expected action 'created', got %q", result.Action)
	}
	if result.PRNumber != 99 {
		t.Errorf("expected new PR number 99, got %d", result.PRNumber)
	}

	// CreatePR should have been called
	if len(prov.createPRCalls) != 1 {
		t.Fatalf("expected 1 CreatePR call, got %d", len(prov.createPRCalls))
	}

	// PR metadata should be updated to new PR
	pr := metadata.GetPR("feat-closed")
	if pr == nil || pr.Number != 99 {
		t.Errorf("expected PR metadata updated to 99, got %v", pr)
	}
}

func TestBranchNoPushSkipsPush(t *testing.T) {
	repo, dir, cleanup := setupTestRepo(t)
	defer cleanup()

	if err := exec.Command("git", "checkout", "-b", "feat-nopush").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "np.txt"), []byte("np"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "No push feature"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-nopush", "main", "")

	prov := &mockProvider{
		name:           "github",
		createPRResult: &provider.PRResult{Number: 1, URL: "url", Action: "created"},
	}

	// NoPush=true, no remote configured — should succeed without push error
	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-nopush",
		Parent: "main",
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch with NoPush should not fail: %v", err)
	}
}

func TestBranchCustomTitle(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-title", "main", "")

	prov := &mockProvider{
		name: "github",
		createPRResult: &provider.PRResult{
			Number: 7,
			URL:    "url",
			Action: "created",
		},
	}

	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-title",
		Parent: "main",
		Title:  "My Custom Title",
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if len(prov.createPRCalls) != 1 {
		t.Fatalf("expected 1 CreatePR call, got %d", len(prov.createPRCalls))
	}
	if prov.createPRCalls[0].Title != "My Custom Title" {
		t.Errorf("expected custom title, got %q", prov.createPRCalls[0].Title)
	}
}

func TestBranchDraftFlag(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-draft", "main", "")

	prov := &mockProvider{
		name: "github",
		createPRResult: &provider.PRResult{
			Number: 8,
			URL:    "url",
			Action: "created",
		},
	}

	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-draft",
		Parent: "main",
		Title:  "Draft PR",
		Draft:  true,
		NoPush: true,
	})
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	if len(prov.createPRCalls) != 1 {
		t.Fatalf("expected 1 CreatePR call, got %d", len(prov.createPRCalls))
	}
	if !prov.createPRCalls[0].Draft {
		t.Error("expected Draft=true in CreatePR call")
	}
}

func TestBranchPushFails(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-pushfail", "main", "")

	prov := &mockProvider{name: "github"}

	// NoPush=false but no remote → push will fail
	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-pushfail",
		Parent: "main",
		NoPush: false,
	})
	if err == nil {
		t.Fatal("expected error when push fails")
	}
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestBranchCreatePRFails(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-createfail", "main", "")

	prov := &mockProvider{
		name:        "github",
		createPRErr: fmt.Errorf("API rate limited"),
	}

	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-createfail",
		Parent: "main",
		Title:  "Will fail",
		NoPush: true,
	})
	if err == nil {
		t.Fatal("expected error when CreatePR fails")
	}
	if got := err.Error(); got != "failed to create PR: API rate limited" {
		t.Errorf("unexpected error: %q", got)
	}
}

func TestBranchPushWithRemote(t *testing.T) {
	repo, dir, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	// Create feature branch with commit
	if err := exec.Command("git", "checkout", "-b", "feat-push").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "push.txt"), []byte("push"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Push feature"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}
	metadata.TrackBranch("feat-push", "main", "")

	prov := &mockProvider{
		name: "github",
		createPRResult: &provider.PRResult{
			Number: 100,
			URL:    "https://github.com/test/repo/pull/100",
			Action: "created",
		},
	}

	result, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "feat-push",
		Parent: "main",
		NoPush: false, // actually push
	})
	if err != nil {
		t.Fatalf("Branch with push failed: %v", err)
	}

	if result.PRNumber != 100 {
		t.Errorf("expected PR number 100, got %d", result.PRNumber)
	}
}

func TestBranchSetPRMetadataFailsForUntrackedBranch(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	// Don't track the branch in metadata — SetPR will fail
	metadata := &config.Metadata{Branches: make(map[string]*config.BranchMetadata)}

	prov := &mockProvider{
		name: "github",
		createPRResult: &provider.PRResult{
			Number: 1,
			URL:    "url",
			Action: "created",
		},
	}

	_, err := Branch(repo, metadata, prov, "origin", Opts{
		Branch: "untracked-branch",
		Parent: "main",
		Title:  "test",
		NoPush: true,
	})
	if err == nil {
		t.Fatal("expected error when SetPR fails for untracked branch")
	}
}

// --- DerivePRTitle() tests ---

func TestDerivePRTitleFromCommitMessage(t *testing.T) {
	repo, dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create branch with a specific commit message
	if err := exec.Command("git", "checkout", "-b", "feat-derive").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "derive.txt"), []byte("derive"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Implement authentication flow"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	title := DerivePRTitle(repo, "feat-derive", "main")
	if title != "Implement authentication flow" {
		t.Errorf("expected commit message as title, got %q", title)
	}
}

func TestDerivePRTitleFallsBackToBranchName(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	// Use a branch range that has no commits (main..main)
	title := DerivePRTitle(repo, "main", "main")
	if title != "Main" {
		t.Errorf("expected fallback to HumanizeBranchName, got %q", title)
	}
}

func TestDerivePRTitleWithMultipleCommits(t *testing.T) {
	repo, dir, cleanup := setupTestRepo(t)
	defer cleanup()

	if err := exec.Command("git", "checkout", "-b", "feat-multi").Run(); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	// First commit
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "First commit message"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Second commit
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "Second commit message"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// DerivePRTitle uses -1, so it gets the latest commit
	title := DerivePRTitle(repo, "feat-multi", "main")
	if title != "Second commit message" {
		t.Errorf("expected latest commit message, got %q", title)
	}
}

func TestDerivePRTitleNonExistentBranch(t *testing.T) {
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	// Non-existent branch — git log will fail, falls back to HumanizeBranchName
	title := DerivePRTitle(repo, "feat/nonexistent-branch", "main")
	if title != "Nonexistent branch" {
		t.Errorf("expected fallback title, got %q", title)
	}
}
