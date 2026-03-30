package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/submit"
)

func TestDetectProvider_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	_, err := detectProvider(tr.repo)
	if err == nil {
		t.Fatal("expected error without remote")
	}
}

func TestDetectProvider_WithGitHubRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://github.com/user/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	prov, err := detectProvider(tr.repo)
	if err != nil {
		t.Fatalf("detectProvider failed: %v", err)
	}
	if prov.Name() != "github" {
		t.Errorf("expected github provider, got %s", prov.Name())
	}
}

func TestDetectProvider_WithUnknownRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://selfhosted.example.com/user/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	prov, err := detectProvider(tr.repo)
	if err != nil {
		t.Fatalf("detectProvider failed: %v", err)
	}
	// Generic provider for unknown hosts
	if prov == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestPrintSubmitResult_Created(t *testing.T) {
	// Just verify no panic
	r := &submit.Result{
		Branch:   "feat-a",
		Parent:   "main",
		PRNumber: 42,
		PRURL:    "https://github.com/user/repo/pull/42",
		Action:   "created",
		Provider: "github",
	}
	printSubmitResult(r)
}

func TestPrintSubmitResult_Updated(t *testing.T) {
	r := &submit.Result{
		Branch:   "feat-a",
		Parent:   "main",
		PRNumber: 42,
		PRURL:    "https://github.com/user/repo/pull/42",
		Action:   "updated",
		Provider: "github",
	}
	printSubmitResult(r)
}

func TestRunSubmit_NoRemote(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-sub", "main")

	submitStack = false
	submitDraft = false
	submitNoPush = false
	defer func() {
		submitStack = false
		submitDraft = false
		submitNoPush = false
	}()

	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error without remote")
	}
}

func TestRunSubmit_OnTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Add a remote so detectProvider passes
	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "https://github.com/user/repo.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	// Stay on main (trunk)
	submitStack = false
	defer func() { submitStack = false }()

	// runSubmit should fail because gh CLI is not available in test env,
	// OR because we're on trunk. Either way it should error.
	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error when submitting from trunk or without CLI")
	}
}
