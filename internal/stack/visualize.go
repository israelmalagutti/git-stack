package stack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// TreeOptions controls how the tree is rendered
type TreeOptions struct {
	ShowCommitSHA bool
	ShowCommitMsg bool
	Detailed      bool
}

// Commit represents a single commit in a branch
type Commit struct {
	SHA     string
	Message string
}

// formatPRLink returns a styled, clickable PR annotation for a node, or "".
func formatPRLink(node *Node) string {
	if node.PRNumber == 0 {
		return ""
	}
	label := fmt.Sprintf("#%d", node.PRNumber)
	if node.PRURL != "" {
		label = colors.Hyperlink(node.PRURL, label)
	}
	return " " + colors.Muted(label)
}

// columnLayout tracks which column each node is assigned to and caches
// sorted children to avoid repeated sorting and git subprocess calls.
type columnLayout struct {
	columns        map[string]int      // node name → column index
	maxColumn      int
	sortedCache    map[string][]*Node  // node name → sorted children (cached)
	timestampCache map[string]int64    // branch name → commit timestamp (cached)
}

// assignColumns recursively assigns columns to all nodes.
// The oldest child inherits the parent's column; subsequent children get new columns.
func (cl *columnLayout) assignColumns(node *Node, col int, repo *git.Repo) {
	cl.columns[node.Name] = col
	if col > cl.maxColumn {
		cl.maxColumn = col
	}

	children := cl.getSortedChildren(repo, node)
	for i, child := range children {
		if i == 0 {
			// Primary (oldest) child inherits parent's column
			cl.assignColumns(child, col, repo)
		} else {
			// Secondary children get new columns
			cl.maxColumn++
			cl.assignColumns(child, cl.maxColumn, repo)
		}
	}
}

// getSortedChildren returns children sorted oldest-first, caching the result
// so that repeated calls during layout and rendering don't re-sort or re-fetch
// git timestamps.
func (cl *columnLayout) getSortedChildren(repo *git.Repo, node *Node) []*Node {
	if len(node.Children) == 0 {
		return nil
	}
	if cached, ok := cl.sortedCache[node.Name]; ok {
		return cached
	}

	var sorted []*Node
	if repo != nil {
		sorted = cl.sortChildrenOldestFirst(repo, node.Children)
	} else {
		sorted = node.SortedChildren()
	}
	cl.sortedCache[node.Name] = sorted
	return sorted
}

// sortChildrenOldestFirst sorts children by commit time ascending (oldest first).
// Uses the layout's timestamp cache to avoid redundant git subprocess calls.
func (cl *columnLayout) sortChildrenOldestFirst(repo *git.Repo, children []*Node) []*Node {
	if len(children) == 0 {
		return nil
	}
	sorted := make([]*Node, len(children))
	copy(sorted, children)

	timestamps := make(map[string]int64, len(sorted))
	for _, child := range sorted {
		if ts, ok := cl.timestampCache[child.Name]; ok {
			timestamps[child.Name] = ts
		} else {
			ts := getCommitTimestamp(repo, child.Name)
			cl.timestampCache[child.Name] = ts
			timestamps[child.Name] = ts
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return timestamps[sorted[i].Name] < timestamps[sorted[j].Name]
	})

	return sorted
}

// flattenedNode represents a node in render order along with metadata
type flattenedNode struct {
	node     *Node
	subMerge bool // true if a sub-merge line should be rendered before this node
}

// flattenBranch produces the render order for a node's primary chain and secondary subtrees.
// Primary (oldest) child above, then secondary children above (with sub-merge), then this node.
func flattenBranch(node *Node, repo *git.Repo, layout *columnLayout) []flattenedNode {
	children := layout.getSortedChildren(repo, node)
	var result []flattenedNode

	// Primary child's subtree above
	if len(children) > 0 {
		result = append(result, flattenBranch(children[0], repo, layout)...)
	}

	// Secondary children's subtrees above (will be closed by sub-merge)
	if len(children) > 1 {
		for _, child := range children[1:] {
			result = append(result, flattenBranch(child, repo, layout)...)
		}
	}

	// This node (with sub-merge flag if it has secondary children)
	result = append(result, flattenedNode{node: node, subMerge: len(children) > 1})

	return result
}

// flattenTree produces the full render order: all branches above trunk, then trunk.
func flattenTree(s *Stack, repo *git.Repo, layout *columnLayout) []flattenedNode {
	children := layout.getSortedChildren(repo, s.Trunk)
	var result []flattenedNode

	for _, child := range children {
		result = append(result, flattenBranch(child, repo, layout)...)
	}

	return result
}

// buildColumnPrefix builds the prefix string with │ at each active column,
// up to but not including the target column.
func buildColumnPrefix(activeCols map[int]bool, upTo int) string {
	chars := colors.DefaultTreeChars()
	var sb strings.Builder
	for c := 0; c < upTo; c++ {
		if activeCols[c] {
			sb.WriteString(colors.Muted(chars.Vertical))
			sb.WriteString("   ")
		} else {
			sb.WriteString("    ")
		}
	}
	return sb.String()
}

// RenderTree renders the stack as a column-based tree with commits.
// Older branches at top, newer branches closer to trunk at bottom.
func (s *Stack) RenderTree(repo *git.Repo, opts TreeOptions) string {
	layout := &columnLayout{
		columns:        make(map[string]int),
		sortedCache:    make(map[string][]*Node),
		timestampCache: make(map[string]int64),
	}
	layout.assignColumns(s.Trunk, 0, repo)

	flat := flattenTree(s, repo, layout)

	var result strings.Builder
	chars := colors.DefaultTreeChars()

	// Track which columns are currently active (have a vertical line passing through)
	activeCols := make(map[int]bool)

	for i, fn := range flat {
		node := fn.node
		col := layout.columns[node.Name]
		depth := s.GetStackDepth(node.Name)

		// Activate this column when we first render in it
		activeCols[col] = true

		// Sub-merge line before this node (closes secondary children's columns)
		if fn.subMerge {
			maxSecCol := findMaxActiveSecondaryCol(node, layout)
			if maxSecCol > col {
				result.WriteString(buildMergeLine(activeCols, col, maxSecCol))
				result.WriteString("\n")
				for c := col + 1; c <= maxSecCol; c++ {
					activeCols[c] = false
				}
			}
		}

		// Node line
		prefix := buildColumnPrefix(activeCols, col)
		indicator := chars.Circle
		if node.IsCurrent {
			indicator = chars.FilledCircle
		}
		var coloredIndicator, branchName string
		if node.IsCurrent {
			coloredIndicator = colors.CycleText(indicator, depth)
			branchName = colors.BranchCurrent(node.Name)
		} else {
			coloredIndicator = colors.Muted(indicator)
			branchName = colors.Muted(node.Name)
		}

		result.WriteString(prefix)
		result.WriteString(coloredIndicator)
		result.WriteString(" ")
		result.WriteString(branchName)
		if node.IsCurrent {
			result.WriteString(colors.Muted(" (current)"))
		}
		result.WriteString(formatPRLink(node))

		// Fetch commits once for both time-ago and commit lines
		var commits []Commit
		if repo != nil {
			commits = s.getBranchCommits(repo, node)
			if len(commits) > 0 {
				timeAgo := getTimeSinceLastCommit(repo, node.Name)
				if timeAgo != "" {
					result.WriteString(colors.Muted(" · " + timeAgo))
				}
			}
		}
		result.WriteString("\n")

		// Commit lines
		if repo != nil {
			for ci, commit := range commits {
				cPrefix := buildColumnPrefix(activeCols, col)
				result.WriteString(cPrefix)
				result.WriteString(colors.Muted(chars.Vertical))
				result.WriteString(" ")

				sha := commit.SHA
				if len(sha) > 7 {
					sha = sha[:7]
				}
				if ci == 0 {
					result.WriteString(colors.CommitSHA(sha))
				} else {
					result.WriteString(colors.Muted(sha))
				}
				result.WriteString(colors.Muted(" - "))

				msg := commit.Message
				if len(msg) > 50 {
					msg = msg[:47] + "..."
				}
				result.WriteString(colors.Muted(msg))
				result.WriteString("\n")
			}
		}

		// Connector lines after the node
		isLastNode := (i == len(flat)-1)
		nextHasSubMerge := (i+1 < len(flat) && flat[i+1].subMerge)

		hasActiveSecondary := false
		for c := 1; c <= layout.maxColumn; c++ {
			if activeCols[c] {
				hasActiveSecondary = true
				break
			}
		}
		isLastBeforeMainMerge := isLastNode && hasActiveSecondary

		if nextHasSubMerge {
			// Last node before a sub-merge: 0 connector lines
		} else if isLastBeforeMainMerge {
			// Last node before main merge: 1 connector line
			connPrefix := buildColumnPrefix(activeCols, col)
			result.WriteString(connPrefix)
			result.WriteString(colors.Muted(chars.Vertical))
			result.WriteString("\n")
		} else {
			// Normal: 2 connector lines
			for n := 0; n < 2; n++ {
				connPrefix := buildColumnPrefix(activeCols, col)
				result.WriteString(connPrefix)
				result.WriteString(colors.Muted(chars.Vertical))
				result.WriteString("\n")
			}
		}
	}

	// Main merge line before trunk (close all still-active non-trunk columns)
	maxActiveCol := 0
	for c := 1; c <= layout.maxColumn; c++ {
		if activeCols[c] {
			maxActiveCol = c
		}
	}
	if maxActiveCol > 0 {
		result.WriteString(buildMergeLine(activeCols, 0, maxActiveCol))
		result.WriteString("\n")
		// Post-merge vertical connector to trunk
		result.WriteString(colors.Muted(chars.Vertical))
		result.WriteString("\n")
	}

	// Render trunk
	s.renderTrunkWithCommits(&result, s.Trunk, repo, opts)

	return result.String()
}

// renderTrunkWithCommits renders the trunk branch
func (s *Stack) renderTrunkWithCommits(result *strings.Builder, node *Node, repo *git.Repo, opts TreeOptions) {
	chars := colors.DefaultTreeChars()
	depth := s.GetStackDepth(node.Name)

	indicator := chars.Circle
	if node.IsCurrent {
		indicator = chars.FilledCircle
	}

	var coloredIndicator, branchName string
	if node.IsCurrent {
		coloredIndicator = colors.CycleText(indicator, depth)
		branchName = colors.BranchCurrent(node.Name)
	} else {
		coloredIndicator = colors.Muted(indicator)
		branchName = colors.Muted(node.Name)
	}

	result.WriteString(coloredIndicator)
	result.WriteString(" ")
	result.WriteString(branchName)

	if node.IsCurrent {
		result.WriteString(colors.Muted(" (current)"))
	}

	var commits []Commit
	if repo != nil {
		commits = getTrunkCommits(repo, node.Name, 3)
		if len(commits) > 0 {
			timeAgo := getTimeSinceLastCommit(repo, node.Name)
			if timeAgo != "" {
				result.WriteString(colors.Muted(" · " + timeAgo))
			}
		}
	}

	result.WriteString("\n")

	// Render trunk commits
	if repo != nil {
		verticalLine := colors.Muted(chars.Vertical)
		for i, commit := range commits {
			result.WriteString(verticalLine)
			result.WriteString(" ")

			sha := commit.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			if i == 0 {
				result.WriteString(colors.CommitSHA(sha))
			} else {
				result.WriteString(colors.Muted(sha))
			}
			result.WriteString(colors.Muted(" - "))

			msg := commit.Message
			if len(msg) > 50 {
				msg = msg[:47] + "..."
			}
			result.WriteString(colors.Muted(msg))
			result.WriteString("\n")
		}

		// Trailing connector for trunk
		result.WriteString(verticalLine)
		result.WriteString("\n")
	}
}

// buildMergeLine builds a merge line closing columns from startCol+1 to maxCol.
// Uses ├ at startCol (tee, since the line continues below) and ┘ at the end.
func buildMergeLine(activeCols map[int]bool, startCol, maxCol int) string {
	chars := colors.DefaultTreeChars()
	var sb strings.Builder

	// Prefix: verticals for columns before startCol
	for c := 0; c < startCol; c++ {
		if activeCols[c] {
			sb.WriteString(colors.Muted(chars.Vertical))
			sb.WriteString("   ")
		} else {
			sb.WriteString("    ")
		}
	}

	// The tee at startCol (line continues below)
	sb.WriteString(colors.Muted(chars.Tee))

	// Horizontals and junctions for each column being closed
	for c := startCol + 1; c <= maxCol; c++ {
		sb.WriteString(colors.Muted(chars.Horizontal + chars.Horizontal + chars.Horizontal))
		if c == maxCol {
			sb.WriteString(colors.Muted(chars.BottomRight))
		} else if activeCols[c] {
			sb.WriteString(colors.Muted("┴"))
		} else {
			sb.WriteString(colors.Muted(chars.Horizontal))
		}
	}

	return sb.String()
}

// findMaxActiveSecondaryCol finds the maximum column used by secondary children
// of a given node.
func findMaxActiveSecondaryCol(node *Node, layout *columnLayout) int {
	nodeCol := layout.columns[node.Name]
	maxCol := nodeCol
	var walkChildren func(n *Node)
	walkChildren = func(n *Node) {
		c := layout.columns[n.Name]
		if c > maxCol {
			maxCol = c
		}
		for _, child := range n.Children {
			walkChildren(child)
		}
	}
	for _, child := range node.Children {
		walkChildren(child)
	}
	// Only return > nodeCol if there are actual secondary columns
	if maxCol > nodeCol {
		return maxCol
	}
	return -1
}

// RenderShort renders a compact column-based view of the stack (no commits).
func (s *Stack) RenderShort(repo *git.Repo) string {
	layout := &columnLayout{
		columns:        make(map[string]int),
		sortedCache:    make(map[string][]*Node),
		timestampCache: make(map[string]int64),
	}
	layout.assignColumns(s.Trunk, 0, repo)

	flat := flattenTree(s, repo, layout)

	var result strings.Builder
	chars := colors.DefaultTreeChars()

	activeCols := make(map[int]bool)

	for i, fn := range flat {
		node := fn.node
		col := layout.columns[node.Name]
		depth := s.GetStackDepth(node.Name)

		// Activate this column
		activeCols[col] = true

		// Sub-merge line before this node (closes secondary children's columns)
		if fn.subMerge {
			maxSecCol := findMaxActiveSecondaryCol(node, layout)
			if maxSecCol > col {
				result.WriteString(buildMergeLine(activeCols, col, maxSecCol))
				result.WriteString("\n")
				// Deactivate the closed columns
				for c := col + 1; c <= maxSecCol; c++ {
					activeCols[c] = false
				}
			}
		}

		// Node line
		prefix := buildColumnPrefix(activeCols, col)
		indicator := chars.Circle
		if node.IsCurrent {
			indicator = chars.FilledCircle
		}
		var coloredIndicator, branchName string
		if node.IsCurrent {
			coloredIndicator = colors.CycleText(indicator, depth)
			branchName = colors.BranchCurrent(node.Name)
		} else {
			coloredIndicator = colors.Muted(indicator)
			branchName = colors.Muted(node.Name)
		}

		result.WriteString(prefix)
		result.WriteString(coloredIndicator)
		result.WriteString(" ")
		result.WriteString(branchName)
		if node.IsCurrent {
			result.WriteString(colors.Muted(" (current)"))
		}
		result.WriteString(formatPRLink(node))
		result.WriteString("\n")

		// Connector lines after the node
		isLastNode := (i == len(flat)-1)
		nextHasSubMerge := (i+1 < len(flat) && flat[i+1].subMerge)

		// Check if main merge will actually render (any non-trunk column still active)
		hasActiveSecondary := false
		for c := 1; c <= layout.maxColumn; c++ {
			if activeCols[c] {
				hasActiveSecondary = true
				break
			}
		}
		isLastBeforeMainMerge := isLastNode && hasActiveSecondary

		if nextHasSubMerge {
			// Last node before a sub-merge: 0 connector lines
		} else if isLastBeforeMainMerge {
			// Last node before main merge: 1 connector line
			connPrefix := buildColumnPrefix(activeCols, col)
			result.WriteString(connPrefix)
			result.WriteString(colors.Muted(chars.Vertical))
			result.WriteString("\n")
		} else {
			// Normal: 2 connector lines
			for n := 0; n < 2; n++ {
				connPrefix := buildColumnPrefix(activeCols, col)
				result.WriteString(connPrefix)
				result.WriteString(colors.Muted(chars.Vertical))
				result.WriteString("\n")
			}
		}
	}

	// Main merge line before trunk (close all still-active non-trunk columns)
	maxActiveCol := 0
	for c := 1; c <= layout.maxColumn; c++ {
		if activeCols[c] {
			maxActiveCol = c
		}
	}
	if maxActiveCol > 0 {
		result.WriteString(buildMergeLine(activeCols, 0, maxActiveCol))
		result.WriteString("\n")
		// Post-merge vertical connector to trunk
		result.WriteString(colors.Muted(chars.Vertical))
		result.WriteString("\n")
	}

	// Render trunk
	trunkIndicator := chars.Circle
	if s.Trunk.IsCurrent {
		trunkIndicator = chars.FilledCircle
	}
	var coloredIndicator, trunkName string
	if s.Trunk.IsCurrent {
		coloredIndicator = colors.CycleText(trunkIndicator, 0)
		trunkName = colors.BranchCurrent(s.Trunk.Name)
	} else {
		coloredIndicator = colors.Muted(trunkIndicator)
		trunkName = colors.Muted(s.Trunk.Name)
	}
	result.WriteString(coloredIndicator)
	result.WriteString(" ")
	result.WriteString(trunkName)
	if s.Trunk.IsCurrent {
		result.WriteString(colors.Muted(" (current)"))
	}
	result.WriteString("\n")

	// Trailing vertical lines after trunk
	verticalLine := colors.Muted(chars.Vertical)
	for n := 0; n < 3; n++ {
		result.WriteString(verticalLine)
		result.WriteString("\n")
	}

	return result.String()
}

// getTimeSinceLastCommit returns relative time since the last commit on a branch
func getTimeSinceLastCommit(repo *git.Repo, branch string) string {
	output, err := repo.RunGitCommand("log", "-1", "--format=%cr", branch)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

// getCommitTimestamp returns the Unix timestamp of the last commit on a branch
func getCommitTimestamp(repo *git.Repo, branch string) int64 {
	output, err := repo.RunGitCommand("log", "-1", "--format=%ct", branch)
	if err != nil {
		return 0
	}
	var timestamp int64
	if _, err := fmt.Sscanf(strings.TrimSpace(output), "%d", &timestamp); err != nil {
		return 0
	}
	return timestamp
}

// getTrunkCommits returns the last n commits on trunk
func getTrunkCommits(repo *git.Repo, branch string, n int) []Commit {
	output, err := repo.RunGitCommand("log", "--oneline", fmt.Sprintf("-%d", n), branch)
	if err != nil || output == "" {
		return nil
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 2 {
			commits = append(commits, Commit{
				SHA:     parts[0],
				Message: parts[1],
			})
		} else if len(parts) == 1 {
			commits = append(commits, Commit{
				SHA:     parts[0],
				Message: "",
			})
		}
	}

	return commits
}

// GetBranchCommits returns the commits unique to this branch (not in parent).
func (s *Stack) GetBranchCommits(repo *git.Repo, node *Node) []Commit {
	return s.getBranchCommits(repo, node)
}

// getBranchCommits returns the commits unique to this branch (not in parent)
func (s *Stack) getBranchCommits(repo *git.Repo, node *Node) []Commit {
	if node.Parent == nil {
		return nil
	}

	output, err := repo.RunGitCommand("log", "--oneline", "--reverse",
		fmt.Sprintf("%s..%s", node.Parent.Name, node.Name))
	if err != nil || output == "" {
		return nil
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 2 {
			commits = append(commits, Commit{
				SHA:     parts[0],
				Message: parts[1],
			})
		} else if len(parts) == 1 {
			commits = append(commits, Commit{
				SHA:     parts[0],
				Message: "",
			})
		}
	}

	return commits
}

// RenderPath renders a path from trunk to a branch
func (s *Stack) RenderPath(branch string) string {
	path := s.FindPath(branch)
	if path == nil {
		return ""
	}

	var result strings.Builder
	for i, node := range path {
		if i > 0 {
			result.WriteString(colors.Muted(" → "))
		}

		var name string
		if node.IsCurrent {
			name = colors.BranchCurrent(node.Name)
		} else if node.IsTrunk {
			name = colors.BranchTrunk(node.Name)
		} else {
			name = colors.CycleText(node.Name, i)
		}
		result.WriteString(name)
	}

	return result.String()
}
