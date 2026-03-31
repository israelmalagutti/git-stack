package mcptools

import (
	"context"
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/land"
	"github.com/israelmalagutti/git-stack/internal/ops"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/repair"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/israelmalagutti/git-stack/internal/submit"
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
	s.AddTool(repairTool, handleRepair)
	s.AddTool(submitTool, handleSubmit)
	s.AddTool(landTool, handleLand)
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

	result, err := ops.CreateBranch(state.Repo, state.Metadata, name, state.Stack.Current)
	if err != nil {
		return errResult(err.Error()), nil
	}

	// Optionally commit
	commitCreated := false
	if commitMsg != "" {
		if _, err := state.Repo.RunGitCommand("commit", "-m", commitMsg); err == nil {
			commitCreated = true
		}
	}

	return jsonResult(createResponse{
		Branch:        result.Branch,
		Parent:        result.Parent,
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
	Deleted            string   `json:"deleted"`
	ReparentedChildren []string `json:"reparented_children"`
	NewParent          string   `json:"new_parent"`
	CheckedOut         string   `json:"checked_out,omitempty"`
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

	result, err := ops.DeleteBranch(state.Repo, state.Metadata, state.Stack, branchName)
	if err != nil {
		return errResult(err.Error()), nil
	}

	return jsonResult(deleteResponse{
		Deleted:            result.Deleted,
		ReparentedChildren: result.ReparentedChildren,
		NewParent:          result.NewParent,
		CheckedOut:         result.CheckedOut,
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
	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}
	pushMetadataRefs(state.Repo, branchName)

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
	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
	}
	deleteRemoteMetadataRef(state.Repo, branchName)

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

		if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
			// Rollback git rename
			_, _ = state.Repo.RunGitCommand("branch", "-m", newName, currentBranch)
			return errResult(fmt.Sprintf("failed to save metadata: %v", err)), nil
		}
		deleteRemoteMetadataRef(state.Repo, currentBranch)
		pushMetadataRefs(state.Repo, newName)
		if len(children) > 0 {
			pushMetadataRefs(state.Repo, children...)
		}
	}

	return jsonResult(renameResponse{OldName: currentBranch, NewName: newName})
}

// --- gs_restack ---

var restackTool = mcp.NewTool("gs_restack",
	mcp.WithDescription(`Rebase branches to align them with their declared parents in the stack. This is the key operation for keeping a stack consistent after modifications.

PREREQUISITE: Working tree must be clean (no uncommitted changes). Commit or stash changes first.

Scope controls which branches are restacked:
- "only": just the specified branch onto its parent
- "upstack": the branch and all its descendants (children, grandchildren, etc.)
- "downstack": ancestors of the branch (between trunk and the branch)
- "all" (default): the entire stack

If a rebase conflict occurs, the tool stops and returns conflict info. Resolve conflicts in the working tree using git commands (edit files, git add, git rebase --continue), then call gs_restack again.

Common triggers: after gs_modify, gs_move, gs_delete, or pulling upstream changes to trunk.

Returns: {restacked[], skipped[], conflict?}`),
	mcp.WithString("branch",
		mcp.Description("Branch to start from (defaults to current branch)"),
	),
	mcp.WithString("scope",
		mcp.Description("Scope: only (single branch), upstack (branch + descendants), downstack (ancestors to trunk), all (entire stack, default)"),
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
		return errResult("uncommitted changes — commit changes with gs_modify (with stage_all=true), or stash them before calling gs_restack"), nil
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
				Conflict:  fmt.Sprintf("conflict restacking '%s' onto '%s' — resolve conflicts in the working tree (edit files, git add, git rebase --continue), then call gs_restack again", branch, parent),
			})
		}

		// Update parent revision
		parentSHA, _ := state.Repo.GetBranchCommit(parent)
		_ = state.Metadata.SetParentRevision(branch, parentSHA)
		if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
			return errResult(fmt.Sprintf("failed to save metadata after restacking '%s': %v", branch, err)), nil
		}

		restacked = append(restacked, branch)
	}

	// Return to original branch
	_ = state.Repo.CheckoutBranch(originalBranch)

	// Push updated metadata refs after restack
	if len(restacked) > 0 {
		pushMetadataRefs(state.Repo)
	}

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
	mcp.WithDescription(`Amend the last commit on the current branch (or create a new commit) and automatically restack direct children.

Parameters:
- message: commit message (required for new commits, optional for amends — omit to keep existing message)
- new_commit: if true, creates a new commit instead of amending (default: false)
- stage_all: if true, stages all working tree changes before committing (default: false)

Typical workflow: make code changes, then call gs_modify with stage_all=true and a message to amend them into the current branch.

NOTE: Only direct children are restacked. If the stack is deeper, call gs_restack with scope "upstack" afterward to propagate changes through grandchildren and beyond.

Returns: {branch, action ("amended" or "committed"), restacked_children[]}`),
	mcp.WithString("message",
		mcp.Description("Commit message (required for new commits, optional for amends — omit to keep existing message)"),
	),
	mcp.WithBoolean("new_commit",
		mcp.Description("Create a new commit instead of amending (default: false)"),
	),
	mcp.WithBoolean("stage_all",
		mcp.Description("Stage all working tree changes (git add -A) before committing (default: false)"),
	),
)

type modifyResponse struct {
	Branch            string   `json:"branch"`
	Action            string   `json:"action"`
	RestackedChildren []string `json:"restacked_children"`
}

func handleModify(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	message := req.GetString("message", "")
	newCommit := req.GetBool("new_commit", false)
	stageAll := req.GetBool("stage_all", false)

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
				_ = state.Metadata.SetParentRevision(child.Name, parentSHA)
			}
		}
		if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
			return errResult(fmt.Sprintf("failed to save metadata after modify: %v", err)), nil
		}
		_ = state.Repo.CheckoutBranch(currentBranch)
	}

	// Push updated metadata refs
	pushMetadataRefs(state.Repo)

	return jsonResult(modifyResponse{
		Branch:            currentBranch,
		Action:            action,
		RestackedChildren: restackedChildren,
	})
}

// --- gs_move ---

var moveTool = mcp.NewTool("gs_move",
	mcp.WithDescription(`Move a branch to a new parent, rebasing it onto the target branch. This changes where the branch sits in the stack tree.

The branch is rebased onto the new parent. Descendants of the moved branch are NOT automatically restacked — call gs_restack with the moved branch and scope "upstack" afterward to propagate the move through the subtree.

Cannot move a branch onto itself or onto one of its own descendants (would create a cycle). Cannot move trunk.

Returns: {branch, old_parent, new_parent}`),
	mcp.WithString("branch",
		mcp.Description("Branch to move (defaults to current branch)"),
	),
	mcp.WithString("onto",
		mcp.Required(),
		mcp.Description("New parent branch to rebase onto"),
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
	_ = state.Metadata.SetParentRevision(branchName, ontoSHA)
	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata after move: %v", err)), nil
	}
	pushMetadataRefs(state.Repo, branchName)

	return jsonResult(moveResponse{
		Branch:    branchName,
		OldParent: oldParent,
		NewParent: onto,
	})
}

// --- gs_fold ---

var foldTool = mcp.NewTool("gs_fold",
	mcp.WithDescription(`Squash-merge the current branch into its parent, combining all commits into a single commit on the parent. The branch is deleted afterward unless keep=true.

Children of the folded branch are reparented to the parent. After folding, call gs_restack with scope "upstack" on the parent to rebase reparented children onto the updated parent.

Use this when a branch's changes are complete and you want to collapse them into the parent. This is destructive — individual commit history on the folded branch is lost.

Cannot fold the trunk branch.

Returns: {folded, into, kept, reparented_children[]}`),
	mcp.WithBoolean("keep",
		mcp.Description("Keep the branch after folding instead of deleting it (default: false)"),
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
		_, _ = state.Repo.RunGitCommand("branch", "-D", currentBranch)
		state.Metadata.UntrackBranch(currentBranch)
	}

	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("failed to save metadata after fold: %v", err)), nil
	}

	// Sync remote refs
	if !keep {
		deleteRemoteMetadataRef(state.Repo, currentBranch)
	}
	pushMetadataRefs(state.Repo)

	return jsonResult(foldResponse{
		Folded:     currentBranch,
		Into:       parentBranch,
		Kept:       keep,
		Reparented: reparented,
	})
}

// --- gs_repair ---

var repairTool = mcp.NewTool("gs_repair",
	mcp.WithDescription(`Scan stack metadata for inconsistencies and optionally fix them.

Checks for: orphaned refs (branch deleted but ref remains), missing parents,
circular parent chains, ref/JSON mismatches, remote-deleted branches (local exists
but remote was deleted upstream).

Defaults to dry-run mode (report only). Pass fix=true to apply fixes.

Returns: {issues_found[], issues_fixed[], remaining[]}`),
	mcp.WithBoolean("fix",
		mcp.Description("If true, apply fixes. Default is false (dry-run)."),
	),
)

type repairIssueJSON struct {
	Kind        string `json:"kind"`
	Branch      string `json:"branch"`
	Description string `json:"description"`
	Fix         string `json:"fix"`
}

type repairResponse struct {
	IssuesFound []repairIssueJSON `json:"issues_found"`
	IssuesFixed []repairIssueJSON `json:"issues_fixed"`
	Remaining   []repairIssueJSON `json:"remaining"`
}

func issueToJSON(iss repair.Issue) repairIssueJSON {
	return repairIssueJSON{
		Kind:        string(iss.Kind),
		Branch:      iss.Branch,
		Description: iss.Description,
		Fix:         iss.Fix,
	}
}

func handleRepair(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fix := req.GetBool("fix", false)

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	issues, err := repair.DetectIssues(state.Repo, state.Metadata, state.Config)
	if err != nil {
		return errResult(fmt.Sprintf("failed to scan for issues: %v", err)), nil
	}

	allIssues := make([]repairIssueJSON, len(issues))
	for i, iss := range issues {
		allIssues[i] = issueToJSON(iss)
	}

	if !fix {
		return jsonResult(repairResponse{
			IssuesFound: allIssues,
			IssuesFixed: []repairIssueJSON{},
			Remaining:   allIssues,
		})
	}

	var fixed, remaining []repairIssueJSON
	for _, iss := range issues {
		if err := repair.ApplyFix(state.Repo, state.Metadata, state.Config, iss); err != nil {
			remaining = append(remaining, issueToJSON(iss))
		} else {
			fixed = append(fixed, issueToJSON(iss))
		}
	}

	if len(fixed) > 0 {
		if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
			return errResult(fmt.Sprintf("fixes applied but failed to save metadata: %v", err)), nil
		}
		pushMetadataRefs(state.Repo)
	}

	if fixed == nil {
		fixed = []repairIssueJSON{}
	}
	if remaining == nil {
		remaining = []repairIssueJSON{}
	}

	return jsonResult(repairResponse{
		IssuesFound: allIssues,
		IssuesFixed: fixed,
		Remaining:   remaining,
	})
}

// --- gs_submit ---

var submitTool = mcp.NewTool("gs_submit",
	mcp.WithDescription(`Create or update a pull request for a branch.

Detects the provider from the remote URL, pushes the branch, and creates/updates
a PR with the correct base branch from stack metadata. Stores the PR number in
metadata refs so teammates can see it.

Returns: {branch, parent, pr_number, pr_url, action, provider}
action is "created" or "updated".

Errors if: provider CLI (gh) is not installed, not authenticated, or provider unsupported.`),
	mcp.WithString("branch",
		mcp.Description("Branch to submit (default: current branch)"),
	),
	mcp.WithBoolean("draft",
		mcp.Description("Submit as draft PR (default: false)"),
	),
	mcp.WithString("title",
		mcp.Description("PR title (default: first commit message or branch name)"),
	),
)

type submitResponse struct {
	Branch   string `json:"branch"`
	Parent   string `json:"parent"`
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
	Action   string `json:"action"`
	Provider string `json:"provider"`
}

func handleSubmit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	branch := req.GetString("branch", state.Stack.Current)
	draft := req.GetBool("draft", false)
	title := req.GetString("title", "")

	node := state.Stack.GetNode(branch)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", branch)), nil
	}
	if node.IsTrunk {
		return errResult("cannot submit trunk branch"), nil
	}
	if node.Parent == nil {
		return errResult("branch has no parent"), nil
	}

	// Detect provider
	remoteURL, err := state.Repo.GetRemoteURL(defaultRemote)
	if err != nil {
		return errResult(fmt.Sprintf("no remote '%s' configured: %v", defaultRemote, err)), nil
	}
	prov, err := provider.DetectFromRemoteURL(remoteURL)
	if err != nil {
		return errResult(fmt.Sprintf("could not detect provider: %v", err)), nil
	}
	if !prov.CLIAvailable() {
		return errResult("provider CLI not found: install gh from https://cli.github.com/"), nil
	}
	if !prov.CLIAuthenticated() {
		return errResult("provider CLI not authenticated: run 'gh auth login'"), nil
	}

	parentBranch := node.Parent.Name

	// Ensure parent is pushed if it's not trunk
	if !node.Parent.IsTrunk && !state.Repo.HasRemoteBranch(parentBranch, defaultRemote) {
		if _, err := state.Repo.RunGitCommand("push", "-u", defaultRemote, parentBranch); err != nil {
			return errResult(fmt.Sprintf("failed to push parent '%s': %v", parentBranch, err)), nil
		}
	}

	result, err := submit.Branch(state.Repo, state.Metadata, prov, defaultRemote, submit.Opts{
		Branch: branch,
		Parent: parentBranch,
		Draft:  draft,
		Title:  title,
	})
	if err != nil {
		return errResult(fmt.Sprintf("failed to submit: %v", err)), nil
	}

	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("PR created but failed to save metadata: %v", err)), nil
	}
	pushMetadataRefs(state.Repo, branch)

	return jsonResult(submitResponse{
		Branch:   result.Branch,
		Parent:   result.Parent,
		PRNumber: result.PRNumber,
		PRURL:    result.PRURL,
		Action:   result.Action,
		Provider: result.Provider,
	})
}

// --- gs_land ---

var landTool = mcp.NewTool("gs_land",
	mcp.WithDescription(`Land a branch whose PR has been merged (or whose commits are in trunk).

Reparents children to the landed branch's parent, updates children's PR base
branches on the provider, deletes the branch locally and remotely, and cleans
up metadata refs.

Works without a provider — falls back to git merge-base check.
Errors if the branch is not merged.

Returns: {landed, new_parent, reparented_children[], updated_pr_bases[], checked_out?}`),
	mcp.WithString("branch",
		mcp.Description("Branch to land (default: current branch)"),
	),
)

type landResponse struct {
	Landed             string             `json:"landed"`
	NewParent          string             `json:"new_parent"`
	ReparentedChildren []string           `json:"reparented_children"`
	UpdatedPRBases     []landPRBaseUpdate `json:"updated_pr_bases"`
	Restacked          []string           `json:"restacked,omitempty"`
	CheckedOut         string             `json:"checked_out,omitempty"`
}

type landPRBaseUpdate struct {
	Branch   string `json:"branch"`
	PRNumber int    `json:"pr_number"`
	NewBase  string `json:"new_base"`
}

func handleLand(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	branch := req.GetString("branch", state.Stack.Current)

	if branch == state.Config.Trunk {
		return errResult("cannot land trunk branch"), nil
	}

	// Detect provider (best-effort)
	var prov provider.Provider
	remoteURL, err := state.Repo.GetRemoteURL(defaultRemote)
	if err == nil {
		if p, err := provider.DetectFromRemoteURL(remoteURL); err == nil {
			if p.CLIAvailable() && p.CLIAuthenticated() {
				prov = p
			}
		}
	}

	result, err := land.Branch(state.Repo, state.Metadata, prov, state.Config.Trunk, state.Stack.Current, defaultRemote, land.Opts{
		Branch: branch,
	})
	if err != nil {
		return errResult(fmt.Sprintf("failed to land: %v", err)), nil
	}

	// Save metadata and sync refs
	if err := state.Metadata.SaveWithRefs(state.Repo, state.Repo.GetMetadataPath()); err != nil {
		return errResult(fmt.Sprintf("branch landed but failed to save metadata: %v", err)), nil
	}
	deleteRemoteMetadataRef(state.Repo, branch)
	if len(result.ReparentedChildren) > 0 {
		pushMetadataRefs(state.Repo, result.ReparentedChildren...)
	}

	// Recursively restack children onto new parent (parity with CLI cmd/land.go restackChildren)
	var restacked []string
	if len(result.ReparentedChildren) > 0 {
		// Rebuild stack since the landed branch was removed
		s, buildErr := stack.BuildStack(state.Repo, state.Config, state.Metadata)
		if buildErr == nil {
			parentNode := s.GetNode(result.NewParent)
			if parentNode != nil {
				restacked = mcpRestackChildren(state.Repo, state.Metadata, s, parentNode)
			}
		}
		// Return to parent after restacking
		_ = state.Repo.CheckoutBranch(result.NewParent)
	}

	// Convert PR base updates
	prUpdates := make([]landPRBaseUpdate, len(result.UpdatedPRBases))
	for i, u := range result.UpdatedPRBases {
		prUpdates[i] = landPRBaseUpdate{
			Branch:   u.Branch,
			PRNumber: u.PRNumber,
			NewBase:  u.NewBase,
		}
	}

	return jsonResult(landResponse{
		Landed:             result.Landed,
		NewParent:          result.NewParent,
		ReparentedChildren: result.ReparentedChildren,
		UpdatedPRBases:     prUpdates,
		CheckedOut:         result.CheckedOut,
		Restacked:          restacked,
	})
}

// mcpRestackChildren recursively restacks all children of a node using
// precise --onto rebase when parentRevision is available, and updates
// parent revision metadata after each successful rebase. Mirrors the
// behavior of cmd/stack_restack.go restackChildren + restackBranchOnto.
func mcpRestackChildren(repo *git.Repo, metadata *config.Metadata, s *stack.Stack, parent *stack.Node) []string {
	var restacked []string
	for _, child := range parent.Children {
		if err := repo.CheckoutBranch(child.Name); err != nil {
			continue
		}

		behind, err := repo.IsBehind(child.Name, parent.Name)
		if err != nil || !behind {
			// Recurse even if this child doesn't need rebase — grandchildren might
			if len(child.Children) > 0 {
				restacked = append(restacked, mcpRestackChildren(repo, metadata, s, child)...)
			}
			continue
		}

		// Use precise --onto rebase when parentRevision is available
		parentRev := metadata.GetParentRevision(child.Name)
		var rebaseErr error
		if parentRev != "" {
			rebaseErr = repo.RebaseOnto(child.Name, parent.Name, parentRev)
		} else {
			rebaseErr = repo.Rebase(child.Name, parent.Name)
		}

		if rebaseErr != nil {
			continue // skip this child on conflict, try siblings
		}

		// Update parent revision after successful rebase
		parentSHA, _ := repo.GetBranchCommit(parent.Name)
		if parentSHA != "" {
			_ = metadata.SetParentRevision(child.Name, parentSHA)
		}

		restacked = append(restacked, child.Name)

		// Recurse into grandchildren
		if len(child.Children) > 0 {
			restacked = append(restacked, mcpRestackChildren(repo, metadata, s, child)...)
		}
	}
	return restacked
}
