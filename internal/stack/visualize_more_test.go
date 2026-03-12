package stack

import "testing"

func TestRenderShortWithRepoAndEmptyMessages(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	// Empty message commit on trunk to exercise parsing
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "--allow-empty-message", "-m", ""); err != nil {
		t.Fatalf("failed to commit empty message on trunk: %v", err)
	}

	// Create branches to build a sibling/child tree
	if err := repo.CreateBranch("feat-a"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-a", "main", "")
	if err := repo.CheckoutBranch("feat-a"); err != nil {
		t.Fatalf("failed to checkout feat-a: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat a"); err != nil {
		t.Fatalf("failed to commit feat-a: %v", err)
	}

	if err := repo.CreateBranch("feat-a-child"); err != nil {
		t.Fatalf("failed to create child branch: %v", err)
	}
	metadata.TrackBranch("feat-a-child", "feat-a", "")
	if err := repo.CheckoutBranch("feat-a-child"); err != nil {
		t.Fatalf("failed to checkout child: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "child"); err != nil {
		t.Fatalf("failed to commit child: %v", err)
	}

	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := repo.CreateBranch("feat-b"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-b", "main", "")
	if err := repo.CheckoutBranch("feat-b"); err != nil {
		t.Fatalf("failed to checkout feat-b: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat b"); err != nil {
		t.Fatalf("failed to commit feat-b: %v", err)
	}

	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := repo.CreateBranch("feat-empty"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-empty", "main", "")
	if err := repo.CheckoutBranch("feat-empty"); err != nil {
		t.Fatalf("failed to checkout feat-empty: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "--allow-empty-message", "-m", ""); err != nil {
		t.Fatalf("failed to commit empty message on branch: %v", err)
	}

	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	s, err := BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}

	if commits := getTrunkCommits(repo, "main", 5); commits == nil {
		t.Fatalf("expected trunk commits")
	}

	node := s.GetNode("feat-empty")
	if node == nil {
		t.Fatalf("expected node for feat-empty")
	}
	if commits := s.getBranchCommits(repo, node); commits == nil {
		t.Fatalf("expected branch commits")
	}

	short := s.RenderShort(repo)
	if short == "" {
		t.Fatalf("expected RenderShort output")
	}

	tree := s.RenderTree(repo, TreeOptions{ShowCommitSHA: true, ShowCommitMsg: true, Detailed: true})
	if tree == "" {
		t.Fatalf("expected RenderTree output")
	}
}
