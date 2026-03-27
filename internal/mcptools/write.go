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
	s.AddTool(restackTool, handleRestack)
	s.AddTool(modifyTool, handleModify)
	s.AddTool(moveTool, handleMove)
	s.AddTool(foldTool, handleFold)
}

// --- gs_checkout ---

var checkoutTool = mcp.NewTool("gs_checkout",
	mcp.WithDescription(`Switch to a specific branch by name. Works with any git branch, including branches not tracked by gs.

Use this when you know the exact branch name. For relative navigation within the stack (move to parent, child, or leaf), use gs_navigate instead — it understands the stack structure and moves along parent-child edges.

Returns: {previous_branch, current_branch}`),
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
		return errResult(fmt.Sprintf("branch '%s' does not exist — call gs_status to see all tracked branches", branchName)), nil
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
	mcp.WithDescription(`Move through the stack along parent-child edges. Direction meanings:
- "down": toward trunk (to the parent branch)
- "up": toward leaves (to a child branch)
- "bottom": jump directly to trunk
- "top": jump to the leaf of the current stack line

The steps parameter (default 1) only applies to "up" and "down" directions.

IMPORTANT: If the current branch has multiple children, "up" navigation is ambiguous. Instead of choosing, the tool returns {error: "ambiguous_navigation", options: [...]} listing the child branch names. Use gs_checkout to pick one.

Use gs_navigate for relative movement within a stack. Use gs_checkout when you know the exact branch name.

Returns on success: {previous_branch, current_branch, steps_taken}
Returns on ambiguity: {error: "ambiguous_navigation", direction, options[], message}`),
	mcp.WithString("direction",
		mcp.Required(),
		mcp.Description("Direction to navigate: up (toward leaves), down (toward trunk), top (to leaf), bottom (to trunk)"),
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
	mcp.WithDescription(`Create a new stacked branch on top of the CURRENT branch. The current branch becomes the new branch's parent.

IMPORTANT: Switch to the desired parent branch FIRST using gs_checkout or gs_navigate before calling gs_create.

If commit_message is provided and there are staged changes, those changes are committed on the new branch. If commit_message is provided but there are no staged changes, commit_created will be false in the response.

To create a stack: call gs_create repeatedly — each new branch stacks on top of the previous one.

Returns: {branch, parent, commit_created}`),
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
	mcp.WithDescription(`Delete a branch from the stack and from git. Children of the deleted branch are automatically reparented to the deleted branch's parent, preserving the stack structure.

If you delete the branch you're currently on, gs automatically checks out the parent branch first.

Use this to clean up merged or abandoned branches. After deleting, consider calling gs_restack with scope "all" to ensure reparented children are properly rebased onto their new parent.

Cannot delete the trunk branch. Always force-deletes (equivalent to git branch -D).

Returns: {deleted, reparented_children[], new_parent, checked_out?}`),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to delete"),
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
	mcp.WithDescription(`Start tracking an existing git branch in the gs stack by declaring its parent. Use this to adopt branches that were created outside of gs (e.g., with plain git checkout -b).

Both the branch and the parent must already exist as git branches. The branch must not already be tracked.

After tracking, call gs_restack with scope "only" on the newly tracked branch to rebase it onto its declared parent if needed.

Returns: {branch, parent}`),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the existing git branch to start tracking"),
	),
	mcp.WithString("parent",
		mcp.Required(),
		mcp.Description("Name of the parent branch in the stack"),
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
	mcp.WithDescription(`Stop tracking a branch in the gs stack. The git branch itself is NOT deleted — it just stops appearing in gs_status and gs_log.

WARNING: If this branch has children in the stack, those children will become orphaned. Consider using gs_move to reparent children before untracking, or use gs_delete instead which handles reparenting automatically.

Cannot untrack the trunk branch.

Returns: {branch, warnings[]}`),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to stop tracking"),
	),
)

type untrackResponse struct {
	Branch   string   `json:"branch"`
	Warnings []string `json:"warnings,omitempty"`
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

	// Check for orphaned children before untracking
	var warnings []string
	children := state.Metadata.GetChildren(branchName)
	if len(children) > 0 {
		warnings = append(warnings, fmt.Sprintf("branch had %d children (%s) that are now orphaned — use gs_move to reparent them", len(children), fmt.Sprintf("%v", children)))
	}

	state.Metadata.UntrackBranch(branchName)
	if err := state.Metadata.Save(state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}

	return jsonResult(untrackResponse{Branch: branchName, Warnings: warnings})
}

// --- gs_rename ---

var renameTool = mcp.NewTool("gs_rename",
	mcp.WithDescription(`Rename the CURRENT branch (both the git branch and all gs tracking metadata). All parent/child references from other branches are updated automatically.

Only works on the current branch. To rename a different branch, call gs_checkout first to switch to it.

Cannot rename the trunk branch. The new name must not collide with an existing branch.

Returns: {old_name, new_name}`),
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

// --- gs_restack ---

var restackTool = mcp.NewTool("gs_restack",
	mcp.WithDescription("Rebase branches to maintain parent-child relationships. Rebases each branch onto its parent using precise --onto when possible. Returns the list of restacked branches or conflict info."),
	mcp.WithString("branch",
		mcp.Description("Branch to start from (defaults to current)"),
	),
	mcp.WithString("scope",
		mcp.Description("Scope of restacking"),
		mcp.Enum("only", "upstack", "downstack", "all"),
	),
)

type restackResponse struct {
	Restacked []string `json:"restacked"`
	Skipped   []string `json:"skipped"`
	Conflict  string   `json:"conflict,omitempty"`
}

func handleRestack(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	// Check for uncommitted changes
	dirty, err := state.Repo.HasUncommittedChanges()
	if err != nil {
		return errResult(fmt.Sprintf("failed to check working tree: %v", err)), nil
	}
	if dirty {
		return errResult("uncommitted changes — commit or stash before restacking"), nil
	}

	branchName := req.GetString("branch", state.Stack.Current)
	scope := req.GetString("scope", "all")

	if state.Stack.GetNode(branchName) == nil && branchName != state.Config.Trunk {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", branchName)), nil
	}

	// Compute branches to restack
	branches := computeMCPRestackBranches(state, branchName, scope)
	if len(branches) == 0 {
		return jsonResult(restackResponse{Restacked: []string{}, Skipped: []string{}})
	}

	originalBranch := state.Stack.Current
	restacked := []string{}
	skipped := []string{}

	for _, branch := range branches {
		node := state.Stack.GetNode(branch)
		if node == nil || node.Parent == nil {
			continue
		}

		parent := node.Parent.Name

		if err := state.Repo.CheckoutBranch(branch); err != nil {
			return errResult(fmt.Sprintf("failed to checkout '%s': %v", branch, err)), nil
		}

		// Check if needs rebase
		behind, err := state.Repo.IsBehind(branch, parent)
		if err != nil {
			return errResult(fmt.Sprintf("failed to check rebase status: %v", err)), nil
		}

		if !behind {
			skipped = append(skipped, branch)
			continue
		}

		// Perform rebase using --onto when possible
		parentRev := state.Metadata.GetParentRevision(branch)
		var rebaseErr error
		if parentRev != "" {
			rebaseErr = state.Repo.RebaseOnto(branch, parent, parentRev)
		} else {
			rebaseErr = state.Repo.Rebase(branch, parent)
		}

		if rebaseErr != nil {
			return jsonResult(restackResponse{
				Restacked: restacked,
				Skipped:   skipped,
				Conflict:  fmt.Sprintf("conflict restacking '%s' onto '%s' — resolve conflicts, then run gs continue", branch, parent),
			})
		}

		// Update parent revision
		parentSHA, _ := state.Repo.GetBranchCommit(parent)
		state.Metadata.SetParentRevision(branch, parentSHA)
		state.Metadata.Save(state.Repo.GetMetadataPath())

		restacked = append(restacked, branch)
	}

	// Return to original branch
	state.Repo.CheckoutBranch(originalBranch)

	return jsonResult(restackResponse{Restacked: restacked, Skipped: skipped})
}

func computeMCPRestackBranches(state *repoState, branch, scope string) []string {
	switch scope {
	case "only":
		if branch == state.Config.Trunk {
			return nil
		}
		return []string{branch}

	case "upstack":
		result := []string{}
		if branch != state.Config.Trunk {
			result = append(result, branch)
		}
		result = append(result, descendantsDFS(state.Stack, branch)...)
		return result

	case "downstack":
		return ancestorsOf(state.Stack, state.Config.Trunk, branch)

	default: // "all"
		if branch == state.Config.Trunk {
			return allTopological(state.Stack)
		}
		ancestors := ancestorsOf(state.Stack, state.Config.Trunk, branch)
		descendants := descendantsDFS(state.Stack, branch)
		return append(ancestors, descendants...)
	}
}

func allTopological(s *stack.Stack) []string {
	nodes := s.GetTopologicalOrder()
	result := make([]string, len(nodes))
	for i, n := range nodes {
		result[i] = n.Name
	}
	return result
}

func ancestorsOf(s *stack.Stack, trunk, branch string) []string {
	path := s.FindPath(branch)
	var result []string
	for _, n := range path {
		if n.Name == trunk {
			continue
		}
		result = append(result, n.Name)
	}
	return result
}

func descendantsDFS(s *stack.Stack, branch string) []string {
	node := s.GetNode(branch)
	if node == nil {
		return nil
	}
	var result []string
	for _, child := range node.SortedChildren() {
		result = append(result, child.Name)
		result = append(result, descendantsDFS(s, child.Name)...)
	}
	return result
}

// --- gs_modify ---

var modifyTool = mcp.NewTool("gs_modify",
	mcp.WithDescription("Amend the current branch's last commit (or create a new commit) and restack children. Stage changes with --all before committing if needed."),
	mcp.WithString("message",
		mcp.Description("Commit message (for amend or new commit)"),
	),
	mcp.WithBoolean("new_commit",
		mcp.Description("Create a new commit instead of amending (default: false)"),
	),
	mcp.WithBoolean("all",
		mcp.Description("Stage all changes before committing (default: false)"),
	),
)

type modifyResponse struct {
	Branch          string   `json:"branch"`
	Action          string   `json:"action"`
	RestackedChildren []string `json:"restacked_children"`
}

func handleModify(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	message := req.GetString("message", "")
	newCommit := req.GetBool("new_commit", false)
	stageAll := req.GetBool("all", false)

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	currentBranch := state.Stack.Current
	node := state.Stack.GetNode(currentBranch)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", currentBranch)), nil
	}

	// Stage all if requested
	if stageAll {
		if _, err := state.Repo.RunGitCommand("add", "-A"); err != nil {
			return errResult(fmt.Sprintf("failed to stage changes: %v", err)), nil
		}
	}

	// Build git commit args
	action := "amended"
	args := []string{"commit"}
	if newCommit {
		action = "committed"
	} else {
		args = append(args, "--amend")
	}
	if message != "" {
		args = append(args, "-m", message)
	} else if !newCommit {
		args = append(args, "--no-edit")
	} else {
		return errResult("message is required when creating a new commit"), nil
	}

	if _, err := state.Repo.RunGitCommand(args...); err != nil {
		return errResult(fmt.Sprintf("failed to %s: %v", action, err)), nil
	}

	// Restack children
	restackedChildren := []string{}
	if len(node.Children) > 0 {
		for _, child := range node.Children {
			if err := state.Repo.CheckoutBranch(child.Name); err != nil {
				continue
			}
			behind, err := state.Repo.IsBehind(child.Name, currentBranch)
			if err != nil || !behind {
				continue
			}
			parentRev := state.Metadata.GetParentRevision(child.Name)
			if parentRev != "" {
				err = state.Repo.RebaseOnto(child.Name, currentBranch, parentRev)
			} else {
				err = state.Repo.Rebase(child.Name, currentBranch)
			}
			if err == nil {
				restackedChildren = append(restackedChildren, child.Name)
				parentSHA, _ := state.Repo.GetBranchCommit(currentBranch)
				state.Metadata.SetParentRevision(child.Name, parentSHA)
			}
		}
		state.Metadata.Save(state.Repo.GetMetadataPath())
		state.Repo.CheckoutBranch(currentBranch)
	}

	return jsonResult(modifyResponse{
		Branch:            currentBranch,
		Action:            action,
		RestackedChildren: restackedChildren,
	})
}

// --- gs_move ---

var moveTool = mcp.NewTool("gs_move",
	mcp.WithDescription("Move a branch to a new parent. Rebases the branch onto the new parent and restacks descendants."),
	mcp.WithString("branch",
		mcp.Description("Branch to move (defaults to current)"),
	),
	mcp.WithString("onto",
		mcp.Required(),
		mcp.Description("New parent branch"),
	),
)

type moveResponse struct {
	Branch    string `json:"branch"`
	OldParent string `json:"old_parent"`
	NewParent string `json:"new_parent"`
}

func handleMove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	onto, err := req.RequireString("onto")
	if err != nil {
		return errResult("missing required parameter: onto"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	branchName := req.GetString("branch", state.Stack.Current)

	if branchName == state.Config.Trunk {
		return errResult("cannot move trunk branch"), nil
	}

	node := state.Stack.GetNode(branchName)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", branchName)), nil
	}

	if !state.Repo.BranchExists(onto) {
		return errResult(fmt.Sprintf("target branch '%s' does not exist", onto)), nil
	}

	if onto == branchName {
		return errResult("cannot move a branch onto itself"), nil
	}

	// Check onto is not a descendant
	for _, desc := range descendantsDFS(state.Stack, branchName) {
		if desc == onto {
			return errResult(fmt.Sprintf("cannot move '%s' onto its descendant '%s'", branchName, onto)), nil
		}
	}

	oldParent := ""
	if node.Parent != nil {
		oldParent = node.Parent.Name
	}

	// Update metadata
	if err := state.Metadata.UpdateParent(branchName, onto); err != nil {
		return errResult(fmt.Sprintf("failed to update metadata: %v", err)), nil
	}

	// Checkout and rebase
	if err := state.Repo.CheckoutBranch(branchName); err != nil {
		return errResult(fmt.Sprintf("failed to checkout: %v", err)), nil
	}

	if err := state.Repo.Rebase(branchName, onto); err != nil {
		return errResult(fmt.Sprintf("rebase conflict moving '%s' onto '%s'", branchName, onto)), nil
	}

	// Update parent revision
	ontoSHA, _ := state.Repo.GetBranchCommit(onto)
	state.Metadata.SetParentRevision(branchName, ontoSHA)
	state.Metadata.Save(state.Repo.GetMetadataPath())

	return jsonResult(moveResponse{
		Branch:    branchName,
		OldParent: oldParent,
		NewParent: onto,
	})
}

// --- gs_fold ---

var foldTool = mcp.NewTool("gs_fold",
	mcp.WithDescription("Fold the current branch into its parent using squash merge. Children are reparented to the parent. The branch is deleted unless keep is true."),
	mcp.WithBoolean("keep",
		mcp.Description("Keep the branch after folding (default: false)"),
	),
)

type foldResponse struct {
	Folded     string   `json:"folded"`
	Into       string   `json:"into"`
	Kept       bool     `json:"kept"`
	Reparented []string `json:"reparented_children"`
}

func handleFold(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keep := req.GetBool("keep", false)

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	currentBranch := state.Stack.Current
	node := state.Stack.GetNode(currentBranch)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", currentBranch)), nil
	}

	if node.IsTrunk {
		return errResult("cannot fold trunk branch"), nil
	}
	if node.Parent == nil {
		return errResult("branch has no parent to fold into"), nil
	}

	parentBranch := node.Parent.Name

	// Checkout parent and squash merge
	if err := state.Repo.CheckoutBranch(parentBranch); err != nil {
		return errResult(fmt.Sprintf("failed to checkout parent: %v", err)), nil
	}

	if _, err := state.Repo.RunGitCommand("merge", "--squash", currentBranch); err != nil {
		return errResult(fmt.Sprintf("failed to squash merge: %v", err)), nil
	}

	commitMsg := fmt.Sprintf("Fold %s into %s", currentBranch, parentBranch)
	if _, err := state.Repo.RunGitCommand("commit", "-m", commitMsg); err != nil {
		return errResult(fmt.Sprintf("failed to commit fold: %v", err)), nil
	}

	// Reparent children
	reparented := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		if err := state.Metadata.UpdateParent(child.Name, parentBranch); err == nil {
			reparented = append(reparented, child.Name)
		}
	}

	if !keep {
		// Delete the folded branch
		state.Repo.RunGitCommand("branch", "-D", currentBranch)
		state.Metadata.UntrackBranch(currentBranch)
	}

	state.Metadata.Save(state.Repo.GetMetadataPath())

	return jsonResult(foldResponse{
		Folded:     currentBranch,
		Into:       parentBranch,
		Kept:       keep,
		Reparented: reparented,
	})
}
