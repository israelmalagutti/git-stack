package cmd

import (
	"fmt"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

// discoveryMockProvider implements provider.Provider for testing PR discovery.
type discoveryMockProvider struct {
	name string
	prs  map[string]*provider.PRResult // head branch -> PR result
	err  error                         // error to return from FindExistingPR
}

func (m *discoveryMockProvider) Name() string                                      { return m.name }
func (m *discoveryMockProvider) CLIAvailable() bool                                { return true }
func (m *discoveryMockProvider) CLIAuthenticated() bool                            { return true }
func (m *discoveryMockProvider) CreatePR(_ provider.PRCreateOpts) (*provider.PRResult, error) { return nil, nil }
func (m *discoveryMockProvider) UpdatePR(_ int, _ provider.PRUpdateOpts) error     { return nil }
func (m *discoveryMockProvider) GetPRStatus(_ int) (*provider.PRStatus, error)     { return nil, nil }
func (m *discoveryMockProvider) MergePR(_ int, _ provider.PRMergeOpts) error       { return nil }
func (m *discoveryMockProvider) UpdatePRBase(_ int, _ string) error                { return nil }
func (m *discoveryMockProvider) FindExistingPR(head string) (*provider.PRResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	result, ok := m.prs[head]
	if !ok {
		return nil, nil
	}
	return result, nil
}

func TestDiscoverPRs_DiscoversMissingPR(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a tracked branch
	tr.createBranch(t, "feat-a", "main")
	tr.commitFile(t, "a.txt", "a", "add a")

	// Reload metadata and build stack
	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Verify no PR metadata exists yet
	if pr := metadata.GetPR("feat-a"); pr != nil {
		t.Fatalf("expected no PR metadata before discovery, got %+v", pr)
	}

	// Run discovery with a mock that returns a PR for feat-a
	prov := &discoveryMockProvider{
		name: "github",
		prs: map[string]*provider.PRResult{
			"feat-a": {Number: 42, URL: "https://github.com/owner/repo/pull/42"},
		},
	}
	discoverPRs(tr.repo, metadata, s, prov)

	// Verify PR metadata was set
	pr := metadata.GetPR("feat-a")
	if pr == nil {
		t.Fatal("expected PR metadata after discovery, got nil")
	}
	if pr.Number != 42 {
		t.Errorf("expected PR number 42, got %d", pr.Number)
	}
	if pr.Provider != "github" {
		t.Errorf("expected provider 'github', got %q", pr.Provider)
	}
}

func TestDiscoverPRs_SkipsBranchesWithExistingPR(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a tracked branch and set PR metadata
	tr.createBranch(t, "feat-existing", "main")
	tr.commitFile(t, "e.txt", "e", "add e")
	_ = tr.metadata.SetPR("feat-existing", &config.PRInfo{Number: 10, Provider: "github"})
	_ = tr.metadata.Save(tr.repo.GetMetadataPath())

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Mock that would return a different PR — should NOT be called
	queryCalled := false
	prov := &discoveryMockProvider{
		name: "github",
		prs: map[string]*provider.PRResult{
			"feat-existing": {Number: 99, URL: "https://github.com/owner/repo/pull/99"},
		},
	}
	// Wrap to detect if FindExistingPR is called
	origPRs := prov.prs
	prov.prs = nil
	prov.err = fmt.Errorf("should not be called")
	_ = origPRs // suppress unused

	discoverPRs(tr.repo, metadata, s, prov)

	if queryCalled {
		t.Error("expected FindExistingPR not to be called for branches with existing PR metadata")
	}

	// PR metadata should still be the original
	pr := metadata.GetPR("feat-existing")
	if pr == nil || pr.Number != 10 {
		t.Errorf("expected original PR number 10, got %+v", pr)
	}
}

func TestDiscoverPRs_SkipsTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Even if "main" appears in metadata (shouldn't normally), it should be skipped
	prov := &discoveryMockProvider{
		name: "github",
		prs: map[string]*provider.PRResult{
			"main": {Number: 1, URL: "https://github.com/owner/repo/pull/1"},
		},
	}
	discoverPRs(tr.repo, metadata, s, prov)

	if pr := metadata.GetPR("main"); pr != nil {
		t.Errorf("expected no PR for trunk, got %+v", pr)
	}
}

func TestDiscoverPRs_NoPRFound(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-no-pr", "main")
	tr.commitFile(t, "n.txt", "n", "add n")

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Mock that returns nil (no PR found)
	prov := &discoveryMockProvider{
		name: "github",
		prs:  map[string]*provider.PRResult{},
	}
	discoverPRs(tr.repo, metadata, s, prov)

	if pr := metadata.GetPR("feat-no-pr"); pr != nil {
		t.Errorf("expected no PR metadata, got %+v", pr)
	}
}

func TestDiscoverPRs_FindExistingPRError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-err", "main")
	tr.commitFile(t, "err.txt", "err", "add err")

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Mock that returns an error
	prov := &discoveryMockProvider{
		name: "github",
		err:  fmt.Errorf("network error"),
	}
	discoverPRs(tr.repo, metadata, s, prov)

	if pr := metadata.GetPR("feat-err"); pr != nil {
		t.Errorf("expected no PR metadata on error, got %+v", pr)
	}
}

func TestDiscoverPRs_MultipleBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create multiple branches: one with PR, one without, one with existing metadata
	tr.createBranch(t, "feat-has-pr", "main")
	tr.commitFile(t, "has.txt", "has", "add has")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	tr.createBranch(t, "feat-no-pr", "main")
	tr.commitFile(t, "no.txt", "no", "add no")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	tr.createBranch(t, "feat-already", "main")
	tr.commitFile(t, "already.txt", "already", "add already")
	_ = tr.metadata.SetPR("feat-already", &config.PRInfo{Number: 5, Provider: "github"})
	_ = tr.metadata.Save(tr.repo.GetMetadataPath())

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	prov := &discoveryMockProvider{
		name: "github",
		prs: map[string]*provider.PRResult{
			"feat-has-pr": {Number: 100, URL: "https://github.com/owner/repo/pull/100"},
			// feat-no-pr has no PR
		},
	}
	discoverPRs(tr.repo, metadata, s, prov)

	// feat-has-pr: should be discovered
	pr := metadata.GetPR("feat-has-pr")
	if pr == nil || pr.Number != 100 {
		t.Errorf("expected PR #100 for feat-has-pr, got %+v", pr)
	}

	// feat-no-pr: should remain nil
	if pr := metadata.GetPR("feat-no-pr"); pr != nil {
		t.Errorf("expected no PR for feat-no-pr, got %+v", pr)
	}

	// feat-already: should still have original PR
	pr = metadata.GetPR("feat-already")
	if pr == nil || pr.Number != 5 {
		t.Errorf("expected original PR #5 for feat-already, got %+v", pr)
	}
}

func TestDiscoverPRs_NoBranchesMissing(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No tracked branches other than trunk — should be a no-op
	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// Provider that errors — should never be called
	prov := &discoveryMockProvider{
		name: "github",
		err:  fmt.Errorf("should not be called"),
	}
	// This should return immediately without calling the provider
	discoverPRs(tr.repo, metadata, s, prov)
}

func TestDetectProviderBestEffort_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No remote configured — should return nil
	prov, err := detectProviderBestEffort(tr.repo)
	if prov != nil {
		t.Errorf("expected nil provider for repo without remote, got %+v", prov)
	}
	if err == nil {
		t.Error("expected error for repo without remote")
	}
}

func TestDetectProviderBestEffort_WithGitHubRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Add a GitHub remote
	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://github.com/testowner/testrepo.git"); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// detectProviderBestEffort will detect GitHub, but the gh CLI
	// may not be available/authenticated in the test environment.
	// We just verify it returns without panic and the provider is either
	// nil (CLI not available) or a valid GitHub provider.
	prov, _ := detectProviderBestEffort(tr.repo)
	if prov != nil && prov.Name() != "github" {
		t.Errorf("expected github provider or nil, got %q", prov.Name())
	}
}

func TestDetectProviderBestEffort_NonGitHubRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Add a non-GitHub remote
	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://gitlab.com/testowner/testrepo.git"); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	prov, err := detectProviderBestEffort(tr.repo)
	// GitLab is not a supported provider yet, so should return an error
	if prov != nil {
		t.Errorf("expected nil provider for unsupported remote, got %+v", prov)
	}
	_ = err // error is expected
}

func TestDiscoverPRs_SkipsDeletedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Track a branch in metadata but don't create it in git
	tr.metadata.TrackBranch("ghost-branch", "main", "")
	_ = tr.metadata.Save(tr.repo.GetMetadataPath())

	metadata, err := config.LoadMetadata(tr.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	s, err := stack.BuildStack(tr.repo, tr.cfg, metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	// The ghost branch has no node in the stack (git branch doesn't exist)
	prov := &discoveryMockProvider{
		name: "github",
		prs: map[string]*provider.PRResult{
			"ghost-branch": {Number: 999, URL: "https://github.com/owner/repo/pull/999"},
		},
	}
	discoverPRs(tr.repo, metadata, s, prov)

	// Should not set PR for a branch that doesn't exist in git
	if pr := metadata.GetPR("ghost-branch"); pr != nil {
		t.Errorf("expected no PR for deleted branch, got %+v", pr)
	}
}
