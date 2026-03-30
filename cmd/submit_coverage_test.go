package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/submit"
)

func TestRunSubmit_StackNoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-stack-sub", "main")

	submitStack = true
	submitDraft = false
	submitNoPush = false
	defer func() {
		submitStack = false
		submitDraft = false
		submitNoPush = false
	}()

	// No remote => detectProvider fails
	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error without remote")
	}
}

func TestRunSubmit_NonStackOnTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://github.com/user/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	// Stay on trunk, non-stack submit
	submitStack = false
	defer func() { submitStack = false }()

	// Should error because gh CLI is not available
	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error when submitting without CLI")
	}
}

func TestPrintSubmitResult_Skipped(t *testing.T) {
	// Test the "else" branch (action != "updated")
	r := &submit.Result{
		Branch:   "feat-x",
		Parent:   "main",
		PRNumber: 10,
		PRURL:    "https://github.com/user/repo/pull/10",
		Action:   "skipped",
		Provider: "github",
	}
	// Should not panic; action not "updated" => shows "Created" styling
	printSubmitResult(r)
}

func TestDetectProvider_SSHRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "git@github.com:user/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	prov, err := detectProvider(tr.repo)
	if err != nil {
		t.Fatalf("detectProvider SSH failed: %v", err)
	}
	if prov.Name() != "github" {
		t.Errorf("expected github provider, got %s", prov.Name())
	}
}
