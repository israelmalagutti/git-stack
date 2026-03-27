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
	s.AddTool(logTool, handleLog)
	s.AddTool(diffTool, handleDiff)
}

// --- gs_status ---

var statusTool = mcp.NewTool("gs_status",
	mcp.WithDescription(`Start here. Returns the full stack state as structured JSON: all tracked branches, their parent-child relationships, current branch, and trunk name.

Use this as your first call to orient yourself in a repository. It gives you the complete picture of the stack tree in one call.

Prefer gs_status over gs_log unless you specifically need commit history on every branch. For detailed info on a single branch (including commits and needs-restack status), follow up with gs_branch_info.

Returns: {trunk, current_branch, initialized, summary: {total_branches, needs_restack[]}, branches: [{name, parent, children[], commit_sha, depth, is_current, is_trunk}]}`),
	mcp.WithReadOnlyHintAnnotation(true),
)

type statusSummary struct {
	TotalBranches int      `json:"total_branches"`
	NeedsRestack  []string `json:"needs_restack"`
}

type statusResponse struct {
	Trunk         string        `json:"trunk"`
	CurrentBranch string        `json:"current_branch"`
	Initialized   bool          `json:"initialized"`
	Summary       statusSummary `json:"summary"`
	Branches      []branchJSON  `json:"branches"`
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

	// Compute needs-restack summary
	needsRestack := []string{}
	for _, node := range state.Stack.GetTopologicalOrder() {
		if node.Parent != nil {
			behind, err := state.Repo.IsBehind(node.Name, node.Parent.Name)
			if err == nil && behind {
				needsRestack = append(needsRestack, node.Name)
			}
		}
	}

	resp := statusResponse{
		Trunk:         state.Stack.TrunkName,
		CurrentBranch: state.Stack.Current,
		Initialized:   true,
		Summary: statusSummary{
			TotalBranches: len(branches),
			NeedsRestack:  needsRestack,
		},
		Branches: branches,
	}

	return jsonResult(resp)
}

// --- gs_branch_info ---

var branchInfoTool = mcp.NewTool("gs_branch_info",
	mcp.WithDescription(`Get detailed information about a single branch: its commits, parent, children, stack depth, and whether it needs restacking.

Use this after gs_status when you need to inspect a specific branch more closely — for example, to see its commits before deciding whether to fold or modify it, or to check if it needs restacking.

If needs_restack is true, the branch has diverged from its parent and you should call gs_restack with scope "only" on that branch.

Returns: {name, parent, children[], commit_sha, commits: [{sha, message}], depth, is_current, is_trunk, needs_restack}`),
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
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs — call gs_status to see all tracked branches, or use gs_track to start tracking it", branchName)), nil
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

// --- gs_log ---

var logTool = mcp.NewTool("gs_log",
	mcp.WithDescription(`Get the stack tree with optional commit history for every branch. This is a superset of gs_status — use it when you need to see what commits each branch contains.

Call with include_commits=true to get the full commit list per branch. Without it, the response is nearly identical to gs_status.

Prefer gs_status for a quick overview. Use gs_log with include_commits=true when you need to understand the content of the entire stack (e.g., before a large restack or to find which branch contains a specific change).

Returns: {trunk, current_branch, branches: [{name, parent, children[], commit_sha, depth, is_current, is_trunk, commits?: [{sha, message}]}]}`),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithBoolean("include_commits",
		mcp.Description("Whether to include commit history for each branch (default: false)"),
	),
)

type logBranchJSON struct {
	Name      string       `json:"name"`
	Parent    string       `json:"parent,omitempty"`
	Children  []string     `json:"children"`
	CommitSHA string       `json:"commit_sha"`
	Depth     int          `json:"depth"`
	IsCurrent bool         `json:"is_current"`
	IsTrunk   bool         `json:"is_trunk"`
	Commits   []commitJSON `json:"commits,omitempty"`
}

type logResponse struct {
	Trunk         string          `json:"trunk"`
	CurrentBranch string          `json:"current_branch"`
	Branches      []logBranchJSON `json:"branches"`
}

func handleLog(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeCommits := req.GetBool("include_commits", false)

	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	branches := make([]logBranchJSON, 0, len(state.Stack.Nodes))

	// Helper to convert a node
	nodeToLogBranch := func(node *stack.Node) logBranchJSON {
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

		b := logBranchJSON{
			Name:      node.Name,
			Parent:    parentName,
			Children:  children,
			CommitSHA: sha,
			Depth:     state.Stack.GetStackDepth(node.Name),
			IsCurrent: node.IsCurrent,
			IsTrunk:   node.IsTrunk,
		}

		if includeCommits {
			commits := state.Stack.GetBranchCommits(state.Repo, node)
			b.Commits = make([]commitJSON, 0, len(commits))
			for _, c := range commits {
				b.Commits = append(b.Commits, commitToJSON(c))
			}
		}

		return b
	}

	// Trunk first
	if state.Stack.Trunk != nil {
		branches = append(branches, nodeToLogBranch(state.Stack.Trunk))
	}

	// Non-trunk in topological order
	for _, node := range state.Stack.GetTopologicalOrder() {
		branches = append(branches, nodeToLogBranch(node))
	}

	resp := logResponse{
		Trunk:         state.Stack.TrunkName,
		CurrentBranch: state.Stack.Current,
		Branches:      branches,
	}

	return jsonResult(resp)
}

// --- gs_diff ---

var diffTool = mcp.NewTool("gs_diff",
	mcp.WithDescription(`Get the unified diff for a branch compared to its parent branch. Shows only the changes introduced by this branch, not inherited changes.

Use this to review what a branch actually changes before deciding to modify, fold, or submit it. Defaults to the current branch if no branch name is provided.

Cannot diff the trunk branch (it has no parent). For large branches with many commits, the diff may be very long — consider using gs_branch_info first to check the commit count.

After reviewing, common next steps: gs_modify to amend, gs_fold to squash into parent, or gs_create to add a follow-up branch.

Returns: {branch, parent, diff} where diff is a standard unified diff string.`),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithString("branch",
		mcp.Description("Branch to diff (defaults to current branch if omitted)"),
	),
)

type diffResponse struct {
	Branch string `json:"branch"`
	Parent string `json:"parent"`
	Diff   string `json:"diff"`
}

func handleDiff(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := loadRepoState()
	if err != nil {
		return errResult(err.Error()), nil
	}

	branchName := req.GetString("branch", "")
	if branchName == "" {
		branchName = state.Stack.Current
	}

	node := state.Stack.GetNode(branchName)
	if node == nil {
		return errResult(fmt.Sprintf("branch '%s' is not tracked by gs — call gs_status to see all tracked branches, or use gs_track to start tracking it", branchName)), nil
	}

	if node.Parent == nil {
		return errResult(fmt.Sprintf("branch '%s' is trunk and has no parent to diff against — use gs_diff on a non-trunk branch instead", branchName)), nil
	}

	diff, err := state.Repo.RunGitCommand("diff", node.Parent.Name+"..."+node.Name)
	if err != nil {
		return errResult(fmt.Sprintf("failed to get diff: %v", err)), nil
	}

	resp := diffResponse{
		Branch: node.Name,
		Parent: node.Parent.Name,
		Diff:   diff,
	}

	return jsonResult(resp)
}
