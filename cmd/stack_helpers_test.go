package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestIsDescendant(t *testing.T) {
	root := &stack.Node{Name: "main"}
	a := &stack.Node{Name: "feat-a", Parent: root}
	b := &stack.Node{Name: "feat-b", Parent: a}
	root.Children = []*stack.Node{a}
	a.Children = []*stack.Node{b}

	if !isDescendant(root, "feat-b") {
		t.Fatalf("expected feat-b to be descendant")
	}
	if isDescendant(a, "main") {
		t.Fatalf("did not expect main to be descendant of feat-a")
	}
}

func TestIsInDescendants(t *testing.T) {
	root := &stack.Node{Name: "main"}
	a := &stack.Node{Name: "feat-a", Parent: root}
	b := &stack.Node{Name: "feat-b", Parent: a}
	root.Children = []*stack.Node{a}
	a.Children = []*stack.Node{b}

	s := &stack.Stack{
		Trunk: root,
		Nodes: map[string]*stack.Node{
			"main":   root,
			"feat-a": a,
			"feat-b": b,
		},
	}

	if !isInDescendants(s, root, "feat-b") {
		t.Fatalf("expected feat-b in descendants of trunk")
	}
	if isInDescendants(s, a, "main") {
		t.Fatalf("did not expect main in descendants of feat-a")
	}
}

func TestFindLeaves(t *testing.T) {
	root := &stack.Node{Name: "main"}
	a := &stack.Node{Name: "feat-a", Parent: root}
	b := &stack.Node{Name: "feat-b", Parent: root}
	c := &stack.Node{Name: "feat-c", Parent: a}
	root.Children = []*stack.Node{a, b}
	a.Children = []*stack.Node{c}

	leaves := findLeaves(root)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves, got %d", len(leaves))
	}
}
