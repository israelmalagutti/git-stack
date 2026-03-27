package mcptools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerReadTools(s *server.MCPServer) {
	s.AddTool(statusTool, handleStatus)
}

// --- gs_status ---

var statusTool = mcp.NewTool("gs_status",
	mcp.WithDescription("Get the full stack state: all branches, their parents, children, current branch, and trunk. Returns structured JSON representing the entire stack tree."),
	mcp.WithReadOnlyHintAnnotation(true),
)

type statusResponse struct {
	Trunk         string       `json:"trunk"`
	CurrentBranch string       `json:"current_branch"`
	Initialized   bool         `json:"initialized"`
	Branches      []branchJSON `json:"branches"`
}

func handleStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	// Build ordered branch list (topological: parents before children, then trunk last)
	branches := make([]branchJSON, 0, len(state.Stack.Nodes))

	// Add trunk first
	if state.Stack.Trunk != nil {
		branches = append(branches, nodeToBranchJSON(state.Stack, state.Stack.Trunk))
	}

	// Add non-trunk branches in topological order
	for _, node := range state.Stack.GetTopologicalOrder() {
		branches = append(branches, nodeToBranchJSON(state.Stack, node))
	}

	resp := statusResponse{
		Trunk:         state.Stack.TrunkName,
		CurrentBranch: state.Stack.Current,
		Initialized:   true,
		Branches:      branches,
	}

	return jsonResult(resp)
}
