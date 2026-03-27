package mcptools

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- gs_checkout tests ---

func TestHandleCheckout_SwitchBranch(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	exec.Command("git", "checkout", "main").Run()

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
	json.Unmarshal([]byte(text), &resp)

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
	exec.Command("git", "checkout", "main").Run()

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
	json.Unmarshal([]byte(text), &resp)

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
	exec.Command("git", "checkout", "main").Run()
	addTrackedBranch(t, "feat/ui", "main")
	exec.Command("git", "checkout", "main").Run()

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
	json.Unmarshal([]byte(text), &resp)

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
	json.Unmarshal([]byte(text), &resp)

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
	exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "top"})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	json.Unmarshal([]byte(text), &resp)

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
	json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "main" {
		t.Errorf("expected bottom to be 'main', got '%s'", resp.CurrentBranch)
	}
}

func TestHandleNavigate_UpMultipleSteps(t *testing.T) {
	cleanup := setupTestRepo(t)
	defer cleanup()

	addTrackedBranch(t, "feat/auth", "main")
	addTrackedBranch(t, "feat/auth-tests", "feat/auth")
	exec.Command("git", "checkout", "main").Run()

	req := makeRequest("gs_navigate", map[string]any{"direction": "up", "steps": 2})
	result, err := handleNavigate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleNavigate returned error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp navigateResponse
	json.Unmarshal([]byte(text), &resp)

	if resp.CurrentBranch != "feat/auth-tests" {
		t.Errorf("expected 'feat/auth-tests' after 2 steps up, got '%s'", resp.CurrentBranch)
	}
	if resp.StepsTaken != 2 {
		t.Errorf("expected 2 steps, got %d", resp.StepsTaken)
	}
}
