package cmd

import (
	"os"
	"os/exec"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestRunInitAlreadyInitialized(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runInit(nil, nil); err == nil {
		t.Fatalf("expected already initialized error")
	}
}

func TestRunInitCancelled(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit cancel failed: %v", err)
		}
	})
}

func TestRunInitNoBranches(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	if err := runInit(nil, nil); err == nil {
		t.Fatalf("expected no branches error")
	}
}
