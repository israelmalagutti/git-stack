package cmd

import (
	"os/exec"
	"testing"
)

func TestPushMetadataRefs_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Should silently return — no panic
	pushMetadataRefs(tr.repo, "some-branch")
}

func TestPushMetadataRefs_NoRemote_NoBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Push all refs (no branches specified) — should silently return
	pushMetadataRefs(tr.repo)
}

func TestPushConfigRef_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	pushConfigRef(tr.repo)
}

func TestDeleteRemoteMetadataRef_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	deleteRemoteMetadataRef(tr.repo, "some-branch")
}

func TestFetchMetadataRefs_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	got := fetchMetadataRefs(tr.repo)
	if got {
		t.Error("expected false without remote")
	}
}

func TestConfigureGSRefspec_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	configureGSRefspec(tr.repo)
}

func TestRepoHasRemote_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if repoHasRemote(tr.repo) {
		t.Error("expected no remote")
	}
}

// Tests with a bare remote

func addBareRemote(t *testing.T, tr *cmdTestRepo) string {
	t.Helper()
	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", bareDir); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("push", "-u", "origin", "main"); err != nil {
		t.Fatalf("failed to push main: %v", err)
	}
	return bareDir
}

func TestPushMetadataRefs_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	// Track a branch and save refs
	tr.createBranch(t, "feat-test", "main")
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Should not panic/error
	pushMetadataRefs(tr.repo, "feat-test")
}

func TestPushMetadataRefs_WithRemote_AllRefs(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	tr.createBranch(t, "feat-all", "main")
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Push all (no branch specified)
	pushMetadataRefs(tr.repo)
}

func TestPushConfigRef_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	pushConfigRef(tr.repo)
}

func TestDeleteRemoteMetadataRef_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	// Best-effort delete of a ref that may not exist on remote
	deleteRemoteMetadataRef(tr.repo, "nonexistent")
}

func TestFetchMetadataRefs_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	// Track and push refs first
	tr.createBranch(t, "feat-fetch", "main")
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}
	pushMetadataRefs(tr.repo, "feat-fetch")

	// Now fetch them back
	got := fetchMetadataRefs(tr.repo)
	if !got {
		t.Error("expected fetch to succeed with remote")
	}
}

func TestConfigureGSRefspec_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	configureGSRefspec(tr.repo)
}

func TestRepoHasRemote_WithRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()
	addBareRemote(t, tr)

	if !repoHasRemote(tr.repo) {
		t.Error("expected remote to be detected")
	}
}

func TestLoadMetadata(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	meta, err := loadMetadata(tr.repo)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
}
