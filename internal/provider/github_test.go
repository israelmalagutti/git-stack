package provider

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }
func strPtr(s string) *string { return &s }

// newTestGitHub creates a GitHubProvider wired for testing with
// CLIAvailable=true, CLIAuthenticated=true, and the given runGH mock.
func newTestGitHub(runGH func(args ...string) (string, error)) *GitHubProvider {
	return &GitHubProvider{
		host:                     "github.com",
		owner:                    "testorg",
		repo:                     "testrepo",
		RunGHOverride:            runGH,
		CLIAvailableOverride:     boolPtr(true),
		CLIAuthenticatedOverride: boolPtr(true),
	}
}

// --- Name / repoFlag (kept from original) ---

func TestGitHubProviderName(t *testing.T) {
	g := NewGitHubProvider("github.com", "user", "repo")
	if g.Name() != "github" {
		t.Errorf("expected 'github', got %q", g.Name())
	}
}

func TestGitHubRepoFlag(t *testing.T) {
	g := NewGitHubProvider("github.com", "myorg", "myrepo")
	if g.repoFlag() != "myorg/myrepo" {
		t.Errorf("expected 'myorg/myrepo', got %q", g.repoFlag())
	}
}

// --- extractPRNumber ---

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{"https://github.com/user/repo/pull/42", 42},
		{"https://github.com/user/repo/pull/1", 1},
		{"https://github.com/user/repo/pull/999", 999},
		{"not-a-url", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := extractPRNumber(tt.url)
		if got != tt.want {
			t.Errorf("extractPRNumber(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

// --- mapReviewDecision ---

func TestMapReviewDecision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"APPROVED", "approved"},
		{"CHANGES_REQUESTED", "changes_requested"},
		{"REVIEW_REQUIRED", "pending"},
		{"", ""},
		{"UNKNOWN", ""},
	}

	for _, tt := range tests {
		got := mapReviewDecision(tt.input)
		if got != tt.want {
			t.Errorf("mapReviewDecision(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- mapCIStatus ---

func TestMapCIStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []ghCheckStatus
		want   string
	}{
		{"empty", nil, ""},
		{"all pass", []ghCheckStatus{{"SUCCESS"}, {"SUCCESS"}}, "pass"},
		{"one fail", []ghCheckStatus{{"SUCCESS"}, {"FAILURE"}}, "fail"},
		{"one error", []ghCheckStatus{{"ERROR"}}, "fail"},
		{"pending", []ghCheckStatus{{"SUCCESS"}, {"PENDING"}}, "pending"},
		{"in progress", []ghCheckStatus{{"IN_PROGRESS"}}, "pending"},
		{"queued", []ghCheckStatus{{"QUEUED"}}, "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCIStatus(tt.checks)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- CLIAvailable ---

func TestGitHubCLIAvailable(t *testing.T) {
	// This test checks that CLIAvailable doesn't panic.
	// Whether gh is installed depends on the environment.
	g := NewGitHubProvider("github.com", "user", "repo")
	_ = g.CLIAvailable() // just ensure no panic
}

func TestGitHubCLIAvailableOverride(t *testing.T) {
	g := NewGitHubProvider("github.com", "user", "repo")
	g.CLIAvailableOverride = boolPtr(true)
	if !g.CLIAvailable() {
		t.Error("expected true with override")
	}

	g.CLIAvailableOverride = boolPtr(false)
	if g.CLIAvailable() {
		t.Error("expected false with override")
	}
}

// --- CLIAuthenticated ---

func TestGitHubCLIAuthenticatedOverride(t *testing.T) {
	g := NewGitHubProvider("github.com", "user", "repo")

	g.CLIAuthenticatedOverride = boolPtr(true)
	if !g.CLIAuthenticated() {
		t.Error("expected true with override")
	}

	g.CLIAuthenticatedOverride = boolPtr(false)
	if g.CLIAuthenticated() {
		t.Error("expected false with override")
	}
}

// --- runGH ---

func TestRunGHOverride(t *testing.T) {
	var captured []string
	g := newTestGitHub(func(args ...string) (string, error) {
		captured = args
		return "ok", nil
	})

	out, err := g.runGH("pr", "list", "--repo", "foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Errorf("expected 'ok', got %q", out)
	}
	if len(captured) != 4 || captured[0] != "pr" {
		t.Errorf("unexpected args: %v", captured)
	}
}

func TestRunGHOverrideError(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("boom")
	})

	_, err := g.runGH("pr", "create")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected boom error, got: %v", err)
	}
}

// --- CreatePR ---

func TestCreatePR_Success(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "https://github.com/testorg/testrepo/pull/42", nil
	})

	result, err := g.CreatePR(PRCreateOpts{
		Base:  "main",
		Head:  "feat/auth",
		Title: "Add auth",
		Body:  "This adds authentication.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Number != 42 {
		t.Errorf("expected PR #42, got #%d", result.Number)
	}
	if result.URL != "https://github.com/testorg/testrepo/pull/42" {
		t.Errorf("unexpected URL: %s", result.URL)
	}
	if result.Action != "created" {
		t.Errorf("expected action 'created', got %q", result.Action)
	}

	// Verify args contain expected flags
	argsStr := strings.Join(capturedArgs, " ")
	for _, want := range []string{"pr", "create", "--base", "main", "--head", "feat/auth", "--title", "Add auth", "--body", "This adds authentication."} {
		if !strings.Contains(argsStr, want) {
			t.Errorf("expected args to contain %q, got: %v", want, capturedArgs)
		}
	}
}

func TestCreatePR_Draft(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "https://github.com/testorg/testrepo/pull/5", nil
	})

	_, err := g.CreatePR(PRCreateOpts{
		Base:  "main",
		Head:  "feat/wip",
		Title: "WIP",
		Draft: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--draft") {
		t.Errorf("expected --draft flag, got: %v", capturedArgs)
	}
}

func TestCreatePR_EmptyBody(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "https://github.com/testorg/testrepo/pull/1", nil
	})

	_, err := g.CreatePR(PRCreateOpts{
		Base:  "main",
		Head:  "feat/x",
		Title: "X",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When Body is empty, it should still pass --body ""
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--body") {
		t.Errorf("expected --body flag even with empty body, got: %v", capturedArgs)
	}
}

func TestCreatePR_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	_, err := g.CreatePR(PRCreateOpts{Base: "main", Head: "feat/x", Title: "X"})
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestCreatePR_NotAuthenticated(t *testing.T) {
	g := &GitHubProvider{
		host:                     "github.com",
		owner:                    "org",
		repo:                     "repo",
		CLIAvailableOverride:     boolPtr(true),
		CLIAuthenticatedOverride: boolPtr(false),
	}

	_, err := g.CreatePR(PRCreateOpts{Base: "main", Head: "feat/x", Title: "X"})
	if !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("expected ErrNotAuthenticated, got: %v", err)
	}
}

func TestCreatePR_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("gh error: rate limited")
	})

	_, err := g.CreatePR(PRCreateOpts{Base: "main", Head: "feat/x", Title: "X"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create PR") {
		t.Errorf("expected 'failed to create PR' in error, got: %v", err)
	}
}

// --- UpdatePR ---

func TestUpdatePR_Success(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	title := "New Title"
	err := g.UpdatePR(42, PRUpdateOpts{Title: &title})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "pr edit 42") {
		t.Errorf("expected 'pr edit 42', got: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "--title") {
		t.Errorf("expected --title flag, got: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "New Title") {
		t.Errorf("expected title value, got: %v", capturedArgs)
	}
}

func TestUpdatePR_WithBody(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	body := "Updated body content"
	err := g.UpdatePR(10, PRUpdateOpts{Body: &body})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--body") {
		t.Errorf("expected --body flag, got: %v", capturedArgs)
	}
}

func TestUpdatePR_WithTitleAndBody(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	title := "T"
	body := "B"
	err := g.UpdatePR(7, PRUpdateOpts{Title: &title, Body: &body})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--title") || !strings.Contains(argsStr, "--body") {
		t.Errorf("expected --title and --body flags, got: %v", capturedArgs)
	}
}

func TestUpdatePR_NoOpts(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", nil
	})

	// No title or body set - should still succeed (edit with no changes)
	err := g.UpdatePR(1, PRUpdateOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdatePR_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	err := g.UpdatePR(1, PRUpdateOpts{})
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestUpdatePR_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("not found")
	})

	err := g.UpdatePR(999, PRUpdateOpts{Title: strPtr("X")})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- GetPRStatus ---

func TestGetPRStatus_Success(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return `{
			"number": 42,
			"state": "OPEN",
			"title": "Add auth",
			"url": "https://github.com/testorg/testrepo/pull/42",
			"isDraft": false,
			"reviewDecision": "APPROVED",
			"mergeable": "MERGEABLE",
			"statusCheckRollup": [{"state": "SUCCESS"}]
		}`, nil
	})

	status, err := g.GetPRStatus(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Number != 42 {
		t.Errorf("expected number 42, got %d", status.Number)
	}
	if status.State != "open" {
		t.Errorf("expected state 'open', got %q", status.State)
	}
	if status.Title != "Add auth" {
		t.Errorf("expected title 'Add auth', got %q", status.Title)
	}
	if status.Draft {
		t.Error("expected draft=false")
	}
	if status.ReviewStatus != "approved" {
		t.Errorf("expected review 'approved', got %q", status.ReviewStatus)
	}
	if status.CIStatus != "pass" {
		t.Errorf("expected CI 'pass', got %q", status.CIStatus)
	}
	if !status.Mergeable {
		t.Error("expected mergeable=true")
	}
}

func TestGetPRStatus_DraftPR(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return `{
			"number": 7,
			"state": "OPEN",
			"title": "WIP",
			"url": "https://github.com/testorg/testrepo/pull/7",
			"isDraft": true,
			"reviewDecision": "",
			"mergeable": "UNKNOWN",
			"statusCheckRollup": []
		}`, nil
	})

	status, err := g.GetPRStatus(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Draft {
		t.Error("expected draft=true")
	}
	if status.Mergeable {
		t.Error("expected mergeable=false for UNKNOWN")
	}
}

func TestGetPRStatus_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	_, err := g.GetPRStatus(1)
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestGetPRStatus_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("not found")
	})

	_, err := g.GetPRStatus(999)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get PR status") {
		t.Errorf("expected 'failed to get PR status', got: %v", err)
	}
}

func TestGetPRStatus_InvalidJSON(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "not json", nil
	})

	_, err := g.GetPRStatus(1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to parse PR status") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// --- MergePR ---

func TestMergePR_DefaultMethod(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	err := g.MergePR(42, PRMergeOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--merge") {
		t.Errorf("expected --merge flag for default, got: %v", capturedArgs)
	}
}

func TestMergePR_Squash(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	err := g.MergePR(42, PRMergeOpts{Method: "squash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--squash") {
		t.Errorf("expected --squash flag, got: %v", capturedArgs)
	}
}

func TestMergePR_Rebase(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	err := g.MergePR(42, PRMergeOpts{Method: "rebase"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--rebase") {
		t.Errorf("expected --rebase flag, got: %v", capturedArgs)
	}
}

func TestMergePR_Auto(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	err := g.MergePR(42, PRMergeOpts{Auto: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--auto") {
		t.Errorf("expected --auto flag, got: %v", capturedArgs)
	}
}

func TestMergePR_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	err := g.MergePR(1, PRMergeOpts{})
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestMergePR_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("merge conflict")
	})

	err := g.MergePR(42, PRMergeOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- UpdatePRBase ---

func TestUpdatePRBase_Success(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	err := g.UpdatePRBase(42, "develop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "pr edit 42") {
		t.Errorf("expected 'pr edit 42', got: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "--base") || !strings.Contains(argsStr, "develop") {
		t.Errorf("expected --base develop, got: %v", capturedArgs)
	}
}

func TestUpdatePRBase_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	err := g.UpdatePRBase(1, "main")
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestUpdatePRBase_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("not found")
	})

	err := g.UpdatePRBase(999, "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- FindExistingPR ---

func TestFindExistingPR_Found(t *testing.T) {
	var capturedArgs []string
	g := newTestGitHub(func(args ...string) (string, error) {
		capturedArgs = args
		return `[{"number": 10, "url": "https://github.com/testorg/testrepo/pull/10"}]`, nil
	})

	result, err := g.FindExistingPR("feat/auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Number != 10 {
		t.Errorf("expected PR #10, got #%d", result.Number)
	}
	if result.URL != "https://github.com/testorg/testrepo/pull/10" {
		t.Errorf("unexpected URL: %s", result.URL)
	}
	if result.Action != "updated" {
		t.Errorf("expected action 'updated', got %q", result.Action)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--head") || !strings.Contains(argsStr, "feat/auth") {
		t.Errorf("expected --head feat/auth in args, got: %v", capturedArgs)
	}
}

func TestFindExistingPR_NotFound(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return `[]`, nil
	})

	result, err := g.FindExistingPR("feat/new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for no existing PR, got: %v", result)
	}
}

func TestFindExistingPR_CLINotAvailable(t *testing.T) {
	g := &GitHubProvider{
		host:                 "github.com",
		owner:                "org",
		repo:                 "repo",
		CLIAvailableOverride: boolPtr(false),
	}

	_, err := g.FindExistingPR("feat/x")
	if !errors.Is(err, ErrCLINotFound) {
		t.Errorf("expected ErrCLINotFound, got: %v", err)
	}
}

func TestFindExistingPR_GHFailure(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "", fmt.Errorf("network error")
	})

	_, err := g.FindExistingPR("feat/x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to check for existing PR") {
		t.Errorf("expected 'failed to check for existing PR', got: %v", err)
	}
}

func TestFindExistingPR_InvalidJSON(t *testing.T) {
	g := newTestGitHub(func(args ...string) (string, error) {
		return "not json", nil
	})

	_, err := g.FindExistingPR("feat/x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to parse PR list") {
		t.Errorf("expected parse error, got: %v", err)
	}
}
