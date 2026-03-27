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

// TestIntegration_FullStackWorkflow tests a complete workflow:
// create stack → navigate → modify → restack → fold
func TestIntegration_FullStackWorkflow(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create a 3-branch stack: main → feat/auth → feat/auth-tests → feat/auth-e2e
	createAndVerify(t, ctx, "feat/auth")
	stageAndCommit(t, "auth.go", "auth code")

	createAndVerify(t, ctx, "feat/auth-tests")
	stageAndCommit(t, "auth_test.go", "auth tests")

	createAndVerify(t, ctx, "feat/auth-e2e")
	stageAndCommit(t, "auth_e2e_test.go", "e2e tests")

	// 2. Verify status shows full tree
	statusResult, _ := handleStatus(ctx, mcp.CallToolRequest{})
	statusText := statusResult.Content[0].(mcp.TextContent).Text
	var status statusResponse
	json.Unmarshal([]byte(statusText), &status)

	if len(status.Branches) != 4 { // main + 3 branches
		t.Fatalf("expected 4 branches, got %d", len(status.Branches))
	}
	if status.CurrentBranch != "feat/auth-e2e" {
		t.Errorf("expected current 'feat/auth-e2e', got '%s'", status.CurrentBranch)
	}

	// 3. Navigate down to main
	navResult, _ := handleNavigate(ctx, makeRequest("gs_navigate", map[string]any{"direction": "bottom"}))
	navText := navResult.Content[0].(mcp.TextContent).Text
	var nav navigateResponse
	json.Unmarshal([]byte(navText), &nav)

	if nav.CurrentBranch != "main" {
		t.Errorf("expected bottom = 'main', got '%s'", nav.CurrentBranch)
	}
	if nav.StepsTaken != 3 {
		t.Errorf("expected 3 steps to bottom, got %d", nav.StepsTaken)
	}

	// 4. Navigate to top (should go to feat/auth-e2e)
	topResult, _ := handleNavigate(ctx, makeRequest("gs_navigate", map[string]any{"direction": "top"}))
	topText := topResult.Content[0].(mcp.TextContent).Text
	var top navigateResponse
	json.Unmarshal([]byte(topText), &top)

	if top.CurrentBranch != "feat/auth-e2e" {
		t.Errorf("expected top = 'feat/auth-e2e', got '%s'", top.CurrentBranch)
	}

	// 5. Go to feat/auth and modify it (amend commit message)
	handleCheckout(ctx, makeRequest("gs_checkout", map[string]any{"branch": "feat/auth"}))

	modResult, _ := handleModify(ctx, makeRequest("gs_modify", map[string]any{
		"message": "improved auth code",
	}))
	modText := modResult.Content[0].(mcp.TextContent).Text
	var mod modifyResponse
	json.Unmarshal([]byte(modText), &mod)

	if mod.Action != "amended" {
		t.Errorf("expected action 'amended', got '%s'", mod.Action)
	}

	// 6. Verify branch info shows correct state
	infoResult, _ := handleBranchInfo(ctx, makeRequest("gs_branch_info", map[string]any{"branch": "feat/auth"}))
	infoText := infoResult.Content[0].(mcp.TextContent).Text
	var info branchInfoResponse
	json.Unmarshal([]byte(infoText), &info)

	if info.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", info.Parent)
	}
	if len(info.Children) != 1 || info.Children[0] != "feat/auth-tests" {
		t.Errorf("expected children [feat/auth-tests], got %v", info.Children)
	}

	// 7. Get diff for feat/auth
	diffResult, _ := handleDiff(ctx, makeRequest("gs_diff", map[string]any{"branch": "feat/auth"}))
	diffText := diffResult.Content[0].(mcp.TextContent).Text
	var diff diffResponse
	json.Unmarshal([]byte(diffText), &diff)

	if diff.Diff == "" {
		t.Error("expected non-empty diff for feat/auth")
	}

	// 8. Get structured log
	logResult, _ := handleLog(ctx, makeRequest("gs_log", map[string]any{"include_commits": true}))
	logText := logResult.Content[0].(mcp.TextContent).Text
	var log logResponse
	json.Unmarshal([]byte(logText), &log)

	if len(log.Branches) != 4 {
		t.Errorf("expected 4 branches in log, got %d", len(log.Branches))
	}

	// Verify commits are included
	for _, b := range log.Branches {
		if b.Name == "feat/auth" && len(b.Commits) == 0 {
			t.Error("expected commits for feat/auth in log with include_commits=true")
		}
	}
}

// TestIntegration_CreateNavigateDeleteFlow tests create → navigate → delete with reparenting
func TestIntegration_CreateNavigateDeleteFlow(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create: main → A → B
	createAndVerify(t, ctx, "feat/A")
	stageAndCommit(t, "a.txt", "A content")
	createAndVerify(t, ctx, "feat/B")
	stageAndCommit(t, "b.txt", "B content")

	// Navigate to main
	handleCheckout(ctx, makeRequest("gs_checkout", map[string]any{"branch": "main"}))

	// Delete A — B should be reparented to main
	delResult, _ := handleDelete(ctx, makeRequest("gs_delete", map[string]any{"branch": "feat/A"}))
	delText := delResult.Content[0].(mcp.TextContent).Text
	var del deleteResponse
	json.Unmarshal([]byte(delText), &del)

	if len(del.ReparentedChildren) != 1 || del.ReparentedChildren[0] != "feat/B" {
		t.Errorf("expected B reparented, got %v", del.ReparentedChildren)
	}

	// Verify B's parent is now main
	infoResult, _ := handleBranchInfo(ctx, makeRequest("gs_branch_info", map[string]any{"branch": "feat/B"}))
	infoText := infoResult.Content[0].(mcp.TextContent).Text
	var info branchInfoResponse
	json.Unmarshal([]byte(infoText), &info)

	if info.Parent != "main" {
		t.Errorf("expected B's parent to be 'main' after delete, got '%s'", info.Parent)
	}
}

// TestIntegration_TrackMoveRestack tests track → move → restack
func TestIntegration_TrackMoveRestack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create two branches off main
	createAndVerify(t, ctx, "feat/auth")
	stageAndCommit(t, "auth.go", "auth")
	handleCheckout(ctx, makeRequest("gs_checkout", map[string]any{"branch": "main"}))

	createAndVerify(t, ctx, "feat/ui")
	stageAndCommit(t, "ui.go", "ui")

	// Move feat/ui onto feat/auth
	moveResult, _ := handleMove(ctx, makeRequest("gs_move", map[string]any{"branch": "feat/ui", "onto": "feat/auth"}))
	moveText := moveResult.Content[0].(mcp.TextContent).Text
	var mv moveResponse
	json.Unmarshal([]byte(moveText), &mv)

	if mv.NewParent != "feat/auth" {
		t.Errorf("expected new parent 'feat/auth', got '%s'", mv.NewParent)
	}

	// Verify via status
	statusResult, _ := handleStatus(ctx, mcp.CallToolRequest{})
	statusText := statusResult.Content[0].(mcp.TextContent).Text
	var status statusResponse
	json.Unmarshal([]byte(statusText), &status)

	for _, b := range status.Branches {
		if b.Name == "feat/ui" && b.Parent != "feat/auth" {
			t.Errorf("expected feat/ui parent 'feat/auth', got '%s'", b.Parent)
		}
	}
}

// TestIntegration_FoldWorkflow tests create → fold → verify merge
func TestIntegration_FoldWorkflow(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create: main → feat/auth → feat/auth-tests
	createAndVerify(t, ctx, "feat/auth")
	stageAndCommit(t, "auth.go", "auth code")
	createAndVerify(t, ctx, "feat/auth-tests")
	stageAndCommit(t, "auth_test.go", "test code")

	// Go to feat/auth and fold it into main
	handleCheckout(ctx, makeRequest("gs_checkout", map[string]any{"branch": "feat/auth"}))

	foldResult, _ := handleFold(ctx, makeRequest("gs_fold", map[string]any{}))
	foldText := foldResult.Content[0].(mcp.TextContent).Text
	var fold foldResponse
	json.Unmarshal([]byte(foldText), &fold)

	if fold.Folded != "feat/auth" {
		t.Errorf("expected folded 'feat/auth', got '%s'", fold.Folded)
	}
	if fold.Into != "main" {
		t.Errorf("expected into 'main', got '%s'", fold.Into)
	}
	if len(fold.Reparented) != 1 || fold.Reparented[0] != "feat/auth-tests" {
		t.Errorf("expected [feat/auth-tests] reparented, got %v", fold.Reparented)
	}

	// Verify status: should be main → feat/auth-tests (feat/auth is gone)
	statusResult, _ := handleStatus(ctx, mcp.CallToolRequest{})
	statusText := statusResult.Content[0].(mcp.TextContent).Text
	var status statusResponse
	json.Unmarshal([]byte(statusText), &status)

	if len(status.Branches) != 2 {
		t.Errorf("expected 2 branches after fold, got %d", len(status.Branches))
	}

	for _, b := range status.Branches {
		if b.Name == "feat/auth" {
			t.Error("feat/auth should not appear after fold")
		}
	}
}

// TestIntegration_RenamePreservesChildren tests rename with children
func TestIntegration_RenamePreservesChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	createAndVerify(t, ctx, "feat/old")
	stageAndCommit(t, "old.txt", "content")
	createAndVerify(t, ctx, "feat/child")
	stageAndCommit(t, "child.txt", "child content")

	// Go to feat/old and rename it
	handleCheckout(ctx, makeRequest("gs_checkout", map[string]any{"branch": "feat/old"}))

	renResult, _ := handleRename(ctx, makeRequest("gs_rename", map[string]any{"new_name": "feat/new"}))
	renText := renResult.Content[0].(mcp.TextContent).Text
	var ren renameResponse
	json.Unmarshal([]byte(renText), &ren)

	if ren.NewName != "feat/new" {
		t.Errorf("expected new name 'feat/new', got '%s'", ren.NewName)
	}

	// Verify child still points to renamed parent
	infoResult, _ := handleBranchInfo(ctx, makeRequest("gs_branch_info", map[string]any{"branch": "feat/child"}))
	infoText := infoResult.Content[0].(mcp.TextContent).Text
	var info branchInfoResponse
	json.Unmarshal([]byte(infoText), &info)

	if info.Parent != "feat/new" {
		t.Errorf("expected child parent 'feat/new', got '%s'", info.Parent)
	}
}

// --- helpers ---

func createAndVerify(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	result, err := handleCreate(ctx, makeRequest("gs_create", map[string]any{"name": name}))
	if err != nil {
		t.Fatalf("failed to create %s: %v", name, err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("create %s error: %s", name, text)
	}
}

func stageAndCommit(t *testing.T, filename, content string) {
	t.Helper()
	cwd, _ := os.Getwd()
	if err := os.WriteFile(filepath.Join(cwd, filename), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
	exec.Command("git", "add", filename).Run()
	exec.Command("git", "commit", "-m", "add "+filename).Run()
}
