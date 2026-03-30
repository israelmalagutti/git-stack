package cmd

import (
	"fmt"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/israelmalagutti/git-stack/internal/submit"
)

func TestSubmitCurrentBranch_CreatePRError(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-err", "main")
	tr.commitFile(t, "err.txt", "data", "err commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{
		name:     "mock",
		cliAvail: true,
		cliAuth:  true,
		createPRFn: func(opts provider.PRCreateOpts) (*provider.PRResult, error) {
			return nil, fmt.Errorf("API error")
		},
	}

	submitNoPush = false
	submitTitle = ""
	defer func() { submitNoPush = false; submitTitle = "" }()

	err = submitCurrentBranch(tr.repo, tr.metadata, mock, s)
	if err == nil {
		t.Fatal("expected error from failing PR creation")
	}
}

func TestSubmitCurrentBranch_NoPush(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-nopush", "main")
	tr.commitFile(t, "nopush.txt", "data", "nopush commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	submitNoPush = true
	submitTitle = "no push PR"
	defer func() { submitNoPush = false; submitTitle = "" }()

	if err := submitCurrentBranch(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitCurrentBranch nopush failed: %v", err)
	}
}

func TestSubmitStackBranches_WithFailingBranch(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create a stack: main -> a -> b
	tr.createBranch(t, "feat-fail-a", "main")
	tr.commitFile(t, "fa.txt", "a", "a commit")
	tr.createBranch(t, "feat-fail-b", "feat-fail-a")
	tr.commitFile(t, "fb.txt", "b", "b commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	callCount := 0
	mock := &mockProvider{
		name:     "mock",
		cliAvail: true,
		cliAuth:  true,
		createPRFn: func(opts provider.PRCreateOpts) (*provider.PRResult, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf("first branch fails")
			}
			return &provider.PRResult{Number: callCount, URL: "https://example.com/pr/" + fmt.Sprint(callCount), Action: "created"}, nil
		},
	}

	submitDraft = false
	submitNoPush = false
	defer func() { submitDraft = false; submitNoPush = false }()

	// Should not error overall - just print failures inline
	if err := submitStackBranches(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitStackBranches with failing branch failed: %v", err)
	}
}

func TestSubmitStackBranches_NoPath(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create a branch not in the current path
	tr.createBranch(t, "feat-other", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Build stack from main
	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	// Stack submit from trunk - should succeed with 0 branches
	if err := submitStackBranches(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitStackBranches no path failed: %v", err)
	}
}

func TestSubmitCurrentBranch_NotTracked(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Create an untracked branch
	if err := tr.repo.CreateBranch("untracked-sub"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-sub"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{name: "mock", cliAvail: true, cliAuth: true}

	err = submitCurrentBranch(tr.repo, tr.metadata, mock, s)
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}

func TestSubmitCurrentBranch_WithExistingPR(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-existing-pr", "main")
	tr.commitFile(t, "existing.txt", "data", "existing commit")

	s, err := stack.BuildStack(tr.repo, tr.cfg, tr.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	mock := &mockProvider{
		name:     "mock",
		cliAvail: true,
		cliAuth:  true,
		findPRFn: func(head string) (*provider.PRResult, error) {
			return &provider.PRResult{Number: 99, URL: "https://example.com/pr/99", Action: "found"}, nil
		},
		createPRFn: func(opts provider.PRCreateOpts) (*provider.PRResult, error) {
			return &provider.PRResult{Number: 99, URL: "https://example.com/pr/99", Action: "updated"}, nil
		},
	}

	submitNoPush = false
	defer func() { submitNoPush = false }()

	if err := submitCurrentBranch(tr.repo, tr.metadata, mock, s); err != nil {
		t.Fatalf("submitCurrentBranch existing PR failed: %v", err)
	}
}

func TestPrintSubmitResult_AllActions(t *testing.T) {
	// Test all action paths
	for _, action := range []string{"created", "updated", "skipped"} {
		r := &submit.Result{
			Branch:   "feat-x",
			Parent:   "main",
			PRNumber: 10,
			PRURL:    "https://example.com/pr/10",
			Action:   action,
			Provider: "github",
		}
		printSubmitResult(r)
	}
}

