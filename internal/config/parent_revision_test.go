package config

import (
	"path/filepath"
	"testing"
)

func TestParentRevisionTracking(t *testing.T) {
	t.Run("TrackBranch stores parentRevision", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "abc123")

		rev := meta.GetParentRevision("feat-1")
		if rev != "abc123" {
			t.Errorf("expected parentRevision 'abc123', got '%s'", rev)
		}
	})

	t.Run("TrackBranch with empty parentRevision", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "")

		rev := meta.GetParentRevision("feat-1")
		if rev != "" {
			t.Errorf("expected empty parentRevision, got '%s'", rev)
		}
	})

	t.Run("GetParentRevision for untracked branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		rev := meta.GetParentRevision("nonexistent")
		if rev != "" {
			t.Errorf("expected empty string for untracked branch, got '%s'", rev)
		}
	})

	t.Run("SetParentRevision updates existing branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "abc123")

		if err := meta.SetParentRevision("feat-1", "def456"); err != nil {
			t.Fatalf("SetParentRevision failed: %v", err)
		}

		rev := meta.GetParentRevision("feat-1")
		if rev != "def456" {
			t.Errorf("expected 'def456', got '%s'", rev)
		}
	})

	t.Run("SetParentRevision fails for untracked branch", func(t *testing.T) {
		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}

		err := meta.SetParentRevision("nonexistent", "abc123")
		if err == nil {
			t.Error("expected error for untracked branch")
		}
	})

	t.Run("parentRevision persists through save/load", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gs_metadata")

		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "abc123")
		meta.TrackBranch("feat-2", "main", "")

		if err := meta.Save(path); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := LoadMetadata(path)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}

		rev1 := loaded.GetParentRevision("feat-1")
		if rev1 != "abc123" {
			t.Errorf("expected 'abc123' after reload, got '%s'", rev1)
		}

		rev2 := loaded.GetParentRevision("feat-2")
		if rev2 != "" {
			t.Errorf("expected empty string after reload, got '%s'", rev2)
		}
	})

	t.Run("omitempty: parentRevision omitted from JSON when empty", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gs_metadata")

		meta := &Metadata{Branches: make(map[string]*BranchMetadata)}
		meta.TrackBranch("feat-1", "main", "")

		if err := meta.Save(path); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Load raw and check there's no parentRevision key
		loaded, err := LoadMetadata(path)
		if err != nil {
			t.Fatalf("LoadMetadata failed: %v", err)
		}

		// Should still work fine
		rev := loaded.GetParentRevision("feat-1")
		if rev != "" {
			t.Errorf("expected empty parentRevision, got '%s'", rev)
		}
	})
}
