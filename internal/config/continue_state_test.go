package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContinueStateSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gw_continue_state")

	state := &ContinueState{
		RemainingBranches: []string{"feat-a", "feat-b", "feat-c"},
		OriginalBranch:    "main",
	}

	if err := state.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadContinueState(path)
	if err != nil {
		t.Fatalf("LoadContinueState failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}

	if len(loaded.RemainingBranches) != 3 {
		t.Fatalf("expected 3 remaining branches, got %d", len(loaded.RemainingBranches))
	}
	if loaded.RemainingBranches[0] != "feat-a" {
		t.Errorf("expected feat-a, got %s", loaded.RemainingBranches[0])
	}
	if loaded.OriginalBranch != "main" {
		t.Errorf("expected original branch 'main', got %s", loaded.OriginalBranch)
	}
}

func TestContinueStateLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent")

	state, err := LoadContinueState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Fatal("expected nil state for nonexistent file")
	}
}

func TestContinueStateLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gw_continue_state")

	if err := os.WriteFile(path, []byte("{bad json"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := LoadContinueState(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClearContinueState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gw_continue_state")

	state := &ContinueState{
		RemainingBranches: []string{"feat-a"},
		OriginalBranch:    "main",
	}
	if err := state.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := ClearContinueState(path); err != nil {
		t.Fatalf("ClearContinueState failed: %v", err)
	}

	// Should be gone
	loaded, err := LoadContinueState(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil state after clear")
	}
}

func TestClearContinueStateNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent")

	// Should not error when file doesn't exist
	if err := ClearContinueState(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestContinueStateSaveError(t *testing.T) {
	// Save to a directory (should fail)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blockfile"), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	state := &ContinueState{
		RemainingBranches: []string{"feat-a"},
		OriginalBranch:    "main",
	}
	// Writing to a directory path should fail
	if err := state.Save(filepath.Join(dir, "blockfile")); err == nil {
		t.Fatal("expected error saving to directory")
	}
}
