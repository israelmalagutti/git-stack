package mcptools

import (
	"encoding/json"
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/mark3labs/mcp-go/mcp"
)

// repoState holds all the state needed by most MCP tool handlers.
// Each tool call creates a fresh instance — no caching between calls.
type repoState struct {
	Repo     *git.Repo
	Config   *config.Config
	Metadata *config.Metadata
	Stack    *stack.Stack
}

// loadRepoState initializes repo, config, metadata, and builds the stack tree.
func loadRepoState() (*repoState, error) {
	repo, err := git.NewRepo()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return nil, fmt.Errorf("gs not initialized (run 'gs init'): %w", err)
	}

	metadata, source, err := config.LoadMetadataWithRefs(repo, repo.GetMetadataPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}
	// Auto-migrate JSON-only metadata to refs
	if source == config.SourceJSON {
		_ = metadata.SaveWithRefs(repo, repo.GetMetadataPath())
	}

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to build stack: %w", err)
	}

	return &repoState{
		Repo:     repo,
		Config:   cfg,
		Metadata: metadata,
		Stack:    s,
	}, nil
}

const defaultRemote = "origin"

// pushMetadataRefs pushes the specified branches' metadata refs to the remote.
// Best-effort: silently skips if no remote exists.
func pushMetadataRefs(repo *git.Repo, branches ...string) {
	if !repo.HasRemote(defaultRemote) {
		return
	}
	if len(branches) == 0 {
		_ = config.PushAllRefs(repo, defaultRemote)
	} else {
		for _, branch := range branches {
			_ = config.PushBranchMeta(repo, defaultRemote, branch)
		}
	}
}

// deleteRemoteMetadataRef deletes a branch's metadata ref from the remote. Best-effort.
func deleteRemoteMetadataRef(repo *git.Repo, branch string) {
	if !repo.HasRemote(defaultRemote) {
		return
	}
	_ = config.DeleteRemoteBranchMeta(repo, defaultRemote, branch)
}

// jsonResult marshals data to JSON and returns an MCP text result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// errResult returns a structured MCP error result.
func errResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}

// branchJSON is the standard JSON representation of a branch for MCP responses.
type branchJSON struct {
	Name      string   `json:"name"`
	Parent    string   `json:"parent,omitempty"`
	Children  []string `json:"children"`
	CommitSHA string   `json:"commit_sha"`
	Depth     int      `json:"depth"`
	IsCurrent bool     `json:"is_current"`
	IsTrunk   bool     `json:"is_trunk"`
}

// nodeToBranchJSON converts a stack node to its JSON representation.
func nodeToBranchJSON(s *stack.Stack, node *stack.Node) branchJSON {
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

	return branchJSON{
		Name:      node.Name,
		Parent:    parentName,
		Children:  children,
		CommitSHA: sha,
		Depth:     s.GetStackDepth(node.Name),
		IsCurrent: node.IsCurrent,
		IsTrunk:   node.IsTrunk,
	}
}
