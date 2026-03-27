package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/israelmalagutti/git-stack/internal/git"
)

// setupRefTestRepo creates a temporary git repo and returns a Repo instance.
func setupRefTestRepo(t *testing.T) (*git.Repo, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-ref-test-*")
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

	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
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

	cleanup := func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}

	return repo, tmpDir, cleanup
}

func TestWriteAndReadRefBranchMeta(t *testing.T) {
	repo, _, cleanup := setupRefTestRepo(t)
	defer cleanup()

	meta := &BranchMetadata{
		Parent:         "main",
		Tracked:        true,
		Created:        time.Date(2026, 3, 27, 14, 30, 0, 0, time.UTC),
		ParentRevision: "abc123",
	}

	t.Run("write and read round-trip", func(t *testing.T) {
		if err := WriteRefBranchMeta(repo, "feat-auth", meta); err != nil {
			t.Fatalf("WriteRefBranchMeta failed: %v", err)
		}

		got, err := ReadRefBranchMeta(repo, "feat-auth")
		if err != nil {
			t.Fatalf("ReadRefBranchMeta failed: %v", err)
		}

		if got.Parent != "main" {
			t.Errorf("expected parent 'main', got %q", got.Parent)
		}
		if !got.Tracked {
			t.Error("expected tracked=true")
		}
		if got.ParentRevision != "abc123" {
			t.Errorf("expected parentRevision 'abc123', got %q", got.ParentRevision)
		}
	})

	t.Run("read nonexistent branch returns error", func(t *testing.T) {
		_, err := ReadRefBranchMeta(repo, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent branch")
		}
	})
}

func TestWriteAndReadRefBranchMetaWithSlashes(t *testing.T) {
	repo, _, cleanup := setupRefTestRepo(t)
	defer cleanup()

	branches := map[string]*BranchMetadata{
		"feat/auth": {
			Parent:  "main",
			Tracked: true,
			Created: time.Now(),
		},
		"feat/auth/ui": {
			Parent:  "feat/auth",
			Tracked: true,
			Created: time.Now(),
		},
	}

	for name, meta := range branches {
		if err := WriteRefBranchMeta(repo, name, meta); err != nil {
			t.Fatalf("WriteRefBranchMeta(%s) failed: %v", name, err)
		}
	}

	for name, expected := range branches {
		got, err := ReadRefBranchMeta(repo, name)
		if err != nil {
			t.Fatalf("ReadRefBranchMeta(%s) failed: %v", name, err)
		}
		if got.Parent != expected.Parent {
			t.Errorf("%s: expected parent %q, got %q", name, expected.Parent, got.Parent)
		}
	}
}

func TestReadAllRefMeta(t *testing.T) {
	repo, _, cleanup := setupRefTestRepo(t)
	defer cleanup()

	t.Run("empty when no refs", func(t *testing.T) {
		branches, err := ReadAllRefMeta(repo)
		if err != nil {
			t.Fatalf("ReadAllRefMeta failed: %v", err)
		}
		if len(branches) != 0 {
			t.Errorf("expected 0 branches, got %d", len(branches))
		}
	})

	t.Run("reads all branch metadata", func(t *testing.T) {
		meta1 := &BranchMetadata{Parent: "main", Tracked: true, Created: time.Now()}
		meta2 := &BranchMetadata{Parent: "feat-a", Tracked: true, Created: time.Now()}

		if err := WriteRefBranchMeta(repo, "feat-a", meta1); err != nil {
			t.Fatalf("WriteRefBranchMeta failed: %v", err)
		}
		if err := WriteRefBranchMeta(repo, "feat-b", meta2); err != nil {
			t.Fatalf("WriteRefBranchMeta failed: %v", err)
		}

		branches, err := ReadAllRefMeta(repo)
		if err != nil {
			t.Fatalf("ReadAllRefMeta failed: %v", err)
		}

		if len(branches) != 2 {
			t.Fatalf("expected 2 branches, got %d", len(branches))
		}

		if branches["feat-a"].Parent != "main" {
			t.Errorf("feat-a: expected parent 'main', got %q", branches["feat-a"].Parent)
		}
		if branches["feat-b"].Parent != "feat-a" {
			t.Errorf("feat-b: expected parent 'feat-a', got %q", branches["feat-b"].Parent)
		}
	})
}

func TestDeleteRefBranchMeta(t *testing.T) {
	repo, _, cleanup := setupRefTestRepo(t)
	defer cleanup()

	meta := &BranchMetadata{Parent: "main", Tracked: true, Created: time.Now()}
	if err := WriteRefBranchMeta(repo, "to-delete", meta); err != nil {
		t.Fatalf("WriteRefBranchMeta failed: %v", err)
	}

	if err := DeleteRefBranchMeta(repo, "to-delete"); err != nil {
		t.Fatalf("DeleteRefBranchMeta failed: %v", err)
	}

	_, err := ReadRefBranchMeta(repo, "to-delete")
	if err == nil {
		t.Error("expected error after deleting branch metadata")
	}
}

func TestWriteAndReadRefConfig(t *testing.T) {
	repo, _, cleanup := setupRefTestRepo(t)
	defer cleanup()

	cfg := NewConfig("main")

	t.Run("write and read round-trip", func(t *testing.T) {
		if err := WriteRefConfig(repo, cfg); err != nil {
			t.Fatalf("WriteRefConfig failed: %v", err)
		}

		got, err := ReadRefConfig(repo)
		if err != nil {
			t.Fatalf("ReadRefConfig failed: %v", err)
		}

		if got.Trunk != "main" {
			t.Errorf("expected trunk 'main', got %q", got.Trunk)
		}
		if got.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", got.Version)
		}
	})

	t.Run("read nonexistent config returns error", func(t *testing.T) {
		// Use a fresh repo to avoid the config we just wrote
		repo2, _, cleanup2 := setupRefTestRepo(t)
		defer cleanup2()

		_, err := ReadRefConfig(repo2)
		if err == nil {
			t.Error("expected error for nonexistent config ref")
		}
	})
}

func TestLoadMetadataWithRefs(t *testing.T) {
	repo, tmpDir, cleanup := setupRefTestRepo(t)
	defer cleanup()

	jsonPath := filepath.Join(tmpDir, ".gs_stack_metadata")

	t.Run("returns empty when nothing exists", func(t *testing.T) {
		meta, source, err := LoadMetadataWithRefs(repo, jsonPath)
		if err != nil {
			t.Fatalf("LoadMetadataWithRefs failed: %v", err)
		}
		if source != SourceEmpty {
			t.Errorf("expected SourceEmpty, got %d", source)
		}
		if len(meta.Branches) != 0 {
			t.Errorf("expected 0 branches, got %d", len(meta.Branches))
		}
	})

	t.Run("loads from JSON when only JSON exists", func(t *testing.T) {
		jsonMeta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		jsonMeta.TrackBranch("feat-json", "main", "")
		if err := jsonMeta.Save(jsonPath); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		meta, source, err := LoadMetadataWithRefs(repo, jsonPath)
		if err != nil {
			t.Fatalf("LoadMetadataWithRefs failed: %v", err)
		}
		if source != SourceJSON {
			t.Errorf("expected SourceJSON, got %d", source)
		}
		if !meta.IsTracked("feat-json") {
			t.Error("expected feat-json to be tracked")
		}

		// Clean up JSON for next test
		_ = os.Remove(jsonPath)
	})

	t.Run("prefers refs when both exist", func(t *testing.T) {
		// Write to refs
		refMeta := &BranchMetadata{Parent: "main", Tracked: true, Created: time.Now()}
		if err := WriteRefBranchMeta(repo, "feat-ref", refMeta); err != nil {
			t.Fatalf("WriteRefBranchMeta failed: %v", err)
		}

		// Write different data to JSON
		jsonMeta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		jsonMeta.TrackBranch("feat-json-only", "main", "")
		if err := jsonMeta.Save(jsonPath); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		meta, source, err := LoadMetadataWithRefs(repo, jsonPath)
		if err != nil {
			t.Fatalf("LoadMetadataWithRefs failed: %v", err)
		}
		if source != SourceRefs {
			t.Errorf("expected SourceRefs, got %d", source)
		}
		if !meta.IsTracked("feat-ref") {
			t.Error("expected feat-ref from refs to be tracked")
		}
	})
}

func TestSaveWithRefs(t *testing.T) {
	repo, tmpDir, cleanup := setupRefTestRepo(t)
	defer cleanup()

	jsonPath := filepath.Join(tmpDir, ".gs_stack_metadata")

	t.Run("writes to both JSON and refs", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-dual", "main", "abc123")

		if err := meta.SaveWithRefs(repo, jsonPath); err != nil {
			t.Fatalf("SaveWithRefs failed: %v", err)
		}

		// Verify JSON was written
		jsonMeta, err := LoadMetadata(jsonPath)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}
		if !jsonMeta.IsTracked("feat-dual") {
			t.Error("expected feat-dual in JSON")
		}

		// Verify ref was written
		refMeta, err := ReadRefBranchMeta(repo, "feat-dual")
		if err != nil {
			t.Fatalf("ReadRefBranchMeta failed: %v", err)
		}
		if refMeta.Parent != "main" {
			t.Errorf("expected parent 'main' in ref, got %q", refMeta.Parent)
		}
	})

	t.Run("cleans up orphaned refs", func(t *testing.T) {
		// First save with two branches
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-keep", "main", "")
		meta.TrackBranch("feat-remove", "main", "")
		if err := meta.SaveWithRefs(repo, jsonPath); err != nil {
			t.Fatalf("SaveWithRefs failed: %v", err)
		}

		// Verify both refs exist
		if _, err := ReadRefBranchMeta(repo, "feat-keep"); err != nil {
			t.Fatalf("expected feat-keep ref to exist: %v", err)
		}
		if _, err := ReadRefBranchMeta(repo, "feat-remove"); err != nil {
			t.Fatalf("expected feat-remove ref to exist: %v", err)
		}

		// Save again without feat-remove
		meta2 := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta2.TrackBranch("feat-keep", "main", "")
		if err := meta2.SaveWithRefs(repo, jsonPath); err != nil {
			t.Fatalf("SaveWithRefs second call failed: %v", err)
		}

		// feat-keep should still exist
		if _, err := ReadRefBranchMeta(repo, "feat-keep"); err != nil {
			t.Fatalf("expected feat-keep ref to still exist: %v", err)
		}

		// feat-remove should be cleaned up
		_, err := ReadRefBranchMeta(repo, "feat-remove")
		if err == nil {
			t.Error("expected feat-remove ref to be cleaned up")
		}
	})
}

func TestSaveWithRefsSlashBranches(t *testing.T) {
	repo, tmpDir, cleanup := setupRefTestRepo(t)
	defer cleanup()

	jsonPath := filepath.Join(tmpDir, ".gs_stack_metadata")

	meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
	meta.TrackBranch("feat/auth", "main", "")
	meta.TrackBranch("feat/auth-ui", "feat/auth", "")

	if err := meta.SaveWithRefs(repo, jsonPath); err != nil {
		t.Fatalf("SaveWithRefs failed: %v", err)
	}

	// Verify we can read back via ReadAllRefMeta
	branches, err := ReadAllRefMeta(repo)
	if err != nil {
		t.Fatalf("ReadAllRefMeta failed: %v", err)
	}

	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches["feat/auth"].Parent != "main" {
		t.Errorf("expected feat/auth parent 'main', got %q", branches["feat/auth"].Parent)
	}
	if branches["feat/auth-ui"].Parent != "feat/auth" {
		t.Errorf("expected feat/auth-ui parent 'feat/auth', got %q", branches["feat/auth-ui"].Parent)
	}
}
