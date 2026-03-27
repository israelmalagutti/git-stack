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

// --- gs_checkout tests ---

func TestHandleCheckout_SwitchBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_checkout", map[string]any{"branch": "feat/auth"})
	result, err := handleCheckout(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckout returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleCheckout returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp checkoutResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.PreviousBranch != "main" {
		t.Errorf("expected previous branch 'main', got '%s'", resp.PreviousBranch)
	}
	if resp.CurrentBranch != "feat/auth" {
		t.Errorf("expected current branch 'feat/auth', got '%s'", resp.CurrentBranch)
	}
}

func TestHandleCheckout_NonexistentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_checkout", map[string]any{"branch": "nonexistent"})
	result, err := handleCheckout(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckout returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent branch")
	}
}

// --- gs_navigate tests ---

func TestHandleNavigate_UpSingleChild(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "up"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleNavigate returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/auth" {
		t.Errorf("expected current branch 'feat/auth', got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 1 {
		t.Errorf("expected 1 step, got %d", resp.StepsTaken)
	}
}

func TestHandleNavigate_UpAmbiguous(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/ui", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "up"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	// Should not be an error — it returns ambiguous options
	if result.IsError {
		t.Fatal("expected non-error ambiguous response")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateAmbiguousResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Error != "ambiguous_navigation" {
		t.Errorf("expected ambiguous_navigation error, got '%s'", resp.Error)
	}
	if len(resp.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(resp.Options))
	}
}

func TestHandleNavigate_Down(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	// Currently on feat/auth

	req := makeRequest("gs_navigate", map[string]any{"direction": "down"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "main" {
		t.Errorf("expected current branch 'main', got '%s'", resp.CurrentBranch)
	}
}

func TestHandleNavigate_DownAtTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_navigate", map[string]any{"direction": "down"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when navigating down from trunk")
	}
}

func TestHandleNavigate_Top(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "top"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/auth-tests" {
		t.Errorf("expected top to be 'feat/auth-tests', got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 2 {
		t.Errorf("expected 2 steps, got %d", resp.StepsTaken)
	}
}

func TestHandleNavigate_Bottom(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")
	// Currently on feat/auth-tests

	req := makeRequest("gs_navigate", map[string]any{"direction": "bottom"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "main" {
		t.Errorf("expected bottom to be 'main', got '%s'", resp.CurrentBranch)
	}
}

func TestHandleNavigate_UpMultipleSteps(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "up", "steps": 2})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/auth-tests" {
		t.Errorf("expected 'feat/auth-tests' after 2 steps up, got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 2 {
		t.Errorf("expected 2 steps, got %d", resp.StepsTaken)
	}
}

// --- gs_create tests ---

func TestHandleCreate_NewBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_create", map[string]any{"name": "feat/new"})
	result, err := handleCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCreate returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleCreate returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp createResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/new" {
		t.Errorf("expected branch 'feat/new', got '%s'", resp.Branch)
	}
	if resp.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", resp.Parent)
	}
	if resp.CommitCreated {
		t.Error("expected no commit created without message")
	}

	// Verify we're on the new branch
	out, _ := exec.Command("git", "branch", "--show-current").Output()
	current := string(out[:len(out)-1])
	if current != "feat/new" {
		t.Errorf("expected current branch 'feat/new', got '%s'", current)
	}
}

func TestHandleCreate_WithCommitMessage(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Stage a change first
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "new-file.txt"), []byte("hello"), 0644)
	_ = exec.Command("git", "add", "new-file.txt").Run()

	req := makeRequest("gs_create", map[string]any{
		"name":           "feat/with-commit",
		"commit_message": "add new file",
	})
	result, err := handleCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCreate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp createResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if !resp.CommitCreated {
		t.Error("expected commit to be created")
	}
}

func TestHandleCreate_DuplicateName(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/existing", "main")

	req := makeRequest("gs_create", map[string]any{"name": "feat/existing"})
	result, err := handleCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCreate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for duplicate branch name")
	}
}

// --- gs_delete tests ---

func TestHandleDelete_SimpleBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/to-delete", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_delete", map[string]any{"branch": "feat/to-delete"})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleDelete returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp deleteResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Deleted != "feat/to-delete" {
		t.Errorf("expected deleted 'feat/to-delete', got '%s'", resp.Deleted)
	}
	if resp.NewParent != "main" {
		t.Errorf("expected new parent 'main', got '%s'", resp.NewParent)
	}
}

func TestHandleDelete_WithChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_delete", map[string]any{"branch": "feat/parent"})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp deleteResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.ReparentedChildren) != 1 {
		t.Fatalf("expected 1 reparented child, got %d", len(resp.ReparentedChildren))
	}
	if resp.ReparentedChildren[0] != "feat/child" {
		t.Errorf("expected reparented child 'feat/child', got '%s'", resp.ReparentedChildren[0])
	}
}

func TestHandleDelete_CurrentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/current", "main")
	// Currently on feat/current

	req := makeRequest("gs_delete", map[string]any{"branch": "feat/current"})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp deleteResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CheckedOut != "main" {
		t.Errorf("expected checkout to 'main', got '%s'", resp.CheckedOut)
	}
}

func TestHandleDelete_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_delete", map[string]any{"branch": "main"})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when deleting trunk")
	}
}

// --- gs_track tests ---

func TestHandleTrack_ExistingBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a git branch without tracking it
	_ = exec.Command("git", "checkout", "-b", "untracked").Run()
	_ = os.WriteFile("untracked.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "untracked.txt").Run()
	_ = exec.Command("git", "commit", "-m", "untracked commit").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_track", map[string]any{"branch": "untracked", "parent": "main"})
	result, err := handleTrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTrack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleTrack returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp trackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "untracked" {
		t.Errorf("expected branch 'untracked', got '%s'", resp.Branch)
	}
	if resp.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", resp.Parent)
	}
}

func TestHandleTrack_AlreadyTracked(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/tracked", "main")

	req := makeRequest("gs_track", map[string]any{"branch": "feat/tracked", "parent": "main"})
	result, _ := handleTrack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for already tracked branch")
	}
}

// --- gs_untrack tests ---

func TestHandleUntrack_TrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/to-untrack", "main")

	req := makeRequest("gs_untrack", map[string]any{"branch": "feat/to-untrack"})
	result, err := handleUntrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleUntrack returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("handleUntrack returned tool error")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp untrackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/to-untrack" {
		t.Errorf("expected branch 'feat/to-untrack', got '%s'", resp.Branch)
	}

	// Verify branch still exists in git but not in gs status
	statusReq := mcp.CallToolRequest{}
	statusResult, _ := handleStatus(context.Background(), statusReq)
	statusText := statusResult.Content[0].(mcp.TextContent).Text
	var statusResp statusResponse
	_ = json.Unmarshal([]byte(statusText), &statusResp)

	for _, b := range statusResp.Branches {
		if b.Name == "feat/to-untrack" {
			t.Error("untracked branch should not appear in status")
		}
	}
}

func TestHandleUntrack_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_untrack", map[string]any{"branch": "main"})
	result, _ := handleUntrack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when untracking trunk")
	}
}

// --- gs_rename tests ---

func TestHandleRename_Success(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/old-name", "main")
	// Currently on feat/old-name

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/new-name"})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRename returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp renameResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.OldName != "feat/old-name" {
		t.Errorf("expected old name 'feat/old-name', got '%s'", resp.OldName)
	}
	if resp.NewName != "feat/new-name" {
		t.Errorf("expected new name 'feat/new-name', got '%s'", resp.NewName)
	}

	// Verify git branch was renamed
	out, _ := exec.Command("git", "branch", "--show-current").Output()
	current := string(out[:len(out)-1])
	if current != "feat/new-name" {
		t.Errorf("expected current branch 'feat/new-name', got '%s'", current)
	}
}

func TestHandleRename_DuplicateName(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/b", "main")
	// Currently on feat/b

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/a"})
	result, _ := handleRename(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for duplicate name")
	}
}

func TestHandleRename_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_rename", map[string]any{"new_name": "new-main"})
	result, _ := handleRename(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when renaming trunk")
	}
}

// --- gs_restack tests ---

func TestHandleRestack_AlreadyUpToDate(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_restack", map[string]any{"scope": "only"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRestack returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp restackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	// Branch was just created — should be up to date
	if len(resp.Restacked) != 0 {
		t.Errorf("expected 0 restacked, got %d", len(resp.Restacked))
	}
}

func TestHandleRestack_NeedRebase(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// Go back to main and add a commit
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("main-change.txt", []byte("change"), 0644)
	_ = exec.Command("git", "add", "main-change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main change").Run()

	// Now feat/auth is behind main
	_ = exec.Command("git", "checkout", "feat/auth").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "only"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp restackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.Restacked) != 1 {
		t.Errorf("expected 1 restacked, got %d", len(resp.Restacked))
	}
	if resp.Conflict != "" {
		t.Errorf("unexpected conflict: %s", resp.Conflict)
	}
}

func TestHandleRestack_UncommittedChangesError(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// Create uncommitted change
	_ = os.WriteFile("dirty.txt", []byte("dirty"), 0644)
	_ = exec.Command("git", "add", "dirty.txt").Run()

	req := makeRequest("gs_restack", map[string]any{})
	result, _ := handleRestack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error with uncommitted changes")
	}
}

// --- gs_modify tests ---

func TestHandleModify_AmendWithMessage(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	// Currently on feat/auth with one commit

	req := makeRequest("gs_modify", map[string]any{"message": "updated commit"})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleModify returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp modifyResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/auth" {
		t.Errorf("expected branch 'feat/auth', got '%s'", resp.Branch)
	}
	if resp.Action != "amended" {
		t.Errorf("expected action 'amended', got '%s'", resp.Action)
	}

	// Verify commit message changed
	out, _ := exec.Command("git", "log", "-1", "--format=%s").Output()
	msg := string(out[:len(out)-1])
	if msg != "updated commit" {
		t.Errorf("expected commit message 'updated commit', got '%s'", msg)
	}
}

func TestHandleModify_NewCommit(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// Add a file to stage
	_ = os.WriteFile("extra.txt", []byte("extra"), 0644)
	_ = exec.Command("git", "add", "extra.txt").Run()

	req := makeRequest("gs_modify", map[string]any{
		"message":    "new commit on branch",
		"new_commit": true,
	})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp modifyResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Action != "committed" {
		t.Errorf("expected action 'committed', got '%s'", resp.Action)
	}
}

func TestHandleModify_NewCommitRequiresMessage(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_modify", map[string]any{"new_commit": true})
	result, _ := handleModify(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when new_commit without message")
	}
}

// --- gs_move tests ---

func TestHandleMove_Success(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/ui", "main")
	// Move feat/ui onto feat/auth

	req := makeRequest("gs_move", map[string]any{"branch": "feat/ui", "onto": "feat/auth"})
	result, err := handleMove(context.Background(), req)
	if err != nil {
		t.Fatalf("handleMove returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleMove returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp moveResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/ui" {
		t.Errorf("expected branch 'feat/ui', got '%s'", resp.Branch)
	}
	if resp.OldParent != "main" {
		t.Errorf("expected old parent 'main', got '%s'", resp.OldParent)
	}
	if resp.NewParent != "feat/auth" {
		t.Errorf("expected new parent 'feat/auth', got '%s'", resp.NewParent)
	}
}

func TestHandleMove_OntoSelfErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_move", map[string]any{"onto": "feat/auth"})
	result, _ := handleMove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when moving onto self")
	}
}

func TestHandleMove_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_move", map[string]any{"branch": "main", "onto": "feat/x"})
	result, _ := handleMove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when moving trunk")
	}
}

// --- gs_fold tests ---

func TestHandleFold_Success(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	// Currently on feat/auth

	req := makeRequest("gs_fold", map[string]any{})
	result, err := handleFold(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFold returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleFold returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp foldResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Folded != "feat/auth" {
		t.Errorf("expected folded 'feat/auth', got '%s'", resp.Folded)
	}
	if resp.Into != "main" {
		t.Errorf("expected into 'main', got '%s'", resp.Into)
	}
	if resp.Kept {
		t.Error("expected kept=false by default")
	}

	// Verify branch was deleted
	out, _ := exec.Command("git", "branch", "--list", "feat/auth").Output()
	if len(out) > 0 {
		t.Error("feat/auth should have been deleted")
	}
}

func TestHandleFold_WithKeep(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	req := makeRequest("gs_fold", map[string]any{"keep": true})
	result, err := handleFold(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFold returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp foldResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if !resp.Kept {
		t.Error("expected kept=true")
	}

	// Verify branch still exists
	out, _ := exec.Command("git", "branch", "--list", "feat/auth").Output()
	if len(out) == 0 {
		t.Error("feat/auth should still exist with --keep")
	}
}

func TestHandleFold_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_fold", map[string]any{})
	result, _ := handleFold(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when folding trunk")
	}
}

func TestHandleFold_WithChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")
	_ = exec.Command("git", "checkout", "feat/auth").Run()

	req := makeRequest("gs_fold", map[string]any{})
	result, err := handleFold(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFold returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp foldResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.Reparented) != 1 {
		t.Fatalf("expected 1 reparented child, got %d", len(resp.Reparented))
	}
	if resp.Reparented[0] != "feat/auth-tests" {
		t.Errorf("expected reparented child 'feat/auth-tests', got '%s'", resp.Reparented[0])
	}
}
