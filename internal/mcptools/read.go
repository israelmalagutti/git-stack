package mcptools

import (
	"context"
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerReadTools(s *server.MCPServer) {
	s.AddTool(statusTool, handleStatus)
	s.AddTool(branchInfoTool, handleBranchInfo)
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

// --- gs_branch_info ---

var branchInfoTool = mcp.NewTool("gs_branch_info",
	mcp.WithDescription("Get detailed information about a specific branch: metadata, commits unique to this branch, parent, children, depth, and whether it needs restacking."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithString("branch",
		mcp.Required(),
		mcp.Description("Name of the branch to get info for"),
	),
)

type commitJSON struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

type branchInfoResponse struct {
	Name         string       `json:"name"`
	Parent       string       `json:"parent,omitempty"`
	Children     []string     `json:"children"`
	CommitSHA    string       `json:"commit_sha"`
	Commits      []commitJSON `json:"commits"`
	Depth        int          `json:"depth"`
	IsCurrent    bool         `json:"is_current"`
	IsTrunk      bool         `json:"is_trunk"`
	NeedsRestack bool         `json:"needs_restack"`
}

func handleBranchInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	branchName, err := req.RequireString("branch")
	if err != nil {
		return errResult("missing required parameter: branch"), nil
	}

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	node := state.Stack.GetNode(branchName)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs", branchName)), nil
	}

	// Get commits unique to this branch
	commits := state.Stack.GetBranchCommits(state.Repo, node)
	commitList := make([]commitJSON, 0, len(commits))
	for _, c := range commits {
		commitList = append(commitList, commitJSON{SHA: c.SHA, Message: c.Message})
	}

	// Check if branch needs restacking
	needsRestack := false
	if node.Parent != nil {
		behind, err := state.Repo.IsBehind(node.Name, node.Parent.Name)
		if err == nil {
			needsRestack = behind
		}
	}

	// Build children list
	children := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child.Name)
	}

	parentName := ""
	if node.Parent != nil {
		parentName = node.Parent.Name
	}

	sha := node.CommitSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}

	resp := branchInfoResponse{
		Name:         node.Name,
		Parent:       parentName,
		Children:     children,
		CommitSHA:    sha,
		Commits:      commitList,
		Depth:        state.Stack.GetStackDepth(node.Name),
		IsCurrent:    node.IsCurrent,
		IsTrunk:      node.IsTrunk,
		NeedsRestack: needsRestack,
	}

	return jsonResult(resp)
}

// commitToJSON converts a stack.Commit to commitJSON.
func commitToJSON(c stack.Commit) commitJSON {
	return commitJSON{SHA: c.SHA, Message: c.Message}
}
