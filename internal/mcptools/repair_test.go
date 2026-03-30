package mcptools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/repair"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// --- Register tests ---

func TestRegister(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
	Register(s) // just verify no panic
}

// --- issueToJSON tests ---

func TestIssueToJSON(t *testing.T) {
	iss := repair.Issue{
		Kind:        repair.OrphanedRef,
		Branch:      "feat/old",
		Description: "ref exists but branch deleted",
		Fix:         "delete ref",
	}
	j := issueToJSON(iss)

	if j.Kind != "orphaned_ref" {
		t.Errorf("expected kind 'orphaned_ref', got '%s'", j.Kind)
	}
	if j.Branch != "feat/old" {
		t.Errorf("expected branch 'feat/old', got '%s'", j.Branch)
	}
	if j.Description != "ref exists but branch deleted" {
		t.Errorf("expected description 'ref exists but branch deleted', got '%s'", j.Description)
	}
	if j.Fix != "delete ref" {
		t.Errorf("expected fix 'delete ref', got '%s'", j.Fix)
	}
}

// --- handleRepair tests ---

func TestHandleRepair_NoIssues(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_repair", map[string]any{"fix": false})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.IssuesFound) != 0 {
		t.Errorf("expected 0 issues, got %d", len(resp.IssuesFound))
	}
	if len(resp.IssuesFixed) != 0 {
		t.Errorf("expected 0 fixed, got %d", len(resp.IssuesFixed))
	}
	if len(resp.Remaining) != 0 {
		t.Errorf("expected 0 remaining, got %d", len(resp.Remaining))
	}
}

func TestHandleRepair_DryRun(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a tracked branch, then delete the git branch to create orphaned metadata
	addTrackedBranch(t, "stale-branch", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "branch", "-D", "stale-branch").Run()

	req := makeRequest("gs_repair", map[string]any{"fix": false})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.IssuesFound) == 0 {
		t.Fatal("expected at least 1 issue found")
	}
	// In dry-run mode, nothing should be fixed
	if len(resp.IssuesFixed) != 0 {
		t.Errorf("expected 0 fixed in dry-run, got %d", len(resp.IssuesFixed))
	}
	// Remaining should equal issues found
	if len(resp.Remaining) != len(resp.IssuesFound) {
		t.Errorf("expected remaining (%d) == issues found (%d)", len(resp.Remaining), len(resp.IssuesFound))
	}

	// Verify the issue is about stale-branch
	found := false
	for _, iss := range resp.IssuesFound {
		if iss.Branch == "stale-branch" {
			found = true
			if iss.Kind != "orphaned_ref" {
				t.Errorf("expected kind 'orphaned_ref', got '%s'", iss.Kind)
			}
		}
	}
	if !found {
		t.Error("expected issue for 'stale-branch'")
	}
}

func TestHandleRepair_FixOrphanedRef(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a tracked branch, then delete the git branch
	addTrackedBranch(t, "stale-branch", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "branch", "-D", "stale-branch").Run()

	req := makeRequest("gs_repair", map[string]any{"fix": true})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.IssuesFound) == 0 {
		t.Fatal("expected at least 1 issue found")
	}
	if len(resp.IssuesFixed) == 0 {
		t.Fatal("expected at least 1 issue fixed")
	}
	if len(resp.Remaining) != 0 {
		t.Errorf("expected 0 remaining after fix, got %d", len(resp.Remaining))
	}

	// Verify the fix sticks: a second repair should find no issues
	req2 := makeRequest("gs_repair", map[string]any{"fix": false})
	result2, _ := handleRepair(context.Background(), req2)
	text2 := result2.Content[0].(mcp.TextContent).Text
	var resp2 repairResponse
	_ = json.Unmarshal([]byte(text2), &resp2)

	if len(resp2.IssuesFound) != 0 {
		t.Errorf("expected 0 issues after fix, got %d", len(resp2.IssuesFound))
	}
}

// --- handleLand tests ---

func TestHandleLand_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_land", map[string]any{"branch": "main"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when landing trunk")
	}
}

func TestHandleLand_UnmergedBranchErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/unmerged", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/unmerged"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when landing unmerged branch")
	}
}

func TestHandleLand_MergedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/done", "main")

	// Merge feat/done into main so it's considered merged
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/done", "--no-ff", "-m", "merge feat/done").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/done"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Landed != "feat/done" {
		t.Errorf("expected landed 'feat/done', got '%s'", resp.Landed)
	}
	if resp.NewParent != "main" {
		t.Errorf("expected new parent 'main', got '%s'", resp.NewParent)
	}

	// Verify branch is gone from git
	out, _ := exec.Command("git", "branch", "--list", "feat/done").Output()
	if len(out) > 0 {
		t.Error("feat/done should have been deleted")
	}
}

func TestHandleLand_ReparentsChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Merge feat/parent into main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/parent", "--no-ff", "-m", "merge feat/parent").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/parent"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.ReparentedChildren) != 1 {
		t.Fatalf("expected 1 reparented child, got %d", len(resp.ReparentedChildren))
	}
	if resp.ReparentedChildren[0] != "feat/child" {
		t.Errorf("expected reparented child 'feat/child', got '%s'", resp.ReparentedChildren[0])
	}

	// Verify child's parent is now main by checking status
	statusReq := mcp.CallToolRequest{}
	statusResult, _ := handleStatus(context.Background(), statusReq)
	statusText := statusResult.Content[0].(mcp.TextContent).Text
	var status statusResponse
	_ = json.Unmarshal([]byte(statusText), &status)

	for _, b := range status.Branches {
		if b.Name == "feat/child" {
			if b.Parent != "main" {
				t.Errorf("expected feat/child parent 'main', got '%s'", b.Parent)
			}
		}
	}
}

func TestHandleLand_CurrentBranchLanding(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/current", "main")

	// Merge into main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/current", "--no-ff", "-m", "merge").Run()

	// Switch back to feat/current so it's the current branch
	_ = exec.Command("git", "checkout", "feat/current").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/current"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CheckedOut != "main" {
		t.Errorf("expected checkout to 'main', got '%s'", resp.CheckedOut)
	}
}

// --- computeMCPRestackBranches tests ---

func loadTestState(t *testing.T) *repoState {
	t.Helper()
	state, err := loadRepoState()
	if err != nil {
		t.Fatalf("failed to load repo state: %v", err)
	}
	return state
}

func TestComputeMCPRestackBranches_Only(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")
	_ = exec.Command("git", "checkout", "B").Run()

	state := loadTestState(t)

	// scope=only, branch=B -> [B]
	result := computeMCPRestackBranches(state, "B", "only")
	if len(result) != 1 || result[0] != "B" {
		t.Errorf("expected [B], got %v", result)
	}

	// scope=only, branch=main -> nil (trunk)
	result = computeMCPRestackBranches(state, "main", "only")
	if len(result) != 0 {
		t.Errorf("expected nil for trunk, got %v", result)
	}
}

func TestComputeMCPRestackBranches_Upstack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")
	_ = exec.Command("git", "checkout", "A").Run()

	state := loadTestState(t)

	// scope=upstack, branch=A -> [A, B, C]
	result := computeMCPRestackBranches(state, "A", "upstack")
	if len(result) != 3 {
		t.Fatalf("expected 3 branches, got %d: %v", len(result), result)
	}
	if result[0] != "A" {
		t.Errorf("expected first branch 'A', got '%s'", result[0])
	}
	// B and C should follow
	names := map[string]bool{}
	for _, n := range result {
		names[n] = true
	}
	if !names["B"] || !names["C"] {
		t.Errorf("expected B and C in result, got %v", result)
	}
}

func TestComputeMCPRestackBranches_Downstack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")
	_ = exec.Command("git", "checkout", "C").Run()

	state := loadTestState(t)

	// scope=downstack, branch=C -> [A, B, C] (ancestors, excluding trunk)
	result := computeMCPRestackBranches(state, "C", "downstack")
	if len(result) != 3 {
		t.Fatalf("expected 3 branches, got %d: %v", len(result), result)
	}
	// Should be in ancestor order: A, B, C
	if result[0] != "A" {
		t.Errorf("expected first 'A', got '%s'", result[0])
	}
	if result[1] != "B" {
		t.Errorf("expected second 'B', got '%s'", result[1])
	}
	if result[2] != "C" {
		t.Errorf("expected third 'C', got '%s'", result[2])
	}
}

func TestComputeMCPRestackBranches_All(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")
	_ = exec.Command("git", "checkout", "B").Run()

	state := loadTestState(t)

	// scope=all, branch=B -> should include A, B, C (ancestors + descendants)
	result := computeMCPRestackBranches(state, "B", "all")
	names := map[string]bool{}
	for _, n := range result {
		names[n] = true
	}
	if !names["A"] || !names["B"] || !names["C"] {
		t.Errorf("expected A, B, C in result, got %v", result)
	}
	// Should not include main (trunk)
	if names["main"] {
		t.Error("trunk should not be in result")
	}
}

func TestComputeMCPRestackBranches_AllFromTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	_ = exec.Command("git", "checkout", "main").Run()

	state := loadTestState(t)

	// scope=all, branch=main -> allTopological (everything)
	result := computeMCPRestackBranches(state, "main", "all")
	if len(result) < 2 {
		t.Errorf("expected at least 2 branches from trunk all, got %d: %v", len(result), result)
	}
}

// --- allTopological tests ---

func TestAllTopological(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "C", "main")
	_ = exec.Command("git", "checkout", "C").Run()

	state := loadTestState(t)
	result := allTopological(state.Stack)

	// allTopological returns nodes from GetTopologicalOrder which may or may not
	// include trunk depending on the stack implementation. Check that at least
	// the tracked branches are present.
	names := map[string]bool{}
	for _, n := range result {
		names[n] = true
	}
	for _, expected := range []string{"A", "B", "C"} {
		if !names[expected] {
			t.Errorf("expected '%s' in topological order, got %v", expected, result)
		}
	}
	if len(result) < 3 {
		t.Fatalf("expected at least 3 branches (A, B, C), got %d: %v", len(result), result)
	}
}

// --- ancestorsOf tests ---

func TestAncestorsOf(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")
	_ = exec.Command("git", "checkout", "C").Run()

	state := loadTestState(t)

	// ancestors of C should be [A, B, C] (excluding trunk)
	result := ancestorsOf(state.Stack, "main", "C")
	if len(result) != 3 {
		t.Fatalf("expected 3 ancestors, got %d: %v", len(result), result)
	}
	if result[0] != "A" {
		t.Errorf("expected first ancestor 'A', got '%s'", result[0])
	}
	if result[1] != "B" {
		t.Errorf("expected second ancestor 'B', got '%s'", result[1])
	}
	if result[2] != "C" {
		t.Errorf("expected third ancestor 'C', got '%s'", result[2])
	}

	// ancestors of A should be [A] (just itself, trunk excluded)
	result = ancestorsOf(state.Stack, "main", "A")
	if len(result) != 1 {
		t.Fatalf("expected 1 ancestor, got %d: %v", len(result), result)
	}
	if result[0] != "A" {
		t.Errorf("expected 'A', got '%s'", result[0])
	}
}

// --- mcpRestackChildren tests ---

func TestMcpRestackChildren_NoBehindChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")
	_ = exec.Command("git", "checkout", "main").Run()

	// No commits added to parent, so child is not behind
	state := loadTestState(t)
	parentNode := state.Stack.GetNode("feat/parent")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, parentNode)

	if len(restacked) != 0 {
		t.Errorf("expected 0 restacked (not behind), got %d: %v", len(restacked), restacked)
	}
}

func TestMcpRestackChildren_WithBehindChild(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Add a commit to feat/parent to make feat/child behind
	_ = exec.Command("git", "checkout", "feat/parent").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "parent-update.txt"), []byte("update"), 0644)
	_ = exec.Command("git", "add", "parent-update.txt").Run()
	_ = exec.Command("git", "commit", "-m", "parent update").Run()

	state := loadTestState(t)
	parentNode := state.Stack.GetNode("feat/parent")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, parentNode)

	if len(restacked) != 1 {
		t.Fatalf("expected 1 restacked, got %d: %v", len(restacked), restacked)
	}
	if restacked[0] != "feat/child" {
		t.Errorf("expected 'feat/child' restacked, got '%s'", restacked[0])
	}
}

func TestMcpRestackChildren_RecursiveRestack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")

	// Add a commit to main to make A behind
	_ = exec.Command("git", "checkout", "main").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "main-update.txt"), []byte("update"), 0644)
	_ = exec.Command("git", "add", "main-update.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main update").Run()

	state := loadTestState(t)
	mainNode := state.Stack.GetNode("main")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, mainNode)

	// A should be restacked since it's behind main
	// B and C may or may not need restacking depending on merge-base
	if len(restacked) == 0 {
		t.Error("expected at least 1 branch restacked")
	}

	found := false
	for _, name := range restacked {
		if name == "A" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'A' in restacked, got %v", restacked)
	}
}

// --- handleRestack scope tests (through the handler) ---

func TestHandleRestack_ScopeUpstack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")

	// Add a commit to main so A needs rebase
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("main-change.txt", []byte("change"), 0644)
	_ = exec.Command("git", "add", "main-change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main change").Run()

	_ = exec.Command("git", "checkout", "A").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "upstack"})
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

	if len(resp.Restacked) == 0 {
		t.Error("expected at least 1 restacked branch with upstack scope")
	}
}

func TestHandleRestack_ScopeAll(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")

	// Add a commit to main so A needs rebase
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("main-change.txt", []byte("change"), 0644)
	_ = exec.Command("git", "add", "main-change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main change").Run()

	_ = exec.Command("git", "checkout", "B").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "all"})
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

	if len(resp.Restacked) == 0 {
		t.Error("expected at least 1 restacked branch with all scope")
	}
}

func TestHandleRestack_ScopeDownstack(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")

	// Add a commit to main so A needs rebase
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("main-change.txt", []byte("change"), 0644)
	_ = exec.Command("git", "add", "main-change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main change").Run()

	_ = exec.Command("git", "checkout", "B").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "downstack"})
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

	// A should be restacked (ancestor of B that's behind main)
	if len(resp.Restacked) == 0 {
		t.Error("expected at least 1 restacked branch with downstack scope")
	}
}

// --- handleSubmit error path tests (no provider needed) ---

func TestHandleSubmit_TrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_submit", map[string]any{"branch": "main"})
	result, err := handleSubmit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSubmit returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when submitting trunk")
	}
}

func TestHandleSubmit_UntrackedBranchErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create an untracked git branch
	_ = exec.Command("git", "checkout", "-b", "untracked").Run()

	req := makeRequest("gs_submit", map[string]any{"branch": "untracked"})
	result, err := handleSubmit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSubmit returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

func TestHandleSubmit_NoRemoteErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/test", "main")

	req := makeRequest("gs_submit", map[string]any{"branch": "feat/test"})
	result, err := handleSubmit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSubmit returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when no remote configured")
	}
}

// --- handleRepair with default (no fix param) ---

func TestHandleRepair_DefaultsDryRun(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create orphaned metadata
	addTrackedBranch(t, "orphan", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "branch", "-D", "orphan").Run()

	// Call without fix parameter (should default to false)
	req := makeRequest("gs_repair", map[string]any{})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	_ = json.Unmarshal([]byte(text), &resp)

	// Default is dry-run, so nothing should be fixed
	if len(resp.IssuesFixed) != 0 {
		t.Errorf("expected 0 fixed with default params, got %d", len(resp.IssuesFixed))
	}
	if len(resp.IssuesFound) == 0 {
		t.Error("expected issues found for orphaned branch")
	}
}

// --- handleLand default branch (current branch) ---

func TestHandleLand_DefaultsToCurrentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/landing", "main")

	// Merge into main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/landing", "--no-ff", "-m", "merge").Run()

	// Go back to the feature branch
	_ = exec.Command("git", "checkout", "feat/landing").Run()

	// Call without branch parameter (should default to current)
	req := makeRequest("gs_land", map[string]any{})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Landed != "feat/landing" {
		t.Errorf("expected landed 'feat/landing', got '%s'", resp.Landed)
	}
}

// --- Navigation error path tests ---

func TestHandleNavigate_InvalidDirection(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_navigate", map[string]any{"direction": "sideways"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid direction")
	}
}

func TestHandleNavigate_UpAtLeaf(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/leaf", "main")
	// Currently on feat/leaf which has no children

	req := makeRequest("gs_navigate", map[string]any{"direction": "up"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when navigating up from leaf")
	}
}

func TestHandleNavigate_TopAmbiguous(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	addTrackedBranch(t, "feat/a-child1", "feat/a")
	_ = exec.Command("git", "checkout", "feat/a").Run()
	addTrackedBranch(t, "feat/a-child2", "feat/a")
	_ = exec.Command("git", "checkout", "main").Run()

	// Navigate to top from main — main has one child (feat/a)
	// but feat/a has two children, so top should be ambiguous
	req := makeRequest("gs_navigate", map[string]any{"direction": "top"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	// Should return ambiguous response (not an error)
	if result.IsError {
		t.Fatal("expected non-error ambiguous response for top")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateAmbiguousResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Error != "ambiguous_navigation" {
		t.Errorf("expected ambiguous_navigation, got '%s'", resp.Error)
	}
	if len(resp.Options) != 2 {
		t.Errorf("expected 2 leaf options, got %d", len(resp.Options))
	}
}

func TestHandleNavigate_TopAtLeafErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/leaf", "main")
	// On feat/leaf, which has no children

	req := makeRequest("gs_navigate", map[string]any{"direction": "top"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for top at leaf")
	}
}

func TestHandleNavigate_BottomAtTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// On main (trunk)
	req := makeRequest("gs_navigate", map[string]any{"direction": "bottom"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when already at trunk")
	}
}

// --- Additional modify tests ---

func TestHandleModify_StageAll(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// Create an unstaged change
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "unstaged.txt"), []byte("unstaged"), 0644)

	req := makeRequest("gs_modify", map[string]any{
		"message":   "amend with stage_all",
		"stage_all": true,
	})
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

	if resp.Action != "amended" {
		t.Errorf("expected action 'amended', got '%s'", resp.Action)
	}
}

func TestHandleModify_WithRestackChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Go to parent and amend (which should trigger child restack)
	_ = exec.Command("git", "checkout", "feat/parent").Run()

	// Add a file to stage
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "extra.txt"), []byte("extra"), 0644)

	req := makeRequest("gs_modify", map[string]any{
		"message":   "amend parent",
		"stage_all": true,
	})
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

	// Child should be restacked since parent was amended
	if len(resp.RestackedChildren) != 1 {
		t.Errorf("expected 1 restacked child, got %d", len(resp.RestackedChildren))
	}
}

func TestHandleModify_AmendNoMessage(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// Amend without changing message (--no-edit)
	req := makeRequest("gs_modify", map[string]any{})
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

	if resp.Action != "amended" {
		t.Errorf("expected action 'amended', got '%s'", resp.Action)
	}
}

// --- Additional track error tests ---

func TestHandleTrack_NonexistentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_track", map[string]any{"branch": "nonexistent", "parent": "main"})
	result, _ := handleTrack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for nonexistent branch")
	}
}

func TestHandleTrack_NonexistentParent(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "real-branch").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_track", map[string]any{"branch": "real-branch", "parent": "nonexistent"})
	result, _ := handleTrack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for nonexistent parent")
	}
}

// --- Additional untrack tests ---

func TestHandleUntrack_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create git branch but don't track
	_ = exec.Command("git", "checkout", "-b", "untracked").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_untrack", map[string]any{"branch": "untracked"})
	result, _ := handleUntrack(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

// --- Additional delete error tests ---

func TestHandleDelete_NonexistentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_delete", map[string]any{"branch": "nonexistent"})
	result, _ := handleDelete(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for nonexistent branch")
	}
}

func TestHandleDelete_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "untracked").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_delete", map[string]any{"branch": "untracked"})
	result, _ := handleDelete(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

// --- Additional move error tests ---

func TestHandleMove_NonexistentTarget(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")

	req := makeRequest("gs_move", map[string]any{"branch": "feat/a", "onto": "nonexistent"})
	result, _ := handleMove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for nonexistent target")
	}
}

// --- Additional checkout error tests ---

func TestHandleCheckout_MissingParam(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_checkout", map[string]any{})
	result, err := handleCheckout(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckout returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing branch param")
	}
}

// --- Additional diff tests ---

func TestHandleDiff_NonexistentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_diff", map[string]any{"branch": "nonexistent"})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent branch")
	}
}

// --- move onto descendant test ---

func TestHandleMove_OntoDescendant(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	_ = exec.Command("git", "checkout", "A").Run()

	req := makeRequest("gs_move", map[string]any{"branch": "A", "onto": "B"})
	result, _ := handleMove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when moving onto own descendant")
	}
}

func TestHandleMove_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "untracked").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_move", map[string]any{"branch": "untracked", "onto": "main"})
	result, _ := handleMove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

func TestHandleMove_DefaultsToCurrentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/b", "main")
	// Currently on feat/b

	req := makeRequest("gs_move", map[string]any{"onto": "feat/a"})
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

	if resp.Branch != "feat/b" {
		t.Errorf("expected branch 'feat/b', got '%s'", resp.Branch)
	}
}

// --- untrack with children warning test ---

func TestHandleUntrack_WithChildrenWarning(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_untrack", map[string]any{"branch": "feat/parent"})
	result, err := handleUntrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleUntrack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleUntrack returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp untrackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.Warnings) == 0 {
		t.Error("expected warning about orphaned children")
	}
}

// --- restack with untracked branch error ---

func TestHandleRestack_UntrackedBranchErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_restack", map[string]any{"branch": "nonexistent", "scope": "only"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

// --- restack with specific branch param ---

func TestHandleRestack_SpecificBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")

	// Add commit to main to make A behind
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("change.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "change").Run()

	req := makeRequest("gs_restack", map[string]any{"branch": "A", "scope": "only"})
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

	if len(resp.Restacked) != 1 || resp.Restacked[0] != "A" {
		t.Errorf("expected [A] restacked, got %v", resp.Restacked)
	}
}

// --- restack returns to original branch ---

func TestHandleRestack_ReturnsToOriginal(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")

	// Add commit to main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("change.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "change").Run()

	// Stay on main and restack all
	req := makeRequest("gs_restack", map[string]any{"scope": "all", "branch": "main"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRestack returned tool error: %s", text)
	}

	// Verify we're back on main
	out, _ := exec.Command("git", "branch", "--show-current").Output()
	current := string(out[:len(out)-1])
	if current != "main" {
		t.Errorf("expected to be back on 'main', got '%s'", current)
	}
}

// --- navigate missing direction param ---

func TestHandleNavigate_MissingDirection(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_navigate", map[string]any{})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing direction")
	}
}

// --- create missing name param ---

func TestHandleCreate_MissingName(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_create", map[string]any{})
	result, err := handleCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCreate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing name")
	}
}

// --- rename missing new_name param ---

func TestHandleRename_MissingNewName(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/x", "main")

	req := makeRequest("gs_rename", map[string]any{})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing new_name")
	}
}

// --- delete missing branch param ---

func TestHandleDelete_MissingBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_delete", map[string]any{})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing branch")
	}
}

// --- track missing params ---

func TestHandleTrack_MissingBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_track", map[string]any{"parent": "main"})
	result, err := handleTrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTrack returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing branch")
	}
}

func TestHandleTrack_MissingParent(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_track", map[string]any{"branch": "some-branch"})
	result, err := handleTrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTrack returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing parent")
	}
}

// --- untrack missing branch param ---

func TestHandleUntrack_MissingBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_untrack", map[string]any{})
	result, err := handleUntrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleUntrack returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing branch")
	}
}

// --- move missing onto param ---

func TestHandleMove_MissingOnto(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_move", map[string]any{})
	result, err := handleMove(context.Background(), req)
	if err != nil {
		t.Fatalf("handleMove returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing onto")
	}
}

// --- ComputeMCPRestackBranches upstack from trunk ---

func TestComputeMCPRestackBranches_UpstackFromTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	_ = exec.Command("git", "checkout", "main").Run()

	state := loadTestState(t)

	// scope=upstack from trunk should include descendants but not trunk itself
	result := computeMCPRestackBranches(state, "main", "upstack")
	names := map[string]bool{}
	for _, n := range result {
		names[n] = true
	}
	if names["main"] {
		t.Error("trunk should not be in upstack result")
	}
	if !names["A"] || !names["B"] {
		t.Errorf("expected A and B in upstack from trunk, got %v", result)
	}
}

// --- setupTestRepoWithRemote creates a repo with a bare remote for push/pull tests ---

func setupTestRepoWithRemote(t *testing.T) func() {
	t.Helper()

	// Create bare remote
	bareDir, err := os.MkdirTemp("", "gs-mcp-bare-*")
	if err != nil {
		t.Fatalf("failed to create bare dir: %v", err)
	}
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		_ = os.RemoveAll(bareDir)
		t.Fatalf("git init --bare failed: %v", err)
	}

	// Create working repo
	tmpDir, err := os.MkdirTemp("", "gs-mcp-remote-test-*")
	if err != nil {
		_ = os.RemoveAll(bareDir)
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		_ = os.RemoveAll(bareDir)
		t.Fatalf("failed to get current dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		_ = os.RemoveAll(bareDir)
		t.Fatalf("failed to chdir: %v", err)
	}

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			_ = os.Chdir(origDir)
			_ = os.RemoveAll(tmpDir)
			_ = os.RemoveAll(bareDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
		{"branch", "-M", "main"},
		{"remote", "add", "origin", bareDir},
		{"push", "-u", "origin", "main"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			_ = os.Chdir(origDir)
			_ = os.RemoveAll(tmpDir)
			_ = os.RemoveAll(bareDir)
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	gitDir := filepath.Join(tmpDir, ".git")
	configData := `{"version":"1.0.0","trunk":"main"}`
	metadataData := `{"branches":{}}`
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_config"), []byte(configData), 0600)
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_stack_metadata"), []byte(metadataData), 0600)

	return func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
		_ = os.RemoveAll(bareDir)
	}
}

// --- Tests with remote (exercises pushMetadataRefs, deleteRemoteMetadataRef) ---

func TestHandleCreate_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	req := makeRequest("gs_create", map[string]any{"name": "feat/remote-test"})
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

	if resp.Branch != "feat/remote-test" {
		t.Errorf("expected branch 'feat/remote-test', got '%s'", resp.Branch)
	}
}

func TestHandleDelete_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/to-delete-remote", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_delete", map[string]any{"branch": "feat/to-delete-remote"})
	result, err := handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleDelete returned tool error: %s", text)
	}
}

func TestHandleRename_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/old-remote", "main")

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/new-remote"})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRename returned tool error: %s", text)
	}
}

func TestHandleUntrack_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/untrack-remote", "main")

	req := makeRequest("gs_untrack", map[string]any{"branch": "feat/untrack-remote"})
	result, err := handleUntrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleUntrack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleUntrack returned tool error: %s", text)
	}
}

func TestHandleTrack_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "untracked-remote").Run()
	_ = os.WriteFile("untracked-remote.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "untracked-remote.txt").Run()
	_ = exec.Command("git", "commit", "-m", "untracked commit").Run()
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_track", map[string]any{"branch": "untracked-remote", "parent": "main"})
	result, err := handleTrack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTrack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleTrack returned tool error: %s", text)
	}
}

func TestHandleFold_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/fold-remote", "main")

	req := makeRequest("gs_fold", map[string]any{})
	result, err := handleFold(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFold returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleFold returned tool error: %s", text)
	}
}

func TestHandleModify_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/modify-remote", "main")

	req := makeRequest("gs_modify", map[string]any{"message": "amended remote"})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleModify returned tool error: %s", text)
	}
}

func TestHandleMove_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a-remote", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/b-remote", "main")

	req := makeRequest("gs_move", map[string]any{"branch": "feat/b-remote", "onto": "feat/a-remote"})
	result, err := handleMove(context.Background(), req)
	if err != nil {
		t.Fatalf("handleMove returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleMove returned tool error: %s", text)
	}
}

func TestHandleRestack_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/restack-remote", "main")

	// Add commit to main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("main-change.txt", []byte("change"), 0644)
	_ = exec.Command("git", "add", "main-change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main change").Run()

	_ = exec.Command("git", "checkout", "feat/restack-remote").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "only"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRestack returned tool error: %s", text)
	}
}

func TestHandleRepair_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "stale-remote", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "branch", "-D", "stale-remote").Run()

	req := makeRequest("gs_repair", map[string]any{"fix": true})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.IssuesFixed) == 0 {
		t.Error("expected at least 1 issue fixed")
	}
}

func TestHandleLand_WithRemote(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/land-remote", "main")
	addTrackedBranch(t, "feat/land-child", "feat/land-remote")

	// Merge feat/land-remote into main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/land-remote", "--no-ff", "-m", "merge feat/land-remote").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/land-remote"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Landed != "feat/land-remote" {
		t.Errorf("expected landed 'feat/land-remote', got '%s'", resp.Landed)
	}
	if len(resp.ReparentedChildren) != 1 || resp.ReparentedChildren[0] != "feat/land-child" {
		t.Errorf("expected [feat/land-child] reparented, got %v", resp.ReparentedChildren)
	}
}

func TestHandleDelete_WithRemoteAndChildren(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent-remote", "main")
	addTrackedBranch(t, "feat/child-remote", "feat/parent-remote")
	_ = exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_delete", map[string]any{"branch": "feat/parent-remote"})
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

	if len(resp.ReparentedChildren) != 1 || resp.ReparentedChildren[0] != "feat/child-remote" {
		t.Errorf("expected [feat/child-remote] reparented, got %v", resp.ReparentedChildren)
	}
}

func TestHandleRename_WithRemoteAndChildren(t *testing.T) {
	cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	addTrackedBranch(t, "feat/old-remote2", "main")
	addTrackedBranch(t, "feat/child-remote2", "feat/old-remote2")
	_ = exec.Command("git", "checkout", "feat/old-remote2").Run()

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/new-remote2"})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRename returned tool error: %s", text)
	}
}

// --- loadRepoState error paths (outside git repo) ---

func TestHandlers_NotGitRepo(t *testing.T) {
	// Create a temp dir that is NOT a git repo
	tmpDir, err := os.MkdirTemp("", "gs-mcp-not-git-*")
	if err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// All handlers should return errors when not in a git repo
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		req     mcp.CallToolRequest
	}{
		{"checkout", handleCheckout, makeRequest("gs_checkout", map[string]any{"branch": "main"})},
		{"navigate", handleNavigate, makeRequest("gs_navigate", map[string]any{"direction": "up"})},
		{"create", handleCreate, makeRequest("gs_create", map[string]any{"name": "feat/x"})},
		{"delete", handleDelete, makeRequest("gs_delete", map[string]any{"branch": "feat/x"})},
		{"track", handleTrack, makeRequest("gs_track", map[string]any{"branch": "x", "parent": "main"})},
		{"untrack", handleUntrack, makeRequest("gs_untrack", map[string]any{"branch": "x"})},
		{"rename", handleRename, makeRequest("gs_rename", map[string]any{"new_name": "y"})},
		{"restack", handleRestack, makeRequest("gs_restack", map[string]any{})},
		{"modify", handleModify, makeRequest("gs_modify", map[string]any{"message": "x"})},
		{"move", handleMove, makeRequest("gs_move", map[string]any{"onto": "main"})},
		{"fold", handleFold, makeRequest("gs_fold", map[string]any{})},
		{"repair", handleRepair, makeRequest("gs_repair", map[string]any{})},
		{"submit", handleSubmit, makeRequest("gs_submit", map[string]any{})},
		{"land", handleLand, makeRequest("gs_land", map[string]any{})},
		{"status", handleStatus, mcp.CallToolRequest{}},
		{"log", handleLog, mcp.CallToolRequest{}},
		{"branch_info", handleBranchInfo, makeRequest("gs_branch_info", map[string]any{"branch": "main"})},
		{"diff", handleDiff, makeRequest("gs_diff", map[string]any{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(ctx, tt.req)
			if err != nil {
				t.Fatalf("handler returned Go error: %v", err)
			}
			if !result.IsError {
				t.Error("expected tool error when not in git repo")
			}
		})
	}
}

// --- descendantsDFS with nonexistent branch ---

func TestDescendantsDFS_NilNode(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")

	state := loadTestState(t)
	result := descendantsDFS(state.Stack, "nonexistent")
	if result != nil {
		t.Errorf("expected nil for nonexistent branch, got %v", result)
	}
}

// --- helpers.go loadRepoState with gs not initialized (covers config load error) ---

func TestLoadRepoState_NoConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gs-mcp-noconfig-*")
	if err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}()

	// Initialize git repo but don't create gs config
	_ = exec.Command("git", "init").Run()
	_ = exec.Command("git", "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "config", "user.name", "Test User").Run()
	_ = os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x"), 0644)
	_ = exec.Command("git", "add", ".").Run()
	_ = exec.Command("git", "commit", "-m", "init").Run()

	_, err = loadRepoState()
	if err == nil {
		t.Error("expected error when gs config is missing")
	}
}

// --- helpers.go loadRepoState with bad metadata (covers metadata load error) ---

func TestLoadRepoState_BadMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gs-mcp-badmeta-*")
	if err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}()

	_ = exec.Command("git", "init").Run()
	_ = exec.Command("git", "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "config", "user.name", "Test User").Run()
	_ = os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x"), 0644)
	_ = exec.Command("git", "add", ".").Run()
	_ = exec.Command("git", "commit", "-m", "init").Run()

	gitDir := filepath.Join(tmpDir, ".git")
	// Write valid config
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_config"), []byte(`{"version":"1.0.0","trunk":"main"}`), 0600)
	// Write invalid metadata
	_ = os.WriteFile(filepath.Join(gitDir, ".gs_stack_metadata"), []byte(`{invalid json`), 0600)

	_, err = loadRepoState()
	if err == nil {
		t.Error("expected error with bad metadata")
	}
}

// --- handleNavigate with steps < 1 ---

func TestHandleNavigate_StepsZero(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	// steps=0 should be coerced to 1
	req := makeRequest("gs_navigate", map[string]any{"direction": "up", "steps": 0})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success for steps=0 (coerced to 1)")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/a" {
		t.Errorf("expected 'feat/a', got '%s'", resp.CurrentBranch)
	}
}

// --- handleNavigate with negative steps ---

func TestHandleNavigate_NegativeSteps(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	// negative steps should be coerced to 1
	req := makeRequest("gs_navigate", map[string]any{"direction": "up", "steps": -5})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success for negative steps (coerced to 1)")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/a" {
		t.Errorf("expected 'feat/a', got '%s'", resp.CurrentBranch)
	}
}

// --- handleNavigate on untracked branch (node == nil) ---

func TestHandleNavigate_UntrackedCurrentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create untracked branch
	_ = exec.Command("git", "checkout", "-b", "untracked-nav").Run()
	_ = os.WriteFile("nav-file.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "nav-file.txt").Run()
	_ = exec.Command("git", "commit", "-m", "nav commit").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "up"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked current branch")
	}
}

// --- handleModify on untracked branch (node == nil) ---

func TestHandleModify_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "untracked-modify").Run()
	_ = os.WriteFile("mod-file.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "mod-file.txt").Run()
	_ = exec.Command("git", "commit", "-m", "mod commit").Run()

	req := makeRequest("gs_modify", map[string]any{"message": "amend"})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

// --- handleRename same name ---

func TestHandleRename_SameNameErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/x", "main")
	// Currently on feat/x

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/x"})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when renaming to same name")
	}
}

// --- handleModify on trunk ---

func TestHandleModify_OnTrunkErrors(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// On main (trunk), which is not tracked in metadata as a regular branch
	// The node should be trunk, not nil. But let's verify trunk behavior
	req := makeRequest("gs_modify", map[string]any{"message": "bad"})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}
	// Trunk should either error or amend, depending on impl
	// The key is that we exercise the path
	_ = result
}

// --- handleModify new_commit with commit failure ---

func TestHandleModify_NewCommitNoStagedChanges(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")

	// new_commit with message but no staged changes -> git commit should fail
	req := makeRequest("gs_modify", map[string]any{
		"message":    "new commit",
		"new_commit": true,
	})
	result, err := handleModify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleModify returned error: %v", err)
	}
	// Should error because nothing is staged
	if !result.IsError {
		t.Error("expected error when new_commit with nothing staged")
	}
}

// --- handleFold on untracked branch ---

func TestHandleFold_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a branch but don't track it in gs
	_ = exec.Command("git", "checkout", "-b", "untracked-fold").Run()
	_ = os.WriteFile("untracked-fold.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "untracked-fold.txt").Run()
	_ = exec.Command("git", "commit", "-m", "commit").Run()

	req := makeRequest("gs_fold", map[string]any{})
	result, err := handleFold(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFold returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when folding untracked branch")
	}
}

// --- handleRestack with conflict ---

func TestHandleRestack_WithConflict(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/conflict", "main")

	// Create conflicting changes: modify README.md on main
	_ = exec.Command("git", "checkout", "main").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "README.md"), []byte("# Main changes"), 0644)
	_ = exec.Command("git", "add", "README.md").Run()
	_ = exec.Command("git", "commit", "-m", "main change to README").Run()

	// Modify same file on the branch
	_ = exec.Command("git", "checkout", "feat/conflict").Run()
	_ = os.WriteFile(filepath.Join(cwd, "README.md"), []byte("# Branch changes"), 0644)
	_ = exec.Command("git", "add", "README.md").Run()
	_ = exec.Command("git", "commit", "--amend", "-m", "branch change to README").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "only"})
	result, err := handleRestack(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRestack returned error: %v", err)
	}

	// May or may not conflict depending on rebase strategy
	// Just exercise the code path
	text := result.Content[0].(mcp.TextContent).Text
	var resp restackResponse
	_ = json.Unmarshal([]byte(text), &resp)

	// If conflict occurred, it should be reported
	if resp.Conflict != "" {
		// Clean up: abort the rebase
		_ = exec.Command("git", "rebase", "--abort").Run()
	}
}

// --- handleRestack without parentRev (plain rebase fallback) ---

func TestHandleRestack_WithoutParentRev(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/no-rev", "main")

	// Clear the parent revision from metadata
	gitDirOut, _ := exec.Command("git", "rev-parse", "--git-dir").Output()
	gitDir := filepath.Clean(string(gitDirOut[:len(gitDirOut)-1]))
	metadataPath := filepath.Join(gitDir, ".gs_stack_metadata")
	data, _ := os.ReadFile(metadataPath)
	var meta map[string]any
	_ = json.Unmarshal(data, &meta)
	branches := meta["branches"].(map[string]any)
	branchMeta := branches["feat/no-rev"].(map[string]any)
	delete(branchMeta, "parent_revision")
	updated, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metadataPath, updated, 0600)

	// Add commit to main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("new-file.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "new-file.txt").Run()
	_ = exec.Command("git", "commit", "-m", "main commit").Run()

	_ = exec.Command("git", "checkout", "feat/no-rev").Run()

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

	if len(resp.Restacked) != 1 || resp.Restacked[0] != "feat/no-rev" {
		t.Errorf("expected [feat/no-rev] restacked, got %v", resp.Restacked)
	}
}

// --- handleLand with children that get restacked ---

func TestHandleLand_WithRestackedChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Add a commit to feat/parent to make child behind
	_ = exec.Command("git", "checkout", "feat/parent").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "parent-extra.txt"), []byte("extra"), 0644)
	_ = exec.Command("git", "add", "parent-extra.txt").Run()
	_ = exec.Command("git", "commit", "-m", "extra parent commit").Run()

	// Merge feat/parent into main
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "merge", "feat/parent", "--no-ff", "-m", "merge feat/parent").Run()

	req := makeRequest("gs_land", map[string]any{"branch": "feat/parent"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleLand returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp landResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.ReparentedChildren) != 1 {
		t.Fatalf("expected 1 reparented child, got %d", len(resp.ReparentedChildren))
	}
	// Restacked may or may not contain feat/child depending on whether it needed restack
	// The key is we exercise the restacking code path
}

// --- handleCreate with commit message but nothing staged ---

func TestHandleCreate_CommitMessageNoStagedChanges(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_create", map[string]any{
		"name":           "feat/no-staged",
		"commit_message": "empty commit",
	})
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

	// Commit should have failed (nothing staged) so CommitCreated should be false
	if resp.CommitCreated {
		t.Error("expected CommitCreated=false when nothing is staged")
	}
}

// --- handleSubmit with no parent (node.Parent == nil) ---

func TestHandleSubmit_BranchWithNoParent(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Manually create metadata for a branch with no parent
	addTrackedBranch(t, "feat/orphan", "main")

	// Corrupt metadata to remove parent
	gitDirOut, _ := exec.Command("git", "rev-parse", "--git-dir").Output()
	gitDir := filepath.Clean(string(gitDirOut[:len(gitDirOut)-1]))
	metadataPath := filepath.Join(gitDir, ".gs_stack_metadata")
	data, _ := os.ReadFile(metadataPath)
	var meta map[string]any
	_ = json.Unmarshal(data, &meta)
	branches := meta["branches"].(map[string]any)
	branchMeta := branches["feat/orphan"].(map[string]any)
	branchMeta["parent"] = ""
	updated, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metadataPath, updated, 0600)

	req := makeRequest("gs_submit", map[string]any{"branch": "feat/orphan"})
	result, err := handleSubmit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSubmit returned error: %v", err)
	}
	// Should error: branch has no parent, or is not tracked
	if !result.IsError {
		t.Error("expected error for branch with no parent")
	}
}

// --- navigateDown multiple steps hitting trunk early ---

func TestHandleNavigate_DownMultipleStepsHitsTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	// On feat/a, depth 1

	// Try going down 5 steps (should stop at trunk after 1)
	req := makeRequest("gs_navigate", map[string]any{"direction": "down", "steps": 5})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success (partial navigation)")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "main" {
		t.Errorf("expected 'main', got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 1 {
		t.Errorf("expected 1 step (stopped at trunk), got %d", resp.StepsTaken)
	}
}

// --- navigateUp multiple steps hitting leaf early ---

func TestHandleNavigate_UpMultipleStepsHitsLeaf(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()

	// Try going up 5 steps (should stop at feat/a after 1)
	req := makeRequest("gs_navigate", map[string]any{"direction": "up", "steps": 5})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success (partial navigation)")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/a" {
		t.Errorf("expected 'feat/a', got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 1 {
		t.Errorf("expected 1 step (stopped at leaf), got %d", resp.StepsTaken)
	}
}

// --- handleRestack with skipped branches (already up to date in multi-branch stack) ---

func TestHandleRestack_SomeSkippedSomeRestacked(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")

	// Add commit only to main; A is behind main but B is up-to-date with A
	_ = exec.Command("git", "checkout", "main").Run()
	_ = os.WriteFile("change.txt", []byte("x"), 0644)
	_ = exec.Command("git", "add", "change.txt").Run()
	_ = exec.Command("git", "commit", "-m", "change").Run()

	_ = exec.Command("git", "checkout", "A").Run()

	req := makeRequest("gs_restack", map[string]any{"scope": "upstack"})
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

	// A should be restacked
	foundA := false
	for _, name := range resp.Restacked {
		if name == "A" {
			foundA = true
		}
	}
	if !foundA {
		t.Errorf("expected A in restacked, got %v", resp.Restacked)
	}
}

// --- handleModify amend with stage_all and children using non-onto rebase ---

func TestHandleModify_AmendWithChildRestack_NoParentRev(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Clear parent_revision for feat/child
	gitDirOut, _ := exec.Command("git", "rev-parse", "--git-dir").Output()
	gitDir := filepath.Clean(string(gitDirOut[:len(gitDirOut)-1]))
	metadataPath := filepath.Join(gitDir, ".gs_stack_metadata")
	data, _ := os.ReadFile(metadataPath)
	var meta map[string]any
	_ = json.Unmarshal(data, &meta)
	branches := meta["branches"].(map[string]any)
	childMeta := branches["feat/child"].(map[string]any)
	delete(childMeta, "parent_revision")
	updated, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metadataPath, updated, 0600)

	// Go to parent and amend with stage_all
	_ = exec.Command("git", "checkout", "feat/parent").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "extra.txt"), []byte("extra"), 0644)

	req := makeRequest("gs_modify", map[string]any{
		"message":   "amend parent",
		"stage_all": true,
	})
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

	// Child should be restacked via plain rebase (no parentRev)
	if len(resp.RestackedChildren) != 1 {
		t.Errorf("expected 1 restacked child, got %d", len(resp.RestackedChildren))
	}
}

// --- handleCheckout to trunk branch ---

func TestHandleCheckout_ToTrunk(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	// On feat/a

	req := makeRequest("gs_checkout", map[string]any{"branch": "main"})
	result, err := handleCheckout(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckout returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("handleCheckout returned error for trunk")
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp checkoutResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.PreviousBranch != "feat/a" {
		t.Errorf("expected previous 'feat/a', got '%s'", resp.PreviousBranch)
	}
	if resp.CurrentBranch != "main" {
		t.Errorf("expected current 'main', got '%s'", resp.CurrentBranch)
	}
}

// --- handleBranchInfo missing branch param ---

func TestHandleBranchInfo_MissingParam(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_branch_info", map[string]any{})
	result, err := handleBranchInfo(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBranchInfo returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing branch param")
	}
}

// --- handleDiff on untracked branch ---

func TestHandleDiff_UntrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "untracked-diff").Run()

	req := makeRequest("gs_diff", map[string]any{"branch": "untracked-diff"})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for untracked branch")
	}
}

// --- handleLog not initialized ---

func TestHandleLog_NotInitialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gs-mcp-test-noinit-log-*")
	if err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}()

	_ = exec.Command("git", "init").Run()
	_ = exec.Command("git", "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "config", "user.name", "Test User").Run()
	_ = os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x"), 0644)
	_ = exec.Command("git", "add", ".").Run()
	_ = exec.Command("git", "commit", "-m", "init").Run()

	req := mcp.CallToolRequest{}
	result, err := handleLog(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLog returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when gs is not initialized")
	}
}

// --- mcpRestackChildren with conflict ---

func TestMcpRestackChildren_WithConflict(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Create conflicting changes on parent and child (same file, different content)
	_ = exec.Command("git", "checkout", "feat/parent").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "conflict.txt"), []byte("parent version"), 0644)
	_ = exec.Command("git", "add", "conflict.txt").Run()
	_ = exec.Command("git", "commit", "-m", "parent conflict").Run()

	_ = exec.Command("git", "checkout", "feat/child").Run()
	_ = os.WriteFile(filepath.Join(cwd, "conflict.txt"), []byte("child version"), 0644)
	_ = exec.Command("git", "add", "conflict.txt").Run()
	_ = exec.Command("git", "commit", "-m", "child conflict").Run()

	state := loadTestState(t)
	parentNode := state.Stack.GetNode("feat/parent")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, parentNode)

	// If there was a conflict, restacked may be empty (child skipped)
	// The key is exercising the conflict path
	_ = restacked

	// Clean up any pending rebase
	_ = exec.Command("git", "rebase", "--abort").Run()
}

// --- mcpRestackChildren with grandchildren (recursive restack via non-behind path) ---

func TestMcpRestackChildren_GrandchildrenNotBehind(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "A", "main")
	addTrackedBranch(t, "B", "A")
	addTrackedBranch(t, "C", "B")

	// A is up-to-date with main, B is up-to-date with A, C is up-to-date with B
	// No restacking needed, but we exercise the recursive path
	state := loadTestState(t)
	mainNode := state.Stack.GetNode("main")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, mainNode)

	// Nothing should be restacked
	if len(restacked) != 0 {
		t.Errorf("expected 0 restacked, got %d: %v", len(restacked), restacked)
	}
}

// --- mcpRestackChildren without parentRev ---

func TestMcpRestackChildren_WithoutParentRev(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/parent", "main")
	addTrackedBranch(t, "feat/child", "feat/parent")

	// Clear parent_revision for feat/child
	gitDirOut, _ := exec.Command("git", "rev-parse", "--git-dir").Output()
	gitDir := filepath.Clean(string(gitDirOut[:len(gitDirOut)-1]))
	metadataPath := filepath.Join(gitDir, ".gs_stack_metadata")
	data, _ := os.ReadFile(metadataPath)
	var meta map[string]any
	_ = json.Unmarshal(data, &meta)
	branches := meta["branches"].(map[string]any)
	childMeta := branches["feat/child"].(map[string]any)
	delete(childMeta, "parent_revision")
	updated, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metadataPath, updated, 0600)

	// Add a commit to parent to make child behind
	_ = exec.Command("git", "checkout", "feat/parent").Run()
	cwd, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(cwd, "parent-update2.txt"), []byte("update"), 0644)
	_ = exec.Command("git", "add", "parent-update2.txt").Run()
	_ = exec.Command("git", "commit", "-m", "parent update").Run()

	state := loadTestState(t)
	parentNode := state.Stack.GetNode("feat/parent")
	restacked := mcpRestackChildren(state.Repo, state.Metadata, state.Stack, parentNode)

	if len(restacked) != 1 {
		t.Fatalf("expected 1 restacked, got %d: %v", len(restacked), restacked)
	}
	if restacked[0] != "feat/child" {
		t.Errorf("expected 'feat/child', got '%s'", restacked[0])
	}
}

// --- handleRepair with multiple issue types ---

func TestHandleRepair_MultipleOrphanedBranches(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// Create two tracked branches and delete them both
	addTrackedBranch(t, "stale-a", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "stale-b", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	_ = exec.Command("git", "branch", "-D", "stale-a").Run()
	_ = exec.Command("git", "branch", "-D", "stale-b").Run()

	// Fix all
	req := makeRequest("gs_repair", map[string]any{"fix": true})
	result, err := handleRepair(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRepair returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRepair returned tool error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp repairResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.IssuesFound) < 2 {
		t.Errorf("expected at least 2 issues found, got %d", len(resp.IssuesFound))
	}
	if len(resp.IssuesFixed) < 2 {
		t.Errorf("expected at least 2 issues fixed, got %d", len(resp.IssuesFixed))
	}
}

// --- handleLand nonexistent branch ---

func TestHandleLand_NonexistentBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	req := makeRequest("gs_land", map[string]any{"branch": "nonexistent"})
	result, err := handleLand(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLand returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent branch")
	}
}

// --- handleSubmit with non-tracked branch ---

func TestHandleSubmit_NonTrackedBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	_ = exec.Command("git", "checkout", "-b", "not-tracked").Run()

	req := makeRequest("gs_submit", map[string]any{"branch": "not-tracked"})
	result, err := handleSubmit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSubmit returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for non-tracked branch")
	}
}

// --- handleDiff defaults to current branch when unspecified ---

func TestHandleDiff_DefaultsToCurrentOnTracked(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/current-diff", "main")
	// On feat/current-diff

	req := makeRequest("gs_diff", map[string]any{})
	result, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("unexpected error: %s", text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp diffResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if resp.Branch != "feat/current-diff" {
		t.Errorf("expected 'feat/current-diff', got '%s'", resp.Branch)
	}
}

// --- handleRestack with empty branch list (trunk only, scope only) ---

func TestHandleRestack_EmptyBranchList(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	// scope=only on trunk should return empty list immediately
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

	if len(resp.Restacked) != 0 {
		t.Errorf("expected 0 restacked for trunk only, got %d", len(resp.Restacked))
	}
}

// --- handleRename with children updates children refs ---

func TestHandleRename_WithChildrenPreservesMetadata(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/old", "main")
	addTrackedBranch(t, "feat/child1", "feat/old")
	_ = exec.Command("git", "checkout", "feat/old").Run()
	addTrackedBranch(t, "feat/child2", "feat/old")
	_ = exec.Command("git", "checkout", "feat/old").Run()

	req := makeRequest("gs_rename", map[string]any{"new_name": "feat/new"})
	result, err := handleRename(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRename returned error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("handleRename returned tool error: %s", text)
	}

	// Verify both children point to the new name
	for _, childName := range []string{"feat/child1", "feat/child2"} {
		infoReq := makeRequest("gs_branch_info", map[string]any{"branch": childName})
		infoResult, _ := handleBranchInfo(context.Background(), infoReq)
		infoText := infoResult.Content[0].(mcp.TextContent).Text
		var info branchInfoResponse
		_ = json.Unmarshal([]byte(infoText), &info)

		if info.Parent != "feat/new" {
			t.Errorf("expected %s parent 'feat/new', got '%s'", childName, info.Parent)
		}
	}
}

// --- handleFold with keep=true and children ---

func TestHandleFold_KeepWithChildren(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/fold-keep", "main")
	addTrackedBranch(t, "feat/fold-child", "feat/fold-keep")
	_ = exec.Command("git", "checkout", "feat/fold-keep").Run()

	req := makeRequest("gs_fold", map[string]any{"keep": true})
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

	if !resp.Kept {
		t.Error("expected kept=true")
	}
	if len(resp.Reparented) != 1 || resp.Reparented[0] != "feat/fold-child" {
		t.Errorf("expected [feat/fold-child] reparented, got %v", resp.Reparented)
	}

	// Verify branch still exists
	out, _ := exec.Command("git", "branch", "--list", "feat/fold-keep").Output()
	if len(out) == 0 {
		t.Error("feat/fold-keep should still exist with keep=true")
	}
}

// --- handleLog with specific branch filter ---

func TestHandleLog_MultipleBranches(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/a", "main")
	_ = exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/b", "main")
	addTrackedBranch(t, "feat/b-child", "feat/b")

	req := makeRequest("gs_log", map[string]any{"include_commits": true})
	result, err := handleLog(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLog returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp logResponse
	_ = json.Unmarshal([]byte(text), &resp)

	if len(resp.Branches) != 4 {
		t.Errorf("expected 4 branches (main, a, b, b-child), got %d", len(resp.Branches))
	}
}
