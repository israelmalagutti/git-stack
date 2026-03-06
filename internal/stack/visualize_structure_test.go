package stack

import (
	"strings"
	"testing"

	"github.com/israelmalagutti/git-wrapper/internal/colors"
)

func init() {
	// Disable colors for deterministic test output
	colors.SetEnabled(false)
}

// buildStack is a test helper to build a stack with the given tree structure.
// Automatically sets IsCurrent on the matching node.
func buildStack(trunk *Node, nodes map[string]*Node, current string) *Stack {
	for _, node := range nodes {
		node.IsCurrent = (node.Name == current)
	}
	return &Stack{
		Trunk:     trunk,
		Nodes:     nodes,
		Current:   current,
		TrunkName: trunk.Name,
	}
}

func TestRenderShortLinearStack(t *testing.T) {
	// main → feat-a (single child, no merge line)
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "feat-a", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a)

	s := buildStack(trunk, map[string]*Node{
		"main":   trunk,
		"feat-a": a,
	}, "feat-a")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ feat-a (current)",
		"│",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort linear stack mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestRenderShortTwoChildren(t *testing.T) {
	// main → [feat-a (primary/oldest), feat-b (secondary)]
	// With nil repo, alphabetical sort: feat-a < feat-b
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "feat-a", Parent: trunk, Children: []*Node{}}
	b := &Node{Name: "feat-b", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a, b)

	s := buildStack(trunk, map[string]*Node{
		"main":   trunk,
		"feat-a": a,
		"feat-b": b,
	}, "feat-a")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ feat-a (current)",
		"│",
		"│",
		"│   ○ feat-b",
		"│   │",
		"├───┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort two children mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestRenderShortThreeChildren(t *testing.T) {
	// main → [a (primary), b (secondary), c (secondary)]
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "a", Parent: trunk, Children: []*Node{}}
	b := &Node{Name: "b", Parent: trunk, Children: []*Node{}}
	c := &Node{Name: "c", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a, b, c)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk,
		"a":    a,
		"b":    b,
		"c":    c,
	}, "a")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ a (current)",
		"│",
		"│",
		"│   ○ b",
		"│   │",
		"│   │",
		"│   │   ○ c",
		"│   │   │",
		"├───┴───┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort three children mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestRenderShortNestedBranching(t *testing.T) {
	// main → [a (primary)]
	// a → [a1 (primary), a2 (secondary)]
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "a", Parent: trunk, Children: []*Node{}}
	a1 := &Node{Name: "a1", Parent: a, Children: []*Node{}}
	a2 := &Node{Name: "a2", Parent: a, Children: []*Node{}}
	a.Children = append(a.Children, a1, a2)
	trunk.Children = append(trunk.Children, a)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk,
		"a":    a,
		"a1":   a1,
		"a2":   a2,
	}, "a1")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ a1 (current)",
		"│",
		"│",
		"│   ○ a2",
		"├───┘",
		"○ a",
		"│",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort nested branching mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestRenderShortDeepNesting(t *testing.T) {
	// main → [C0 (primary), C1 (secondary)]
	// C1 → [C1a (primary), C1b (secondary)]
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	c0 := &Node{Name: "C0", Parent: trunk, Children: []*Node{}}
	c1 := &Node{Name: "C1", Parent: trunk, Children: []*Node{}}
	c1a := &Node{Name: "C1a", Parent: c1, Children: []*Node{}}
	c1b := &Node{Name: "C1b", Parent: c1, Children: []*Node{}}
	c1.Children = append(c1.Children, c1a, c1b)
	trunk.Children = append(trunk.Children, c0, c1)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk,
		"C0":   c0,
		"C1":   c1,
		"C1a":  c1a,
		"C1b":  c1b,
	}, "C0")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ C0 (current)",
		"│",
		"│",
		"│   ○ C1a",
		"│   │",
		"│   │",
		"│   │   ○ C1b",
		"│   ├───┘",
		"│   ○ C1",
		"│   │",
		"├───┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort deep nesting mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestRenderShortCurrentOnTrunk(t *testing.T) {
	// main (current) → [feat-a]
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "feat-a", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a)

	s := buildStack(trunk, map[string]*Node{
		"main":   trunk,
		"feat-a": a,
	}, "main")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"○ feat-a",
		"│",
		"│",
		"◉ main (current)",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("RenderShort current on trunk mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}
