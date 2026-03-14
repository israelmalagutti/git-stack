package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Load returns error if not exists", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "nonexistent")
		_, err := Load(configPath)
		if err == nil {
			t.Error("expected error for nonexistent config")
		}
	})

	t.Run("NewConfig creates config with trunk", func(t *testing.T) {
		cfg := NewConfig("main")

		if cfg.Trunk != "main" {
			t.Errorf("expected trunk 'main', got '%s'", cfg.Trunk)
		}
		if cfg.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got '%s'", cfg.Version)
		}
	})

	t.Run("saves and loads config", func(t *testing.T) {
		cfg := NewConfig("develop")
		configPath := filepath.Join(tmpDir, ".gw_config")

		if err := cfg.Save(configPath); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if loaded.Trunk != "develop" {
			t.Errorf("expected trunk 'develop', got '%s'", loaded.Trunk)
		}
	})

	t.Run("IsInitialized returns false for nonexistent", func(t *testing.T) {
		if IsInitialized(filepath.Join(tmpDir, "nope")) {
			t.Error("should return false for nonexistent path")
		}
	})

	t.Run("IsInitialized returns true for existing", func(t *testing.T) {
		path := filepath.Join(tmpDir, "exists")
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		if !IsInitialized(path) {
			t.Error("should return true for existing path")
		}
	})
}

func TestMetadata(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates empty metadata if not exists", func(t *testing.T) {
		metadataPath := filepath.Join(tmpDir, "new_metadata")
		meta, err := LoadMetadata(metadataPath)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}

		if meta.Branches == nil {
			t.Error("Branches map should be initialized")
		}

		if len(meta.Branches) != 0 {
			t.Errorf("expected 0 branches, got %d", len(meta.Branches))
		}
	})

	t.Run("TrackBranch adds branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		meta.TrackBranch("feat-1", "main", "")

		if !meta.IsTracked("feat-1") {
			t.Error("feat-1 should be tracked")
		}

		parent, ok := meta.GetParent("feat-1")
		if !ok {
			t.Error("should have parent")
		}
		if parent != "main" {
			t.Errorf("expected parent 'main', got '%s'", parent)
		}
	})

	t.Run("UntrackBranch removes branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-2", "main", "")

		meta.UntrackBranch("feat-2")

		if meta.IsTracked("feat-2") {
			t.Error("feat-2 should not be tracked")
		}
	})

	t.Run("UpdateParent changes parent", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-3", "main", "")

		err := meta.UpdateParent("feat-3", "feat-1")
		if err != nil {
			t.Fatalf("UpdateParent failed: %v", err)
		}

		parent, _ := meta.GetParent("feat-3")
		if parent != "feat-1" {
			t.Errorf("expected parent 'feat-1', got '%s'", parent)
		}
	})

	t.Run("UpdateParent fails for untracked", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		err := meta.UpdateParent("nonexistent", "main")
		if err == nil {
			t.Error("expected error for untracked branch")
		}
	})

	t.Run("GetChildren returns child branches", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "")
		meta.TrackBranch("feat-2", "main", "")
		meta.TrackBranch("feat-3", "feat-1", "")

		children := meta.GetChildren("main")
		if len(children) != 2 {
			t.Errorf("expected 2 children of main, got %d", len(children))
		}

		children = meta.GetChildren("feat-1")
		if len(children) != 1 {
			t.Errorf("expected 1 child of feat-1, got %d", len(children))
		}
	})

	t.Run("saves and loads metadata", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-a", "main", "")
		meta.TrackBranch("feat-b", "feat-a", "")

		metadataPath := filepath.Join(tmpDir, ".gw_metadata")
		if err := meta.Save(metadataPath); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := LoadMetadata(metadataPath)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}

		if len(loaded.Branches) != 2 {
			t.Errorf("expected 2 branches, got %d", len(loaded.Branches))
		}

		parent, _ := loaded.GetParent("feat-b")
		if parent != "feat-a" {
			t.Error("parent relationship not preserved")
		}
	})

	t.Run("GetParent returns false for untracked", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		_, ok := meta.GetParent("nonexistent")
		if ok {
			t.Error("should return false for untracked branch")
		}
	})
}
