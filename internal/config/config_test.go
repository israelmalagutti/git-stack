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
	defer func() { _ = os.RemoveAll(tmpDir) }()

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
		configPath := filepath.Join(tmpDir, ".gs_config")

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

	t.Run("Save to invalid path fails", func(t *testing.T) {
		cfg := NewConfig("main")
		err := cfg.Save(filepath.Join(tmpDir, "nonexistent-dir", "config"))
		if err == nil {
			t.Error("expected error saving to nonexistent directory")
		}
	})

	t.Run("Load invalid JSON config", func(t *testing.T) {
		badCfgPath := filepath.Join(tmpDir, "bad_config")
		if err := os.WriteFile(badCfgPath, []byte("{bad"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		_, err := Load(badCfgPath)
		if err == nil {
			t.Error("expected error for invalid JSON config")
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
	defer func() { _ = os.RemoveAll(tmpDir) }()

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

		metadataPath := filepath.Join(tmpDir, ".gs_metadata")
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

	t.Run("LoadMetadata with invalid JSON", func(t *testing.T) {
		badPath := filepath.Join(tmpDir, "bad_metadata")
		if err := os.WriteFile(badPath, []byte("{invalid json"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		_, err := LoadMetadata(badPath)
		if err == nil {
			t.Error("expected error for invalid JSON metadata")
		}
	})

	t.Run("LoadMetadata with null branches", func(t *testing.T) {
		nullPath := filepath.Join(tmpDir, "null_branches_metadata")
		if err := os.WriteFile(nullPath, []byte(`{"branches": null}`), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		meta, err := LoadMetadata(nullPath)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}
		if meta.Branches == nil {
			t.Error("expected Branches map to be initialized even when null in JSON")
		}
	})

	t.Run("SetParentRevision and GetParentRevision", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-rev", "main", "")

		if err := meta.SetParentRevision("feat-rev", "abc123"); err != nil {
			t.Fatalf("SetParentRevision failed: %v", err)
		}

		got := meta.GetParentRevision("feat-rev")
		if got != "abc123" {
			t.Errorf("expected 'abc123', got %q", got)
		}
	})

	t.Run("SetParentRevision fails for untracked", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		err := meta.SetParentRevision("nonexistent", "abc123")
		if err == nil {
			t.Error("expected error for untracked branch")
		}
	})

	t.Run("GetParentRevision returns empty for untracked", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		got := meta.GetParentRevision("nonexistent")
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("Save to invalid path fails", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		err := meta.Save(filepath.Join(tmpDir, "nonexistent-dir", "metadata"))
		if err == nil {
			t.Error("expected error saving to nonexistent directory")
		}
	})

	t.Run("SetPR and GetPR round-trip", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-pr", "main", "")

		pr := &PRInfo{Number: 42, Provider: "github"}
		err := meta.SetPR("feat-pr", pr)
		if err != nil {
			t.Fatalf("SetPR failed: %v", err)
		}

		got := meta.GetPR("feat-pr")
		if got == nil {
			t.Fatal("GetPR returned nil")
		}
		if got.Number != 42 {
			t.Errorf("expected PR number 42, got %d", got.Number)
		}
		if got.Provider != "github" {
			t.Errorf("expected provider 'github', got %q", got.Provider)
		}
	})

	t.Run("SetPR fails for untracked branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		pr := &PRInfo{Number: 1, Provider: "github"}
		err := meta.SetPR("untracked", pr)
		if err == nil {
			t.Error("expected error for untracked branch")
		}
	})

	t.Run("GetPR returns nil for untracked branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		got := meta.GetPR("nonexistent")
		if got != nil {
			t.Errorf("expected nil for untracked branch, got %+v", got)
		}
	})

	t.Run("GetPR returns nil when no PR set", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-no-pr", "main", "")

		got := meta.GetPR("feat-no-pr")
		if got != nil {
			t.Errorf("expected nil when no PR set, got %+v", got)
		}
	})
}
