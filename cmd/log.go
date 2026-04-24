package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/cmdutil"
	"github.com/israelmalagutti/git-stack/internal/provider"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	logShort bool
	logLong  bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display a visual representation of the current stack",
	Long: `Display a visual representation of the stack structure.

Shows branches as a tree starting from the trunk branch, with
the current branch highlighted.

Modes:
  gs log         - Standard tree view (*branch = current)
  gs log --short - Compact indented view (● = current, ○ = other)
  gs log --long  - Detailed view with commit messages
  gs log --json  - Machine-readable JSON output`,
	RunE: runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVar(&logShort, "short", false, "Show compact view")
	logCmd.Flags().BoolVar(&logLong, "long", false, "Show detailed view with commit messages")
}

// logResult is the JSON-serializable output of gs log.
type logResult struct {
	Trunk         string          `json:"trunk"`
	CurrentBranch string          `json:"current_branch"`
	Branches      []logBranchInfo `json:"branches"`
}

// logBranchInfo describes a single branch in the stack.
type logBranchInfo struct {
	Name      string   `json:"name"`
	Parent    string   `json:"parent"`
	Children  []string `json:"children"`
	CommitSHA string   `json:"commit_sha"`
	Depth     int      `json:"depth"`
	IsCurrent bool     `json:"is_current"`
	IsTrunk   bool     `json:"is_trunk"`
	PRURL     string   `json:"pr_url,omitempty"`
}

// buildLogResult constructs the JSON result from stack data.
func buildLogResult(rs *repoState) logResult {
	s := rs.Stack

	// Start with trunk
	branches := make([]logBranchInfo, 0, len(s.Nodes))
	if s.Trunk != nil {
		branches = append(branches, nodeToLogBranch(s, s.Trunk))
	}

	// Then topological order (non-trunk)
	for _, node := range s.GetTopologicalOrder() {
		branches = append(branches, nodeToLogBranch(s, node))
	}

	return logResult{
		Trunk:         s.TrunkName,
		CurrentBranch: s.Current,
		Branches:      branches,
	}
}

// nodeToLogBranch converts a stack node to a JSON-friendly struct.
func nodeToLogBranch(s *stack.Stack, node *stack.Node) logBranchInfo {
	children := make([]string, 0, len(node.Children))
	for _, c := range node.SortedChildren() {
		children = append(children, c.Name)
	}

	parentName := ""
	if node.Parent != nil {
		parentName = node.Parent.Name
	}

	sha := node.CommitSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}

	return logBranchInfo{
		Name:      node.Name,
		Parent:    parentName,
		Children:  children,
		CommitSHA: sha,
		Depth:     s.GetStackDepth(node.Name),
		IsCurrent: node.IsCurrent,
		IsTrunk:   node.IsTrunk,
		PRURL:     node.PRURL,
	}
}

func runLog(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	if err := rs.Stack.ValidateStack(); err != nil {
		return fmt.Errorf("invalid stack structure: %w", err)
	}

	// Populate PR URLs from remote (best-effort, non-fatal)
	if remoteURL, err := rs.Repo.GetRemoteURL("origin"); err == nil {
		if host, owner, repoName, err := provider.ParseRemoteURL(remoteURL); err == nil {
			rs.Stack.SetPRURLs(fmt.Sprintf("https://%s/%s/%s", host, owner, repoName))
		}
	}

	// JSON mode: emit structured data and return
	if cmd != nil && cmdutil.JSONMode(cmd) {
		return cmdutil.PrintJSON(buildLogResult(rs))
	}

	var output string
	if logShort {
		output = rs.Stack.RenderShort(rs.Repo)
	} else {
		opts := stack.TreeOptions{
			ShowCommitSHA: true,
			ShowCommitMsg: logLong,
			Detailed:      logLong,
		}
		output = rs.Stack.RenderTree(rs.Repo, opts)
	}

	fmt.Print(output)

	return nil
}
