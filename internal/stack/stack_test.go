package stack

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestNode(t *testing.T) {
	t.Run("node structure", func(t *testing.T) {
		parent := &Node{Name: "main", IsTrunk: true}
		child := &Node{Name: "feat-1", Parent: parent}
		parent.Children = append(parent.Children, child)

		if child.Parent.Name != "main" {
			t.Errorf("expected parent 'main', got '%s'", child.Parent.Name)
		}

		if len(parent.Children) != 1 {
			t.Errorf("expected 1 child, got %d", len(parent.Children))
		}

		if parent.Children[0].Name != "feat-1" {
			t.Errorf("expected child 'feat-1', got '%s'", parent.Children[0].Name)
		}
	})
}

func TestStack(t *testing.T) {
	t.Run("GetNode returns node", func(t *testing.T) {
		s := &Stack{
			Nodes: map[string]*Node{
				"main":   {Name: "main"},
				"feat-1": {Name: "feat-1"},
			},
		}

		node := s.GetNode("feat-1")
		if node == nil {
			t.Fatal("expected node, got nil")
		}
		if node.Name != "feat-1" {
			t.Errorf("expected 'feat-1', got '%s'", node.Name)
		}
	})

	t.Run("GetNode returns nil for unknown", func(t *testing.T) {
		s := &Stack{Nodes: map[string]*Node{}}

		node := s.GetNode("unknown")
		if node != nil {
			t.Error("expected nil for unknown branch")
		}
	})

	t.Run("GetParent returns parent node", func(t *testing.T) {
		parent := &Node{Name: "main"}
		child := &Node{Name: "feat-1", Parent: parent}

		s := &Stack{
			Nodes: map[string]*Node{
				"main":   parent,
				"feat-1": child,
			},
		}

		p := s.GetParent("feat-1")
		if p == nil {
			t.Fatal("expected parent, got nil")
		}
		if p.Name != "main" {
			t.Errorf("expected 'main', got '%s'", p.Name)
		}
	})

	t.Run("GetParent returns nil for unknown branch", func(t *testing.T) {
		s := &Stack{Nodes: map[string]*Node{}}
		if parent := s.GetParent("missing"); parent != nil {
			t.Errorf("expected nil parent for missing branch")
		}
	})

	t.Run("GetChildren returns children", func(t *testing.T) {
		parent := &Node{Name: "main", Children: []*Node{}}
		child1 := &Node{Name: "feat-1", Parent: parent}
		child2 := &Node{Name: "feat-2", Parent: parent}
		parent.Children = append(parent.Children, child1, child2)

		s := &Stack{
			Nodes: map[string]*Node{
				"main":   parent,
				"feat-1": child1,
				"feat-2": child2,
			},
		}

		children := s.GetChildren("main")
		if len(children) != 2 {
			t.Errorf("expected 2 children, got %d", len(children))
		}
	})

	t.Run("GetChildren returns nil for missing branch", func(t *testing.T) {
		s := &Stack{Nodes: map[string]*Node{}}
		if children := s.GetChildren("missing"); children != nil {
			t.Errorf("expected nil children for missing branch")
		}
	})

	t.Run("FindPath returns path from trunk", func(t *testing.T) {
		trunk := &Node{Name: "main", IsTrunk: true}
		feat1 := &Node{Name: "feat-1", Parent: trunk}
		feat2 := &Node{Name: "feat-2", Parent: feat1}

		s := &Stack{
			Trunk: trunk,
			Nodes: map[string]*Node{
				"main":   trunk,
				"feat-1": feat1,
				"feat-2": feat2,
			},
		}

		path := s.FindPath("feat-2")
		if len(path) != 3 {
			t.Fatalf("expected path length 3, got %d", len(path))
		}

		if path[0].Name != "main" {
			t.Errorf("expected path[0] 'main', got '%s'", path[0].Name)
		}
		if path[1].Name != "feat-1" {
			t.Errorf("expected path[1] 'feat-1', got '%s'", path[1].Name)
		}
		if path[2].Name != "feat-2" {
			t.Errorf("expected path[2] 'feat-2', got '%s'", path[2].Name)
		}
	})

	t.Run("GetStackDepth returns correct depth", func(t *testing.T) {
		trunk := &Node{Name: "main", IsTrunk: true}
		feat1 := &Node{Name: "feat-1", Parent: trunk}
		feat2 := &Node{Name: "feat-2", Parent: feat1}

		s := &Stack{
			Trunk: trunk,
			Nodes: map[string]*Node{
				"main":   trunk,
				"feat-1": feat1,
				"feat-2": feat2,
			},
		}

		if depth := s.GetStackDepth("main"); depth != 0 {
			t.Errorf("expected depth 0 for trunk, got %d", depth)
		}

		if depth := s.GetStackDepth("feat-1"); depth != 1 {
			t.Errorf("expected depth 1 for feat-1, got %d", depth)
		}

		if depth := s.GetStackDepth("feat-2"); depth != 2 {
			t.Errorf("expected depth 2 for feat-2, got %d", depth)
		}
		if depth := s.GetStackDepth("missing"); depth != -1 {
			t.Errorf("expected depth -1 for missing branch, got %d", depth)
		}
	})

	t.Run("GetAllBranches returns all branches", func(t *testing.T) {
		s := &Stack{
			Nodes: map[string]*Node{
				"main":   {Name: "main"},
				"feat-1": {Name: "feat-1"},
				"feat-2": {Name: "feat-2"},
			},
		}

		branches := s.GetAllBranches()
		if len(branches) != 3 {
			t.Errorf("expected 3 branches, got %d", len(branches))
		}
	})

	t.Run("SortedChildren returns nil when empty", func(t *testing.T) {
		node := &Node{Name: "main"}
		if children := node.SortedChildren(); children != nil {
			t.Errorf("expected nil children when empty")
		}
	})
}

func TestValidateStack(t *testing.T) {
	t.Run("valid stack passes", func(t *testing.T) {
		trunk := &Node{Name: "main", IsTrunk: true}
		feat1 := &Node{Name: "feat-1", Parent: trunk}

		s := &Stack{
			Trunk: trunk,
			Nodes: map[string]*Node{
				"main":   trunk,
				"feat-1": feat1,
			},
		}

		if err := s.ValidateStack(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("detects cycle", func(t *testing.T) {
		feat1 := &Node{Name: "feat-1"}
		feat2 := &Node{Name: "feat-2", Parent: feat1}
		feat1.Parent = feat2

		s := &Stack{
			Nodes: map[string]*Node{
				"feat-1": feat1,
				"feat-2": feat2,
			},
		}

		err := s.ValidateStack()
		if err == nil {
			t.Error("expected cycle detection error")
		}
	})
}

func TestBuildStackMetadata(t *testing.T) {
	t.Run("builds parent-child relationships from metadata", func(t *testing.T) {
		meta := &config.Metadata{
			Branches: map[string]*config.BranchMetadata{
				"feat-1": {Parent: "main"},
				"feat-2": {Parent: "feat-1"},
			},
		}

		if meta.Branches["feat-1"].Parent != "main" {
			t.Errorf("expected feat-1 parent 'main', got '%s'", meta.Branches["feat-1"].Parent)
		}

		if meta.Branches["feat-2"].Parent != "feat-1" {
			t.Errorf("expected feat-2 parent 'feat-1', got '%s'", meta.Branches["feat-2"].Parent)
		}
	})
}
