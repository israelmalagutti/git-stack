package stack

import (
	"fmt"
	"sort"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// Node represents a branch in the stack tree
type Node struct {
	Name      string
	Parent    *Node
	Children  []*Node
	IsTrunk   bool
	IsCurrent bool
	CommitSHA string
}

// Stack represents the entire stack structure
type Stack struct {
	Trunk     *Node
	Nodes     map[string]*Node
	Current   string
	TrunkName string
}

// BuildStack constructs the stack tree from metadata
func BuildStack(repo *git.Repo, cfg *config.Config, metadata *config.Metadata) (*Stack, error) {
	stack := &Stack{
		Nodes:     make(map[string]*Node),
		TrunkName: cfg.Trunk,
	}

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err == nil {
		stack.Current = currentBranch
	}

	// Verify trunk exists
	if !repo.BranchExists(cfg.Trunk) {
		return nil, fmt.Errorf("trunk branch '%s' does not exist", cfg.Trunk)
	}

	// Create trunk node
	trunkSHA, _ := repo.GetBranchCommit(cfg.Trunk)
	trunk := &Node{
		Name:      cfg.Trunk,
		IsTrunk:   true,
		IsCurrent: cfg.Trunk == stack.Current,
		CommitSHA: trunkSHA,
		Children:  []*Node{},
	}
	stack.Trunk = trunk
	stack.Nodes[cfg.Trunk] = trunk

	// Create nodes for all tracked branches (skip if branch doesn't exist)
	for branchName := range metadata.Branches {
		if branchName == cfg.Trunk {
			continue
		}

		// Skip branches that don't exist in git
		if !repo.BranchExists(branchName) {
			continue
		}

		commitSHA, _ := repo.GetBranchCommit(branchName)
		node := &Node{
			Name:      branchName,
			IsCurrent: branchName == stack.Current,
			CommitSHA: commitSHA,
			Children:  []*Node{},
		}
		stack.Nodes[branchName] = node
	}

	// Build parent-child relationships
	for branchName, meta := range metadata.Branches {
		if branchName == cfg.Trunk {
			continue
		}

		child := stack.Nodes[branchName]
		parent := stack.Nodes[meta.Parent]

		if parent != nil && child != nil {
			child.Parent = parent
			parent.Children = append(parent.Children, child)
		}
	}

	return stack, nil
}

// GetNode returns a node by branch name
func (s *Stack) GetNode(branch string) *Node {
	return s.Nodes[branch]
}

// GetParent returns the parent of a branch
func (s *Stack) GetParent(branch string) *Node {
	node := s.GetNode(branch)
	if node == nil {
		return nil
	}
	return node.Parent
}

// GetChildren returns the children of a branch
func (s *Stack) GetChildren(branch string) []*Node {
	node := s.GetNode(branch)
	if node == nil {
		return nil
	}
	return node.Children
}

// FindPath finds the path from trunk to a given branch
func (s *Stack) FindPath(branch string) []*Node {
	node := s.GetNode(branch)
	if node == nil {
		return nil
	}

	// Build path leaf→trunk, then reverse to trunk→leaf.
	// This avoids O(n²) slice prepend on each iteration.
	path := []*Node{}
	current := node
	for current != nil {
		path = append(path, current)
		current = current.Parent
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

// GetStackDepth returns the depth of a branch in the stack (0 = trunk)
func (s *Stack) GetStackDepth(branch string) int {
	path := s.FindPath(branch)
	if path == nil {
		return -1
	}
	return len(path) - 1
}

// GetAllBranches returns all branches in the stack
func (s *Stack) GetAllBranches() []string {
	branches := make([]string, 0, len(s.Nodes))
	for name := range s.Nodes {
		branches = append(branches, name)
	}
	return branches
}

// ValidateStack checks if the stack structure is valid
func (s *Stack) ValidateStack() error {
	// Check for cycles
	visited := make(map[string]bool)
	for name := range s.Nodes {
		if err := s.detectCycle(name, visited, make(map[string]bool)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stack) detectCycle(branch string, visited, recursionStack map[string]bool) error {
	if recursionStack[branch] {
		return fmt.Errorf("cycle detected in stack at branch: %s", branch)
	}

	if visited[branch] {
		return nil
	}

	visited[branch] = true
	recursionStack[branch] = true

	node := s.GetNode(branch)
	if node != nil && node.Parent != nil {
		if err := s.detectCycle(node.Parent.Name, visited, recursionStack); err != nil {
			return err
		}
	}

	recursionStack[branch] = false
	return nil
}

// SortedChildren returns the children of a node sorted alphabetically by name
func (n *Node) SortedChildren() []*Node {
	if len(n.Children) == 0 {
		return nil
	}
	sorted := make([]*Node, len(n.Children))
	copy(sorted, n.Children)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// GetTopologicalOrder returns all non-trunk branches in topological order (parents before children)
func (s *Stack) GetTopologicalOrder() []*Node {
	var result []*Node
	visited := make(map[string]bool)

	var visit func(node *Node)
	visit = func(node *Node) {
		if visited[node.Name] {
			return
		}
		visited[node.Name] = true

		// Add this node if it's not trunk
		if !node.IsTrunk {
			result = append(result, node)
		}

		// Visit children in sorted order for deterministic output
		for _, child := range node.SortedChildren() {
			visit(child)
		}
	}

	// Start from trunk
	if s.Trunk != nil {
		visit(s.Trunk)
	}

	return result
}
