package mcptools

import (
	"context"
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerWriteTools(s *server.MCPServer) {
	s.AddTool(checkoutTool, handleCheckout)
	s.AddTool(navigateTool, handleNavigate)
	s.AddTool(createTool, handleCreate)
	s.AddTool(deleteTool, handleDelete)
	s.AddTool(trackTool, handleTrack)
	s.AddTool(untrackTool, handleUntrack)
	s.AddTool(renameTool, handleRename)
}

// --- gs_checkout ---

var checkoutTool = mcp.NewTool("gs_checkout",
	mcp.WithDescription("Switch to a branch in the stack."),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to switch to"),
	),
)

type checkoutResponse struct {
	PreviousBranch string `json:"previous_branch"`
	CurrentBranch  string `json:"current_branch"`
}

func handleCheckout(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	branchName, err := req.RequireString("branch")
	if err != nil {
		return errResult("missing required parameter: branch"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	previousBranch := state.Stack.Current

	if !state.Repo.BranchExists(branchName) {
		return errResult(fmt.Sprintf("branch '%s' does not exist", branchName)), nil
	}

	if err := state.Repo.CheckoutBranch(branchName); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	return jsonResult(checkoutResponse{
		PreviousBranch: previousBranch,
		CurrentBranch:  branchName,
	})
}

// --- gs_navigate ---

var navigateTool = mcp.NewTool("gs_navigate",
	mcp.WithDescription("Move up/down/top/bottom in the stack. Returns the new current branch. If navigation is ambiguous (multiple children when going up), returns the list of options instead of prompting."),
	mcp.WithString("direction",
		mcp.Required(),
		mcp.Description("Direction to navigate"),
		mcp.Enum("up", "down", "top", "bottom"),
	),
	mcp.WithNumber("steps",
		mcp.Description("Number of steps to move (only for up/down, default: 1)"),
	),
)

type navigateResponse struct {
	PreviousBranch string `json:"previous_branch"`
	CurrentBranch  string `json:"current_branch"`
	StepsTaken     int    `json:"steps_taken"`
}

type navigateAmbiguousResponse struct {
	Error     string   `json:"error"`
	Direction string   `json:"direction"`
	Options   []string `json:"options"`
	Message   string   `json:"message"`
}

func handleNavigate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	direction, err := req.RequireString("direction")
	if err != nil {
		return errResult("missing required parameter: direction"), nil
	}

	steps := req.GetInt("steps", 1)
	if steps < 1 {
		steps = 1
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	previousBranch := state.Stack.Current
	node := state.Stack.GetNode(previousBranch)
	if node == nil {
		return errResult(fmt.Sprintf("current branch '%s' is not tracked by gs", previousBranch)), nil
	}

	switch direction {
	case "up":
		return navigateUp(state, node, previousBranch, steps)
	case "down":
		return navigateDown(state, node, previousBranch, steps)
	case "top":
		return navigateTop(state, node, previousBranch)
	case "bottom":
		return navigateBottom(state, previousBranch)
	default:
		return errResult(fmt.Sprintf("invalid direction: %s", direction)), nil
	}
}

func navigateUp(state *repoState, node *stack.Node, previousBranch string, steps int) (*mcp.CallToolResult, error) {
	current := node
	stepsTaken := 0

	for i := 0; i < steps; i++ {
		children := current.Children
		if len(children) == 0 {
			break
		}
		if len(children) > 1 {
			options := make([]string, len(children))
			for j, child := range children {
				options[j] = child.Name
			}
			return jsonResult(navigateAmbiguousResponse{
				Error:     "ambiguous_navigation",
				Direction: "up",
				Options:   options,
				Message:   fmt.Sprintf("branch '%s' has %d children — use gs_checkout to pick one", current.Name, len(children)),
			})
		}
		current = children[0]
		stepsTaken++
	}

	if stepsTaken == 0 {
		return errResult(fmt.Sprintf("already at top of stack (branch '%s' has no children)", previousBranch)), nil
	}

	if err := state.Repo.CheckoutBranch(current.Name); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	return jsonResult(navigateResponse{
		PreviousBranch: previousBranch,
		CurrentBranch:  current.Name,
		StepsTaken:     stepsTaken,
	})
}

func navigateDown(state *repoState, node *stack.Node, previousBranch string, steps int) (*mcp.CallToolResult, error) {
	current := node
	stepsTaken := 0

	for i := 0; i < steps; i++ {
		if current.Parent == nil {
			break
		}
		current = current.Parent
		stepsTaken++
	}

	if stepsTaken == 0 {
		return errResult(fmt.Sprintf("already at bottom of stack (branch '%s' is trunk)", previousBranch)), nil
	}

	if err := state.Repo.CheckoutBranch(current.Name); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	return jsonResult(navigateResponse{
		PreviousBranch: previousBranch,
		CurrentBranch:  current.Name,
		StepsTaken:     stepsTaken,
	})
}

func navigateTop(state *repoState, node *stack.Node, previousBranch string) (*mcp.CallToolResult, error) {
	leaves := findLeaves(node)

	if len(leaves) == 0 {
		return errResult(fmt.Sprintf("already at top of stack (branch '%s' has no children)", previousBranch)), nil
	}

	if len(leaves) > 1 {
		options := make([]string, len(leaves))
		for i, leaf := range leaves {
			options[i] = leaf.Name
		}
		return jsonResult(navigateAmbiguousResponse{
			Error:     "ambiguous_navigation",
			Direction: "top",
			Options:   options,
			Message:   fmt.Sprintf("multiple leaf branches reachable from '%s' — use gs_checkout to pick one", previousBranch),
		})
	}

	if err := state.Repo.CheckoutBranch(leaves[0].Name); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	depth := 0
	for n := leaves[0]; n != node; n = n.Parent {
		depth++
	}

	return jsonResult(navigateResponse{
		PreviousBranch: previousBranch,
		CurrentBranch:  leaves[0].Name,
		StepsTaken:     depth,
	})
}

func navigateBottom(state *repoState, previousBranch string) (*mcp.CallToolResult, error) {
	if previousBranch == state.Stack.TrunkName {
		return errResult("already at trunk"), nil
	}

	if err := state.Repo.CheckoutBranch(state.Stack.TrunkName); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	// Count steps from previous to trunk
	node := state.Stack.GetNode(previousBranch)
	depth := 0
	for n := node; n != nil && !n.IsTrunk; n = n.Parent {
		depth++
	}

	return jsonResult(navigateResponse{
		PreviousBranch: previousBranch,
		CurrentBranch:  state.Stack.TrunkName,
		StepsTaken:     depth,
	})
}

// findLeaves returns all leaf nodes (no children) reachable from the given node's subtree.
// Does not include the node itself if it has no children.
func findLeaves(node *stack.Node) []*stack.Node {
	if len(node.Children) == 0 {
		return nil
	}
	var leaves []*stack.Node
	var walk func(n *stack.Node)
	walk = func(n *stack.Node) {
		if len(n.Children) == 0 {
			leaves = append(leaves, n)
			return
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	for _, child := range node.Children {
		walk(child)
	}
	return leaves
}

// --- gs_create ---

var createTool = mcp.NewTool("gs_create",
	mcp.WithDescription("Create a new stacked branch on top of the current branch. Optionally commit staged changes with a message."),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name for the new branch"),
	),
	mcp.WithString("commit_message",
		mcp.Description("If provided, commit staged changes with this message on the new branch"),
	),
)

type createResponse struct {
	Branch        string `json:"branch"`
	Parent        string `json:"parent"`
	CommitCreated bool   `json:"commit_created"`
}

func handleCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return errResult("missing required parameter: name"), nil
	}

	commitMsg := req.GetString("commit_message", "")

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	parentBranch := state.Stack.Current

	if state.Repo.BranchExists(name) {
		return errResult(fmt.Sprintf("branch '%s' already exists", name)), nil
	}

	// Create and checkout the new branch
	if err := state.Repo.CreateBranch(name); err != nil {
		return errResult(fmt.Sprintf("failed to create branch: %v", err)), nil
	}
	if err := state.Repo.CheckoutBranch(name); err != nil {
		return errResult(fmt.Sprintf("failed to checkout branch: %v", err)), nil
	}

	// Track in metadata
	parentSHA, _ := state.Repo.GetBranchCommit(parentBranch)
	state.Metadata.TrackBranch(name, parentBranch, parentSHA)
	if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}

	// Optionally commit
	commitCreated := false
	if commitMsg != "" {
		_, err := state.Repo.RunGitCommand("commit", "-m", commitMsg)
		if err != nil {
			// Commit failed (probably no staged changes) — not fatal
			commitCreated = false
		} else {
			commitCreated = true
		}
	}

	return jsonResult(createResponse{
		Branch:        name,
		Parent:        parentBranch,
		CommitCreated: commitCreated,
	})
}

// --- gs_delete ---

var deleteTool = mcp.NewTool("gs_delete",
	mcp.WithDescription("Delete a branch from the stack. Children are reparented to the deleted branch's parent. If deleting the current branch, checks out the parent first."),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to delete"),
	),
	mcp.WithBoolean("force",
		mcp.Description("Force delete even if branch has unmerged changes (default: true for MCP)"),
	),
)

type deleteResponse struct {
	Deleted           string   `json:"deleted"`
	ReparentedChildren []string `json:"reparented_children"`
	NewParent         string   `json:"new_parent"`
	CheckedOut        string   `json:"checked_out,omitempty"`
}

func handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	branchName, err := req.RequireString("branch")
	if err != nil {
		return errResult("missing required parameter: branch"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	// Validate
	if branchName == state.Config.Trunk {
		return errResult("cannot delete trunk branch"), nil
	}
	if !state.Repo.BranchExists(branchName) {
		return errResult(fmt.Sprintf("branch '%s' does not exist", branchName)), nil
	}
	if !state.Metadata.IsTracked(branchName) {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", branchName)), nil
	}

	node := state.Stack.GetNode(branchName)
	if node == nil {
		return errResult("branch not found in stack"), nil
	}

	parentBranch, ok := state.Metadata.GetParent(branchName)
	if !ok {
		return errResult("branch has no parent"), nil
	}

	// If deleting current branch, checkout parent first
	checkedOut := ""
	if branchName == state.Stack.Current {
		if err := state.Repo.CheckoutBranch(parentBranch); err != nil {
			return errResult(fmt.Sprintf("failed to checkout parent: %v", err)), nil
		}
		checkedOut = parentBranch
	}

	// Reparent children
	reparented := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		if err := state.Metadata.UpdateParent(child.Name, parentBranch); err != nil {
			return errResult(fmt.Sprintf("failed to reparent '%s': %v", child.Name, err)), nil
		}
		reparented = append(reparented, child.Name)
	}

	// Delete the git branch
	if _, err := state.Repo.RunGitCommand("branch", "-D", branchName); err != nil {
		return errResult(fmt.Sprintf("failed to delete branch: %v", err)), nil
	}

	// Remove from metadata
	state.Metadata.UntrackBranch(branchName)
	if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}

	return jsonResult(deleteResponse{
		Deleted:            branchName,
		ReparentedChildren: reparented,
		NewParent:          parentBranch,
		CheckedOut:         checkedOut,
	})
}

// --- gs_track ---

var trackTool = mcp.NewTool("gs_track",
	mcp.WithDescription("Start tracking an existing git branch in the stack by specifying its parent."),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the existing branch to track"),
	),
	mcp.WithString("parent",
		mcp.Required(),
		mcp.Description("Name of the parent branch"),
	),
)

type trackResponse struct {
	Branch string `json:"branch"`
	Parent string `json:"parent"`
}

func handleTrack(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	branchName, err := req.RequireString("branch")
	if err != nil {
		return errResult("missing required parameter: branch"), nil
	}
	parentName, err := req.RequireString("parent")
	if err != nil {
		return errResult("missing required parameter: parent"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	if !state.Repo.BranchExists(branchName) {
		return errResult(fmt.Sprintf("branch '%s' does not exist", branchName)), nil
	}
	if !state.Repo.BranchExists(parentName) {
		return errResult(fmt.Sprintf("parent branch '%s' does not exist", parentName)), nil
	}
	if state.Metadata.IsTracked(branchName) {
		return errResult(fmt.Sprintf("branch '%s' is already tracked", branchName)), nil
	}

	parentSHA, _ := state.Repo.GetBranchCommit(parentName)
	state.Metadata.TrackBranch(branchName, parentName, parentSHA)
	if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}

	return jsonResult(trackResponse{Branch: branchName, Parent: parentName})
}

// --- gs_untrack ---

var untrackTool = mcp.NewTool("gs_untrack",
	mcp.WithDescription("Stop tracking a branch in the stack. The git branch is not deleted."),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to untrack"),
	),
)

type untrackResponse struct {
	Branch string `json:"branch"`
}

func handleUntrack(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	branchName, err := req.RequireString("branch")
	if err != nil {
		return errResult("missing required parameter: branch"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	if branchName == state.Config.Trunk {
		return errResult("cannot untrack trunk branch"), nil
	}
	if !state.Metadata.IsTracked(branchName) {
		return errResult(fmt.Sprintf("branch '%s' is not tracked", branchName)), nil
	}

	state.Metadata.UntrackBranch(branchName)
	if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}

	return jsonResult(untrackResponse{Branch: branchName})
}

// --- gs_rename ---

var renameTool = mcp.NewTool("gs_rename",
	mcp.WithDescription("Rename the current branch and update gs tracking metadata."),
	mcp.WithString("new_name",
		mcp.Required(),
		mcp.Description("New name for the current branch"),
	),
)

type renameResponse struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func handleRename(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	newName, err := req.RequireString("new_name")
	if err != nil {
		return errResult("missing required parameter: new_name"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	currentBranch := state.Stack.Current

	if currentBranch == state.Config.Trunk {
		return errResult("cannot rename trunk branch"), nil
	}
	if newName == currentBranch {
		return errResult(fmt.Sprintf("branch is already named '%s'", newName)), nil
	}
	if state.Repo.BranchExists(newName) {
		return errResult(fmt.Sprintf("branch '%s' already exists", newName)), nil
	}

	// Rename git branch
	if _, err := state.Repo.RunGitCommand("branch", "-m", currentBranch, newName); err != nil {
		return errResult(fmt.Sprintf("failed to rename branch: %v", err)), nil
	}

	// Update metadata
	if state.Metadata.IsTracked(currentBranch) {
		parent, _ := state.Metadata.GetParent(currentBranch)
		children := state.Metadata.GetChildren(currentBranch)
		existingParentRev := state.Metadata.GetParentRevision(currentBranch)

		state.Metadata.UntrackBranch(currentBranch)
		state.Metadata.TrackBranch(newName, parent, existingParentRev)

		// Update children to point to new parent name
		for _, child := range children {
			childParentRev := state.Metadata.GetParentRevision(child)
			state.Metadata.TrackBranch(child, newName, childParentRev)
		}

		if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
			// Rollback git rename
			state.Repo.RunGitCommand("branch", "-m", newName, currentBranch)
			return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
		}
	}

	return jsonResult(renameResponse{OldName: currentBranch, NewName: newName})
}
