package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

// mockProvider implements provider.Provider for testing submit flows.
type mockProvider struct {
	name          string
	cliAvail      bool
	cliAuth       bool
	createPRFn    func(opts provider.PRCreateOpts) (*provider.PRResult, error)
	findPRFn      func(head string) (*provider.PRResult, error)
	getPRStatusFn func(number int) (*provider.PRStatus, error)
}

func (m *mockProvider) Name() string                                    { return m.name }
func (m *mockProvider) CLIAvailable() bool                              { return m.cliAvail }
func (m *mockProvider) CLIAuthenticated() bool                          { return m.cliAuth }
func (m *mockProvider) UpdatePR(number int, opts provider.PRUpdateOpts) error { return nil }
func (m *mockProvider) MergePR(number int, opts provider.PRMergeOpts) error   { return nil }
func (m *mockProvider) UpdatePRBase(number int, newBase string) error         { return nil }

func (m *mockProvider) CreatePR(opts provider.PRCreateOpts) (*provider.PRResult, error) {
	if m.createPRFn != nil {
		return m.createPRFn(opts)
	}
	return &provider.PRResult{Number: 1, URL: "https://example.com/pr/1", Action: "created"}, nil
}

func (m *mockProvider) FindExistingPR(head string) (*provider.PRResult, error) {
	if m.findPRFn != nil {
		return m.findPRFn(head)
	}
	return nil, nil
}

func (m *mockProvider) GetPRStatus(number int) (*provider.PRStatus, error) {
	if m.getPRStatusFn != nil {
		return m.getPRStatusFn(number)
	}
	return nil, nil
}

func setupSubmitTestRepo(t *testing.T) (*cmdTestRepo, string) {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	// Create bare remote + local clone
	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "Init"},
		{"git", "-C", dir, "branch", "-M", "main"},
		{"git", "-C", dir, "remote", "add", "origin", bareDir},
		{"git", "-C", dir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	if err := os.Chdir(dir); err != nil {
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
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	tr := &cmdTestRepo{
		repo:     repo,
		cfg:      cfg,
		metadata: metadata,
		dir:      dir,
		cleanup: func() {
			_ = os.Chdir(origDir)
		},
	}

	return tr, bareDir
}

func TestSubmitCurrentBranch_WithMockProvider(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create a tracked branch with a commit
	tr.createBranch(t, "feat-submit", "main")
	tr.commitFile(t, "submit.txt", "data", "submit commit")

	// Build stack
	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{
		name:     "mock",
		cliAvail: true,
		cliAuth:  true,
	}

	submitDraft = false
	submitTitle = "Test PR"
	submitNoPush = false
	defer func() {
		submitDraft = false
		submitTitle = ""
		submitNoPush = false
	}()

	if err := submitCurrentBranch(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitCurrentBranch failed: %v", err)
	}
}

func TestSubmitCurrentBranch_OnTrunk(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Stay on main (trunk)
	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	err = submitCurrentBranch(tr.repo, tr.metadata, mock, s)
	if err == nil {
		t.Fatal("expected error submitting trunk")
	}
}

func TestSubmitCurrentBranch_WithParentNotPushed(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create chain: main -> parent -> child
	tr.createBranch(t, "feat-parent-np", "main")
	tr.commitFile(t, "parent-np.txt", "data", "parent commit")
	tr.createBranch(t, "feat-child-np", "feat-parent-np")
	tr.commitFile(t, "child-np.txt", "data", "child commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	submitNoPush = false
	defer func() { submitNoPush = false }()

	// This should push the parent first, then submit the child
	if err := submitCurrentBranch(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitCurrentBranch parent not pushed failed: %v", err)
	}
}

func TestSubmitStackBranches_WithMockProvider(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create a stack: main -> a -> b
	tr.createBranch(t, "feat-stack-a", "main")
	tr.commitFile(t, "sa.txt", "sa", "stack a commit")
	tr.createBranch(t, "feat-stack-b", "feat-stack-a")
	tr.commitFile(t, "sb.txt", "sb", "stack b commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	submitDraft = false
	submitNoPush = false
	defer func() {
		submitDraft = false
		submitNoPush = false
	}()

	if err := submitStackBranches(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitStackBranches failed: %v", err)
	}
}

func TestSubmitStackBranches_OnTrunk(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// On main, with no children in path
	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	// Stack submit from trunk — should succeed but submit 0 branches
	if err := submitStackBranches(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitStackBranches from trunk failed: %v", err)
	}
}

func TestSubmitCurrentBranch_Draft(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-draft", "main")
	tr.commitFile(t, "draft.txt", "data", "draft commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	submitDraft = true
	submitNoPush = false
	defer func() { submitDraft = false; submitNoPush = false }()

	if err := submitCurrentBranch(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitCurrentBranch draft failed: %v", err)
	}
}
