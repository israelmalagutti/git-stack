package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveConfigFiles(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	commonDir := tr.repo.GetCommonDir()

	// Create some gs and gw config files
	files := []string{
		".gs_config", ".gs_stack_metadata", ".gs_continue_state",
		".gw_config", ".gw_stack_metadata", ".gw_continue_state",
	}
	for _, f := range files {
		path := filepath.Join(commonDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	removeConfigFiles(tr.repo)

	for _, f := range files {
		path := filepath.Join(commonDir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", f)
		}
	}
}

func TestRemoveConfigFiles_NoneExist(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Should not panic even if none of the files exist
	// (the setupCmdTestRepo creates .gs_config and .gs_stack_metadata,
	// but removeConfigFiles should handle missing ones too)
	removeConfigFiles(tr.repo)
}

func TestMigrateFromGW_NoLegacyFiles(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	migrated, err := migrateFromGW(tr.repo)
	if err != nil {
		t.Fatalf("migrateFromGW error: %v", err)
	}
	if migrated {
		t.Error("expected no migration when no legacy files exist")
	}
}

func TestMigrateFromGW_WithLegacyFiles(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	commonDir := tr.repo.GetCommonDir()

	// Remove the gs files so migration can proceed
	os.Remove(filepath.Join(commonDir, ".gs_config"))
	os.Remove(filepath.Join(commonDir, ".gs_stack_metadata"))

	// Create legacy gw files
	gw := map[string]string{
		".gw_config":         `{"trunk":"main"}`,
		".gw_stack_metadata": `{"branches":{}}`,
	}
	for name, content := range gw {
		path := filepath.Join(commonDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	migrated, err := migrateFromGW(tr.repo)
	if err != nil {
		t.Fatalf("migrateFromGW error: %v", err)
	}
	if !migrated {
		t.Error("expected migration to occur")
	}

	// gs files should now exist
	for _, gsFile := range []string{".gs_config", ".gs_stack_metadata"} {
		path := filepath.Join(commonDir, gsFile)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after migration", gsFile)
		}
	}

	// gw files should be gone
	for _, gwFile := range []string{".gw_config", ".gw_stack_metadata"} {
		path := filepath.Join(commonDir, gwFile)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed after migration", gwFile)
		}
	}
}

func TestMigrateFromGW_GsAlreadyExists(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	commonDir := tr.repo.GetCommonDir()

	// gs files already exist (from setupCmdTestRepo). Create gw files.
	gwFiles := []string{".gw_config", ".gw_stack_metadata"}
	for _, f := range gwFiles {
		path := filepath.Join(commonDir, f)
		if err := os.WriteFile(path, []byte("legacy"), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	migrated, err := migrateFromGW(tr.repo)
	if err != nil {
		t.Fatalf("migrateFromGW error: %v", err)
	}
	if !migrated {
		t.Error("expected migration flag (gw files removed)")
	}

	// gw files should be removed
	for _, f := range gwFiles {
		path := filepath.Join(commonDir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", f)
		}
	}
}

func TestRunInit_Reset(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	initReset = true
	defer func() { initReset = false }()

	// After reset, init should re-prompt for trunk selection
	// Mock askOne to select "main"
	withAskOne(t, []interface{}{"main"}, func() {
		err := runInit(initCmd, nil)
		if err != nil {
			t.Fatalf("runInit --reset failed: %v", err)
		}
	})
}
