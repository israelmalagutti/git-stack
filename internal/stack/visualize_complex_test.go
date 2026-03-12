package stack

import (
	"strings"
	"testing"
)

// buildComplexTree creates the full test structure matching setup-test-repo.sh:
//
//	main (trunk)
//	├── TEST_1
//	│   ├── TEST_1_1
//	│   ├── TEST_1_2
//	│   └── TEST_1_3
//	├── TEST_2
//	│   ├── TEST_2_1
//	│   │   ├── TEST_2_1_1
//	│   │   │   └── TEST_2_1_1_1
//	│   │   └── TEST_2_2
//	│   ├── TEST_2_3
//	│   ├── TEST_2_4
//	│   └── TEST_2_5
//	└── TEST_3
//	    ├── TEST_3_1
//	    ├── TEST_3_2
//	    └── TEST_3_3
func buildComplexTree(current string) *Stack {
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}

	// Level 1
	t1 := &Node{Name: "TEST_1", Parent: trunk, Children: []*Node{}}
	t2 := &Node{Name: "TEST_2", Parent: trunk, Children: []*Node{}}
	t3 := &Node{Name: "TEST_3", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, t1, t2, t3)

	// TEST_1 children
	t1_1 := &Node{Name: "TEST_1_1", Parent: t1, Children: []*Node{}}
	t1_2 := &Node{Name: "TEST_1_2", Parent: t1, Children: []*Node{}}
	t1_3 := &Node{Name: "TEST_1_3", Parent: t1, Children: []*Node{}}
	t1.Children = append(t1.Children, t1_1, t1_2, t1_3)

	// TEST_2 children (direct: TEST_2_1, TEST_2_3, TEST_2_4, TEST_2_5)
	t2_1 := &Node{Name: "TEST_2_1", Parent: t2, Children: []*Node{}}
	t2_3 := &Node{Name: "TEST_2_3", Parent: t2, Children: []*Node{}}
	t2_4 := &Node{Name: "TEST_2_4", Parent: t2, Children: []*Node{}}
	t2_5 := &Node{Name: "TEST_2_5", Parent: t2, Children: []*Node{}}
	t2.Children = append(t2.Children, t2_1, t2_3, t2_4, t2_5)

	// TEST_2_1 children (TEST_2_1_1 is primary, TEST_2_2 is secondary)
	t2_1_1 := &Node{Name: "TEST_2_1_1", Parent: t2_1, Children: []*Node{}}
	t2_2 := &Node{Name: "TEST_2_2", Parent: t2_1, Children: []*Node{}}
	t2_1.Children = append(t2_1.Children, t2_1_1, t2_2)

	// TEST_2_1_1 → TEST_2_1_1_1 (deepest nesting: 4 levels)
	t2_1_1_1 := &Node{Name: "TEST_2_1_1_1", Parent: t2_1_1, Children: []*Node{}}
	t2_1_1.Children = append(t2_1_1.Children, t2_1_1_1)

	// TEST_3 children
	t3_1 := &Node{Name: "TEST_3_1", Parent: t3, Children: []*Node{}}
	t3_2 := &Node{Name: "TEST_3_2", Parent: t3, Children: []*Node{}}
	t3_3 := &Node{Name: "TEST_3_3", Parent: t3, Children: []*Node{}}
	t3.Children = append(t3.Children, t3_1, t3_2, t3_3)

	nodes := map[string]*Node{
		"main": trunk, "TEST_1": t1, "TEST_2": t2, "TEST_3": t3,
		"TEST_1_1": t1_1, "TEST_1_2": t1_2, "TEST_1_3": t1_3,
		"TEST_2_1": t2_1, "TEST_2_2": t2_2, "TEST_2_3": t2_3,
		"TEST_2_4": t2_4, "TEST_2_5": t2_5,
		"TEST_2_1_1": t2_1_1, "TEST_2_1_1_1": t2_1_1_1,
		"TEST_3_1": t3_1, "TEST_3_2": t3_2, "TEST_3_3": t3_3,
	}

	return buildStack(trunk, nodes, current)
}

// TestRenderShortComplexTreeCurrentOnTrunk verifies the full tree rendering
// with current branch on trunk. This is the most comprehensive layout test:
// 3 siblings from main, one with 5 children, deep nesting to 4 levels.
func TestRenderShortComplexTreeCurrentOnTrunk(t *testing.T) {
	s := buildComplexTree("main")
	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"○ TEST_1_1",
		"│",
		"│",
		"│   ○ TEST_1_2",
		"│   │",
		"│   │",
		"│   │   ○ TEST_1_3",
		"├───┴───┘",
		"○ TEST_1",
		"│",
		"│",
		"│           ○ TEST_2_1_1_1",
		"│           │",
		"│           │",
		"│           ○ TEST_2_1_1",
		"│           │",
		"│           │",
		"│           │   ○ TEST_2_2",
		"│           ├───┘",
		"│           ○ TEST_2_1",
		"│           │",
		"│           │",
		"│           │       ○ TEST_2_3",
		"│           │       │",
		"│           │       │",
		"│           │       │   ○ TEST_2_4",
		"│           │       │   │",
		"│           │       │   │",
		"│           │       │   │   ○ TEST_2_5",
		"│           ├───────┴───┴───┘",
		"│           ○ TEST_2",
		"│           │",
		"│           │",
		"│           │                   ○ TEST_3_1",
		"│           │                   │",
		"│           │                   │",
		"│           │                   │   ○ TEST_3_2",
		"│           │                   │   │",
		"│           │                   │   │",
		"│           │                   │   │   ○ TEST_3_3",
		"│           │                   ├───┴───┘",
		"│           │                   ○ TEST_3",
		"│           │                   │",
		"├───────────┴───────────────────┘",
		"│",
		"◉ main (current)",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Complex tree (current=main) mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortComplexTreeCurrentOnLeaf verifies rendering when current
// branch is a deep leaf node (TEST_2_1_1_1, depth 4).
func TestRenderShortComplexTreeCurrentOnLeaf(t *testing.T) {
	s := buildComplexTree("TEST_2_1_1_1")
	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"○ TEST_1_1",
		"│",
		"│",
		"│   ○ TEST_1_2",
		"│   │",
		"│   │",
		"│   │   ○ TEST_1_3",
		"├───┴───┘",
		"○ TEST_1",
		"│",
		"│",
		"│           ◉ TEST_2_1_1_1 (current)",
		"│           │",
		"│           │",
		"│           ○ TEST_2_1_1",
		"│           │",
		"│           │",
		"│           │   ○ TEST_2_2",
		"│           ├───┘",
		"│           ○ TEST_2_1",
		"│           │",
		"│           │",
		"│           │       ○ TEST_2_3",
		"│           │       │",
		"│           │       │",
		"│           │       │   ○ TEST_2_4",
		"│           │       │   │",
		"│           │       │   │",
		"│           │       │   │   ○ TEST_2_5",
		"│           ├───────┴───┴───┘",
		"│           ○ TEST_2",
		"│           │",
		"│           │",
		"│           │                   ○ TEST_3_1",
		"│           │                   │",
		"│           │                   │",
		"│           │                   │   ○ TEST_3_2",
		"│           │                   │   │",
		"│           │                   │   │",
		"│           │                   │   │   ○ TEST_3_3",
		"│           │                   ├───┴───┘",
		"│           │                   ○ TEST_3",
		"│           │                   │",
		"├───────────┴───────────────────┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Complex tree (current=TEST_2_1_1_1) mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortComplexTreeCurrentOnMiddle verifies rendering when current
// branch is a mid-level node with children (TEST_2_1, depth 2).
func TestRenderShortComplexTreeCurrentOnMiddle(t *testing.T) {
	s := buildComplexTree("TEST_2_1")
	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"○ TEST_1_1",
		"│",
		"│",
		"│   ○ TEST_1_2",
		"│   │",
		"│   │",
		"│   │   ○ TEST_1_3",
		"├───┴───┘",
		"○ TEST_1",
		"│",
		"│",
		"│           ○ TEST_2_1_1_1",
		"│           │",
		"│           │",
		"│           ○ TEST_2_1_1",
		"│           │",
		"│           │",
		"│           │   ○ TEST_2_2",
		"│           ├───┘",
		"│           ◉ TEST_2_1 (current)",
		"│           │",
		"│           │",
		"│           │       ○ TEST_2_3",
		"│           │       │",
		"│           │       │",
		"│           │       │   ○ TEST_2_4",
		"│           │       │   │",
		"│           │       │   │",
		"│           │       │   │   ○ TEST_2_5",
		"│           ├───────┴───┴───┘",
		"│           ○ TEST_2",
		"│           │",
		"│           │",
		"│           │                   ○ TEST_3_1",
		"│           │                   │",
		"│           │                   │",
		"│           │                   │   ○ TEST_3_2",
		"│           │                   │   │",
		"│           │                   │   │",
		"│           │                   │   │   ○ TEST_3_3",
		"│           │                   ├───┴───┘",
		"│           │                   ○ TEST_3",
		"│           │                   │",
		"├───────────┴───────────────────┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Complex tree (current=TEST_2_1) mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortFiveChildrenFromOneParent specifically tests the 5-child fan-out
// from TEST_2 in isolation.
func TestRenderShortFiveChildrenFromOneParent(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	parent := &Node{Name: "parent", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, parent)

	c1 := &Node{Name: "c1", Parent: parent, Children: []*Node{}}
	c2 := &Node{Name: "c2", Parent: parent, Children: []*Node{}}
	c3 := &Node{Name: "c3", Parent: parent, Children: []*Node{}}
	c4 := &Node{Name: "c4", Parent: parent, Children: []*Node{}}
	c5 := &Node{Name: "c5", Parent: parent, Children: []*Node{}}
	parent.Children = append(parent.Children, c1, c2, c3, c4, c5)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk, "parent": parent,
		"c1": c1, "c2": c2, "c3": c3, "c4": c4, "c5": c5,
	}, "c1")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ c1 (current)",
		"│",
		"│",
		"│   ○ c2",
		"│   │",
		"│   │",
		"│   │   ○ c3",
		"│   │   │",
		"│   │   │",
		"│   │   │   ○ c4",
		"│   │   │   │",
		"│   │   │   │",
		"│   │   │   │   ○ c5",
		"├───┴───┴───┴───┘",
		"○ parent",
		"│",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Five children mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortFourLevelDeepChain tests a linear chain 4 levels deep
// (no branching, just depth).
func TestRenderShortFourLevelDeepChain(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	l1 := &Node{Name: "L1", Parent: trunk, Children: []*Node{}}
	l2 := &Node{Name: "L2", Parent: l1, Children: []*Node{}}
	l3 := &Node{Name: "L3", Parent: l2, Children: []*Node{}}
	l4 := &Node{Name: "L4", Parent: l3, Children: []*Node{}}
	trunk.Children = append(trunk.Children, l1)
	l1.Children = append(l1.Children, l2)
	l2.Children = append(l2.Children, l3)
	l3.Children = append(l3.Children, l4)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk, "L1": l1, "L2": l2, "L3": l3, "L4": l4,
	}, "L4")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ L4 (current)",
		"│",
		"│",
		"○ L3",
		"│",
		"│",
		"○ L2",
		"│",
		"│",
		"○ L1",
		"│",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Four level deep chain mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortDeepNestingWithBranching tests 4 levels deep where
// intermediate nodes also branch (the TEST_2_1 subtree scenario).
func TestRenderShortDeepNestingWithBranching(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}
	a := &Node{Name: "A", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a)

	// A has two children: B (primary) and B2 (secondary)
	b := &Node{Name: "B", Parent: a, Children: []*Node{}}
	b2 := &Node{Name: "B2", Parent: a, Children: []*Node{}}
	a.Children = append(a.Children, b, b2)

	// B has a child C, and C has a child D
	c := &Node{Name: "C", Parent: b, Children: []*Node{}}
	b.Children = append(b.Children, c)
	d := &Node{Name: "D", Parent: c, Children: []*Node{}}
	c.Children = append(c.Children, d)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk, "A": a, "B": b, "B2": b2, "C": c, "D": d,
	}, "D")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ D (current)",
		"│",
		"│",
		"○ C",
		"│",
		"│",
		"○ B",
		"│",
		"│",
		"│   ○ B2",
		"├───┘",
		"○ A",
		"│",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Deep nesting with branching mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// TestRenderShortThreeSiblingsEachWithChildren tests three siblings from trunk,
// each having their own children — the widest part of the tree.
func TestRenderShortThreeSiblingsEachWithChildren(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true, Children: []*Node{}}

	a := &Node{Name: "A", Parent: trunk, Children: []*Node{}}
	b := &Node{Name: "B", Parent: trunk, Children: []*Node{}}
	c := &Node{Name: "C", Parent: trunk, Children: []*Node{}}
	trunk.Children = append(trunk.Children, a, b, c)

	a1 := &Node{Name: "A1", Parent: a, Children: []*Node{}}
	a2 := &Node{Name: "A2", Parent: a, Children: []*Node{}}
	a.Children = append(a.Children, a1, a2)

	b1 := &Node{Name: "B1", Parent: b, Children: []*Node{}}
	b2 := &Node{Name: "B2", Parent: b, Children: []*Node{}}
	b.Children = append(b.Children, b1, b2)

	c1 := &Node{Name: "C1", Parent: c, Children: []*Node{}}
	c.Children = append(c.Children, c1)

	s := buildStack(trunk, map[string]*Node{
		"main": trunk, "A": a, "B": b, "C": c,
		"A1": a1, "A2": a2, "B1": b1, "B2": b2, "C1": c1,
	}, "A1")

	got := s.RenderShort(nil)
	expected := strings.Join([]string{
		"◉ A1 (current)",
		"│",
		"│",
		"│   ○ A2",
		"├───┘",
		"○ A",
		"│",
		"│",
		"│       ○ B1",
		"│       │",
		"│       │",
		"│       │   ○ B2",
		"│       ├───┘",
		"│       ○ B",
		"│       │",
		"│       │",
		"│       │       ○ C1",
		"│       │       │",
		"│       │       │",
		"│       │       ○ C",
		"│       │       │",
		"├───────┴───────┘",
		"│",
		"○ main",
		"│",
		"│",
		"│",
		"",
	}, "\n")

	if got != expected {
		t.Errorf("Three siblings each with children mismatch.\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}
