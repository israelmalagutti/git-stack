package config

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestLoadConfigPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"trunk":"main"}`), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	// Remove read permission
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0600) }()

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestLoadMetadataPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, []byte(`{"branches":{}}`), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0600) }()

	_, err := LoadMetadata(path)
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestLoadContinueStatePermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "continue_state.json")
	if err := os.WriteFile(path, []byte(`{"remainingBranches":[]}`), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0600) }()

	_, err := LoadContinueState(path)
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestClearContinueStatePermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	// Make directory read-only so Remove fails with permission error (not NotExist)
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := ClearContinueState(path)
	if err == nil {
		t.Fatal("expected permission error from ClearContinueState")
	}
}

func TestContinueStateSaveToInvalidPath(t *testing.T) {
	state := &ContinueState{
		RemainingBranches: []string{"a"},
		OriginalBranch:    "main",
	}
	err := state.Save(filepath.Join(t.TempDir(), "no-such-dir", "state.json"))
	if err == nil {
		t.Fatal("expected error saving to nonexistent directory")
	}
}

func TestSaveConfigToInvalidPath(t *testing.T) {
	cfg := NewConfig("main")
	err := cfg.Save(filepath.Join(t.TempDir(), "no-such-dir", "config.json"))
	if err == nil {
		t.Fatal("expected error saving config to nonexistent directory")
	}
}

func TestSaveMetadataToInvalidPath(t *testing.T) {
	meta := &Metadata{Branches: map[string]*BranchMetadata{}}
	err := meta.Save(filepath.Join(t.TempDir(), "no-such-dir", "meta.json"))
	if err == nil {
		t.Fatal("expected error saving metadata to nonexistent directory")
	}
}
