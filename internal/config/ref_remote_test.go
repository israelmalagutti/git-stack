package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/israelmalagutti/git-stack/internal/git"
)

// setupRemoteRefTestRepos creates a local repo with a bare remote for testing ref sync.
func setupRemoteRefTestRepos(t *testing.T) (localRepo *git.Repo, otherRepo *git.Repo, cleanup func()) {
	t.Helper()

	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	local := filepath.Join(base, "local")
	other := filepath.Join(base, "other")

	if err := exec.Command("git", "init", "--bare", remote).Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}
	if err := exec.Command("git", "init", local).Run(); err != nil {
		t.Fatalf("failed to init local: %v", err)
	}
	if err := exec.Command("git", "init", other).Run(); err != nil {
		t.Fatalf("failed to init other: %v", err)
	}

	for _, dir := range []string{local, other} {
		for _, args := range [][]string{
			{"git", "-C", dir, "config", "user.email", "test@test.com"},
			{"git", "-C", dir, "config", "user.name", "Test User"},
			{"git", "-C", dir, "config", "commit.gpgsign", "false"},
			{"git", "-C", dir, "remote", "add", "origin", remote},
		} {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
				t.Fatalf("failed to run %v: %v", args, err)
			}
		}
	}

	// Initial commit in local
	readme := filepath.Join(local, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", local, "add", "."},
		{"git", "-C", local, "commit", "-m", "initial"},
		{"git", "-C", local, "branch", "-M", "main"},
		{"git", "-C", local, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	// Set up other repo
	for _, args := range [][]string{
		{"git", "-C", other, "fetch", "origin"},
		{"git", "-C", other, "checkout", "-b", "main", "origin/main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	origDir, _ := os.Getwd()

	// Create Repo objects
	if err := os.Chdir(local); err != nil {
		t.Fatalf("failed to chdir to local: %v", err)
	}
	lr, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (local) failed: %v", err)
	}

	if err := os.Chdir(other); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}
	or, err := git.NewRepo()
	if err != nil {
		t.Fatalf("NewRepo (other) failed: %v", err)
	}

	// Return to local as default
	if err := os.Chdir(local); err != nil {
		t.Fatalf("failed to chdir back to local: %v", err)
	}

	return lr, or, func() {
		_ = os.Chdir(origDir)
	}
}

func TestPushAndFetchBranchMeta(t *testing.T) {
	localRepo, otherRepo, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	// Alice writes branch metadata and pushes
	meta := &BranchMetadata{
		Parent:  "main",
		Tracked: true,
		Created: time.Date(2026, 3, 27, 14, 30, 0, 0, time.UTC),
	}
	if err := WriteRefBranchMeta(localRepo, "feat/auth", meta); err != nil {
		t.Fatalf("WriteRefBranchMeta failed: %v", err)
	}

	if err := PushBranchMeta(localRepo, "origin", "feat/auth"); err != nil {
		t.Fatalf("PushBranchMeta failed: %v", err)
	}

	// Bob fetches
	otherDir := otherRepo.GetWorkDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}

	if err := FetchAllRefs(otherRepo, "origin"); err != nil {
		t.Fatalf("FetchAllRefs failed: %v", err)
	}

	// Bob reads the metadata
	got, err := ReadRefBranchMeta(otherRepo, "feat/auth")
	if err != nil {
		t.Fatalf("ReadRefBranchMeta (other) failed: %v", err)
	}

	if got.Parent != "main" {
		t.Errorf("expected parent 'main', got %q", got.Parent)
	}
	if !got.Tracked {
		t.Error("expected tracked=true")
	}
}

func TestPushAllRefsAndFetchAll(t *testing.T) {
	localRepo, otherRepo, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	// Write config + multiple branch metadata
	cfg := NewConfig("main")
	if err := WriteRefConfig(localRepo, cfg); err != nil {
		t.Fatalf("WriteRefConfig failed: %v", err)
	}

	branches := map[string]*BranchMetadata{
		"feat/auth":    {Parent: "main", Tracked: true, Created: time.Now()},
		"feat/auth-ui": {Parent: "feat/auth", Tracked: true, Created: time.Now()},
		"feat/logging": {Parent: "main", Tracked: true, Created: time.Now()},
	}
	for name, meta := range branches {
		if err := WriteRefBranchMeta(localRepo, name, meta); err != nil {
			t.Fatalf("WriteRefBranchMeta(%s) failed: %v", name, err)
		}
	}

	// Push all
	if err := PushAllRefs(localRepo, "origin"); err != nil {
		t.Fatalf("PushAllRefs failed: %v", err)
	}

	// Bob fetches
	otherDir := otherRepo.GetWorkDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}

	if err := FetchAllRefs(otherRepo, "origin"); err != nil {
		t.Fatalf("FetchAllRefs failed: %v", err)
	}

	// Bob reads all metadata
	allMeta, err := ReadAllRefMeta(otherRepo)
	if err != nil {
		t.Fatalf("ReadAllRefMeta (other) failed: %v", err)
	}

	if len(allMeta) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(allMeta))
	}

	for name, expected := range branches {
		got, ok := allMeta[name]
		if !ok {
			t.Errorf("missing branch %s", name)
			continue
		}
		if got.Parent != expected.Parent {
			t.Errorf("%s: expected parent %q, got %q", name, expected.Parent, got.Parent)
		}
	}

	// Bob reads config
	gotCfg, err := ReadRefConfig(otherRepo)
	if err != nil {
		t.Fatalf("ReadRefConfig (other) failed: %v", err)
	}
	if gotCfg.Trunk != "main" {
		t.Errorf("expected trunk 'main', got %q", gotCfg.Trunk)
	}
}

func TestDeleteRemoteBranchMeta(t *testing.T) {
	localRepo, otherRepo, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	// Write and push
	meta := &BranchMetadata{Parent: "main", Tracked: true, Created: time.Now()}
	if err := WriteRefBranchMeta(localRepo, "feat-delete", meta); err != nil {
		t.Fatalf("WriteRefBranchMeta failed: %v", err)
	}
	if err := PushBranchMeta(localRepo, "origin", "feat-delete"); err != nil {
		t.Fatalf("PushBranchMeta failed: %v", err)
	}

	// Delete from remote
	if err := DeleteRemoteBranchMeta(localRepo, "origin", "feat-delete"); err != nil {
		t.Fatalf("DeleteRemoteBranchMeta failed: %v", err)
	}

	// Bob fetches - should not get the deleted ref
	otherDir := otherRepo.GetWorkDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}

	_ = FetchAllRefs(otherRepo, "origin")

	allMeta, err := ReadAllRefMeta(otherRepo)
	if err != nil {
		t.Fatalf("ReadAllRefMeta failed: %v", err)
	}

	if _, found := allMeta["feat-delete"]; found {
		t.Error("expected feat-delete to not be present after remote deletion")
	}
}

func TestPushConfig(t *testing.T) {
	localRepo, otherRepo, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	// Write config locally
	cfg := NewConfig("main")
	if err := WriteRefConfig(localRepo, cfg); err != nil {
		t.Fatalf("WriteRefConfig failed: %v", err)
	}

	// Push config to remote
	if err := PushConfig(localRepo, "origin"); err != nil {
		t.Fatalf("PushConfig failed: %v", err)
	}

	// Bob fetches and reads config
	otherDir := otherRepo.GetWorkDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("failed to chdir to other: %v", err)
	}

	if err := FetchAllRefs(otherRepo, "origin"); err != nil {
		t.Fatalf("FetchAllRefs failed: %v", err)
	}

	gotCfg, err := ReadRefConfig(otherRepo)
	if err != nil {
		t.Fatalf("ReadRefConfig (other) failed: %v", err)
	}

	if gotCfg.Trunk != "main" {
		t.Errorf("expected trunk 'main', got %q", gotCfg.Trunk)
	}
	if gotCfg.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", gotCfg.Version)
	}
}

func TestConfigureRemoteRefspec(t *testing.T) {
	localRepo, _, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	if err := ConfigureRemoteRefspec(localRepo, "origin"); err != nil {
		t.Fatalf("ConfigureRemoteRefspec failed: %v", err)
	}

	// Verify the refspec was added
	has, err := localRepo.HasRefspec("origin", "+refs/gs/*:refs/gs/*")
	if err != nil {
		t.Fatalf("HasRefspec failed: %v", err)
	}
	if !has {
		t.Error("expected gs refspec to be configured")
	}

	// Calling again should be idempotent
	if err := ConfigureRemoteRefspec(localRepo, "origin"); err != nil {
		t.Fatalf("ConfigureRemoteRefspec (idempotent) failed: %v", err)
	}
}

func TestSaveWithRefsCleanup(t *testing.T) {
	localRepo, _, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	localDir := localRepo.GetWorkDir()
	metaPath := filepath.Join(localDir, ".gs_stack_metadata")

	meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
	meta.TrackBranch("feat/one", "main", "")
	meta.TrackBranch("feat/two", "feat/one", "")

	// SaveWithRefs writes both JSON and refs
	if err := meta.SaveWithRefs(localRepo, metaPath); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	// Verify JSON file was written
	loaded, err := LoadMetadata(metaPath)
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}
	if len(loaded.Branches) != 2 {
		t.Errorf("expected 2 branches in JSON, got %d", len(loaded.Branches))
	}

	// Verify refs were written
	allMeta, err := ReadAllRefMeta(localRepo)
	if err != nil {
		t.Fatalf("ReadAllRefMeta failed: %v", err)
	}
	if len(allMeta) != 2 {
		t.Errorf("expected 2 branches in refs, got %d", len(allMeta))
	}

	// Now remove a branch and save again -- stale ref should be cleaned
	meta.UntrackBranch("feat/two")
	if err := meta.SaveWithRefs(localRepo, metaPath); err != nil {
		t.Fatalf("SaveWithRefs (after untrack) failed: %v", err)
	}

	allMeta, err = ReadAllRefMeta(localRepo)
	if err != nil {
		t.Fatalf("ReadAllRefMeta after cleanup failed: %v", err)
	}
	if _, found := allMeta["feat/two"]; found {
		t.Error("expected feat/two ref to be cleaned up after untrack")
	}
}

func TestWriteRefBranchMetaValidation(t *testing.T) {
	localRepo, _, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	// Branch name with "--" should fail validation
	meta := &BranchMetadata{Parent: "main", Tracked: true}
	err := WriteRefBranchMeta(localRepo, "feat--bad", meta)
	if err == nil {
		t.Error("expected error for branch name containing --")
	}
}

func TestLoadMetadataWithRefsFallback(t *testing.T) {
	localRepo, _, cleanup := setupRemoteRefTestRepos(t)
	defer cleanup()

	localDir := localRepo.GetWorkDir()
	metaPath := filepath.Join(localDir, ".gs_stack_metadata")

	// No refs and no JSON -- should return empty
	meta, source, err := LoadMetadataWithRefs(localRepo, metaPath)
	if err != nil {
		t.Fatalf("LoadMetadataWithRefs failed: %v", err)
	}
	if source != SourceEmpty {
		t.Errorf("expected SourceEmpty, got %d", source)
	}
	if len(meta.Branches) != 0 {
		t.Errorf("expected 0 branches, got %d", len(meta.Branches))
	}

	// Write JSON metadata (no refs)
	jsonMeta := &Metadata{Branches: make(map[string]*BranchMetadata)}
	jsonMeta.TrackBranch("json-feat", "main", "")
	if err := jsonMeta.Save(metaPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	meta, source, err = LoadMetadataWithRefs(localRepo, metaPath)
	if err != nil {
		t.Fatalf("LoadMetadataWithRefs failed: %v", err)
	}
	if source != SourceJSON {
		t.Errorf("expected SourceJSON, got %d", source)
	}
	if !meta.IsTracked("json-feat") {
		t.Error("expected json-feat to be tracked")
	}

	// Write refs -- should prefer refs over JSON
	refMeta := &BranchMetadata{Parent: "main", Tracked: true}
	if err := WriteRefBranchMeta(localRepo, "ref/feat", refMeta); err != nil {
		t.Fatalf("WriteRefBranchMeta failed: %v", err)
	}

	meta, source, err = LoadMetadataWithRefs(localRepo, metaPath)
	if err != nil {
		t.Fatalf("LoadMetadataWithRefs failed: %v", err)
	}
	if source != SourceRefs {
		t.Errorf("expected SourceRefs, got %d", source)
	}
}
