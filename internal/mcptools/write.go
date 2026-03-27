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
