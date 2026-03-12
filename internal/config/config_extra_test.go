package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for invalid json")
	}

	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatalf("expected error for missing config")
	}
}

func TestSaveConfigError(t *testing.T) {
	cfg := NewConfig("main")
	dir := t.TempDir()
	if err := cfg.Save(dir); err == nil {
		t.Fatalf("expected error writing config to directory")
	}
}

func TestMetadataUpdateParentErrors(t *testing.T) {
	meta := &Metadata{Branches: map[string]*BranchMetadata{}}
	if err := meta.UpdateParent("missing", "main"); err == nil {
		t.Fatalf("expected error updating missing branch")
	}

	meta.TrackBranch("child", "main", "")
	if err := meta.UpdateParent("child", "new-parent"); err != nil {
		t.Fatalf("unexpected error updating parent: %v", err)
	}
}

func TestMetadataLoadSaveErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("{bad"), 0600); err != nil {
		t.Fatalf("failed to write bad json: %v", err)
	}
	if _, err := LoadMetadata(badPath); err == nil {
		t.Fatalf("expected error loading bad metadata")
	}

	meta := &Metadata{Branches: map[string]*BranchMetadata{}}
	if err := meta.Save(dir); err == nil {
		t.Fatalf("expected error saving metadata to directory")
	}
}
