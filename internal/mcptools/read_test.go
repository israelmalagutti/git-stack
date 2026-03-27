package mcptools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// setupTestRepo creates a temp git repo with gs initialized and returns a cleanup function.
// The working directory is changed to the temp repo.
func setupTestRepo(t *testing.T) func() {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gs-mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to get current dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to chdir: %v", err)
	}

	// Initialize git repo
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			os.Chdir(origDir)
			os.RemoveAll(tmpDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	// Create initial commit on main
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644); err != nil {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write file: %v", err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
		{"branch", "-M", "main"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			os.Chdir(origDir)
			os.RemoveAll(tmpDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	// Initialize gs config
	gitDir := filepath.Join(tmpDir, ".git")
	configData := `{"version":"1.0.0","trunk":"main"}`
	metadataData := `{"branches":{}}`
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_config"), []byte(configData), 0600)
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_stack_metadata"), []byte(metadataData), 0600)

	return func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}
}

// addTrackedBranch creates a git branch and tracks it in gs metadata.
func addTrackedBranch(t *testing.T, name, parent string) {
	t.Helper()

	// Create and checkout branch
	if err := exec.Command("git", "checkout", "-b", name).Run(); err != nil {
		t.Fatalf("git checkout -b %s failed: %v", name, err)
	}

	// Create a commit on the branch using a safe filename (replace / with _)
	safeFilename := filepath.Base(name) + ".txt"
	cwd, _ := os.Getwd()
	fullPath := filepath.Join(cwd, safeFilename)
	if err := os.WriteFile(fullPath, []byte("content of "+name), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", fullPath, err)
	}
	_ = exec.Command("git", "add", safeFilename).Run()
	_ = exec.Command("git", "commit", "-m", "commit on "+name).Run()

	// Update metadata
	gitDirOut, _ := exec.Command("git", "rev-parse", "--git-dir").Output()
	gitDir := filepath.Clean(string(gitDirOut[:len(gitDirOut)-1]))
	metadataPath := filepath.Join(gitDir, ".gs_stack_metadata")
	data, _ := os.ReadFile(metadataPath)

	var meta map[string]any
	_ = json.Unmarshal(data, &meta)
	branches := meta["branches"].(map[string]any)
	branches[name] = map[string]any{
		"parent":  parent,
		"tracked": true,
		"created": "2026-01-01T00:00:00Z",
	}
	updated, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metadataPath, updated, 0600)
}

func TestHandleStatus_EmptyStack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := mcp.CallToolRequest{}
	result, err := handleStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStatus returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleStatus returned tool error: %v", result.Content)
	}

	// Parse response
	text := result.Content[0].(mcp.TextContent).Text
	var resp statusResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Trunk != "main" {
		t.Errorf("expected trunk 'main', got '%s'", resp.Trunk)
	}
	if resp.CurrentBranch != "main" {
		t.Errorf("expected current branch 'main', got '%s'", resp.CurrentBranch)
	}
	if !resp.Initialized {
		t.Error("expected initialized to be true")
	}
	if len(resp.Branches) != 1 {
		t.Errorf("expected 1 branch (trunk only), got %d", len(resp.Branches))
	}
	if resp.Branches[0].Name != "main" {
		t.Errorf("expected branch name 'main', got '%s'", resp.Branches[0].Name)
	}
	if !resp.Branches[0].IsTrunk {
		t.Error("expected trunk branch to have is_trunk=true")
	}
}

func TestHandleStatus_WithBranches(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a tracked branch
	addTrackedBranch(t, "feat/auth", "main")

	// Switch back to main for a second branch
	exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/ui", "main")

	req := mcp.CallToolRequest{}
	result, err := handleStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStatus returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp statusResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/ui" {
		t.Errorf("expected current branch 'feat/ui', got '%s'", resp.CurrentBranch)
	}
	if len(resp.Branches) != 3 {
		t.Errorf("expected 3 branches, got %d", len(resp.Branches))
	}

	// Find feat/auth in response
	found := false
	for _, b := range resp.Branches {
		if b.Name == "feat/auth" {
			found = true
			if b.Parent != "main" {
				t.Errorf("expected feat/auth parent 'main', got '%s'", b.Parent)
			}
			if b.Depth != 1 {
				t.Errorf("expected feat/auth depth 1, got %d", b.Depth)
			}
			if b.IsTrunk {
				t.Error("feat/auth should not be trunk")
			}
		}
	}
	if !found {
		t.Error("feat/auth not found in response")
	}
}

func TestHandleStatus_NotInitialized(t *testing.T) {
	// Create a git repo without gs init
	tmpDir, err := os.MkdirTemp("", "gs-mcp-test-noinit-*")
	if err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	req := mcp.CallToolRequest{}
	result, err := handleStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStatus returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result when gs is not initialized")
	}
}

// --- gs_branch_info tests ---

func makeRequest(name string, args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

func TestHandleBranchInfo_TrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_branch_info", map[string]any{"branch": "feat/auth"})
	result, err := handleBranchInfo(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBranchInfo returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleBranchInfo returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp branchInfoResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.Name != "feat/auth" {
		t.Errorf("expected name 'feat/auth', got '%s'", resp.Name)
	}
	if resp.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", resp.Parent)
	}
	if resp.Depth != 1 {
		t.Errorf("expected depth 1, got %d", resp.Depth)
	}
	if resp.IsTrunk {
		t.Error("should not be trunk")
	}
	if len(resp.Commits) == 0 {
		t.Error("expected at least one commit")
	}
	if resp.Commits[0].Message != "commit on feat/auth" {
		t.Errorf("unexpected commit message: %s", resp.Commits[0].Message)
	}
}

func TestHandleBranchInfo_TrunkBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_branch_info", map[string]any{"branch": "main"})
	result, err := handleBranchInfo(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBranchInfo returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp branchInfoResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.Name != "main" {
		t.Errorf("expected name 'main', got '%s'", resp.Name)
	}
	if !resp.IsTrunk {
		t.Error("expected is_trunk=true for main")
	}
	if resp.Depth != 0 {
		t.Errorf("expected depth 0 for trunk, got %d", resp.Depth)
	}
}

func TestHandleBranchInfo_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch in git but don't track it in gs
	exec.Command("git", "checkout", "-b", "untracked-branch").Run()

	req := makeRequest("gs_branch_info", map[string]any{"branch": "untracked-branch"})
	result, err := handleBranchInfo(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBranchInfo returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

func TestHandleBranchInfo_WithChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")

	req := makeRequest("gs_branch_info", map[string]any{"branch": "feat/auth"})
	result, err := handleBranchInfo(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBranchInfo returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp branchInfoResponse
	json.Unmarshal([]byte(text), &resp)

	if len(resp.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(resp.Children))
	}
	if resp.Children[0] != "feat/auth-tests" {
		t.Errorf("expected child 'feat/auth-tests', got '%s'", resp.Children[0])
	}
}

// --- gs_log tests ---

func TestHandleLog_BasicTree(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := mcp.CallToolRequest{}
	result, err := handleLog(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLog returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp logResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.Trunk != "main" {
		t.Errorf("expected trunk 'main', got '%s'", resp.Trunk)
	}
	if len(resp.Branches) != 2 {
		t.Errorf("expected 2 branches, got %d", len(resp.Branches))
	}

	// By default, commits should not be included
	for _, b := range resp.Branches {
		if len(b.Commits) > 0 {
			t.Errorf("expected no commits without include_commits, got %d for %s", len(b.Commits), b.Name)
		}
	}
}

func TestHandleLog_WithCommits(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_log", map[string]any{"include_commits": true})
	result, err := handleLog(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLog returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp logResponse
	json.Unmarshal([]byte(text), &resp)

	// Find feat/auth and check it has commits
	for _, b := range resp.Branches {
		if b.Name == "feat/auth" {
			if len(b.Commits) == 0 {
				t.Error("expected commits for feat/auth with include_commits=true")
			}
			return
		}
	}
	t.Error("feat/auth not found in response")
}

// --- gs_diff tests ---

func TestHandleDiff_BranchWithChanges(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_diff", map[string]any{"branch": "feat/auth"})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleDiff returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp diffResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/auth" {
		t.Errorf("expected branch 'feat/auth', got '%s'", resp.Branch)
	}
	if resp.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", resp.Parent)
	}
	if resp.Diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestHandleDiff_DefaultsToCurrent(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	// feat/auth is now the current branch

	req := makeRequest("gs_diff", map[string]any{})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp diffResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/auth" {
		t.Errorf("expected default to current branch 'feat/auth', got '%s'", resp.Branch)
	}
}

func TestHandleDiff_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_diff", map[string]any{"branch": "main"})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when diffing trunk (no parent)")
	}
}
