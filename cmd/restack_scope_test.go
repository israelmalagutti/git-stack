package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-wrapper/internal/config"
	"github.com/israelmalagutti/git-wrapper/internal/stack"
)

// resetRestackFlags resets all scope flags to their defaults before each test
func resetRestackFlags() {
	restackOnly = false
	restackUpstack = false
	restackDownstack = false
	restackBranchFlag = ""
}

func TestComputeRestackBranchesDefault(t *testing.T) {
	resetRestackFlags()

	// Build a stack: main -> feat-a -> feat-a1
	//                     -> feat-b
	trunk := &stack.Node{Name: "main", IsTrunk: true, Children: []*stack.Node{}}
	featA := &stack.Node{Name: "feat-a", Parent: trunk, Children: []*stack.Node{}}
	featA1 := &stack.Node{Name: "feat-a1", Parent: featA, Children: []*stack.Node{}}
	featB := &stack.Node{Name: "feat-b", Parent: trunk, Children: []*stack.Node{}}
	trunk.Children = []*stack.Node{featA, featB}
	featA.Children = []*stack.Node{featA1}

	s := &stack.Stack{
		Trunk:     trunk,
		TrunkName: "main",
		Nodes: map[string]*stack.Node{
			"main":    trunk,
			"feat-a":  featA,
			"feat-a1": featA1,
			"feat-b":  featB,
		},
	}
	cfg := &config.Config{Trunk: "main"}

	// Default from feat-a: ancestors(feat-a) + feat-a + descendants(feat-a)
	branches, err := computeRestackBranches(s, cfg, "feat-a")
	if err != nil {
		t.Fatalf("computeRestackBranches failed: %v", err)
	}

	// Should include feat-a and feat-a1 (ancestors of feat-a are just feat-a itself since trunk is skipped)
	expected := []string{"feat-a", "feat-a1"}
	if len(branches) != len(expected) {
		t.Fatalf("expected %d branches, got %d: %v", len(expected), len(branches), branches)
	}
	for i, b := range branches {
		if b != expected[i] {
			t.Errorf("branch[%d]: expected %s, got %s", i, expected[i], b)
		}
	}
}

func TestComputeRestackBranchesDefaultFromTrunk(t *testing.T) {
	resetRestackFlags()

	trunk := &stack.Node{Name: "main", IsTrunk: true, Children: []*stack.Node{}}
	featA := &stack.Node{Name: "feat-a", Parent: trunk, Children: []*stack.Node{}}
	featB := &stack.Node{Name: "feat-b", Parent: trunk, Children: []*stack.Node{}}
	trunk.Children = []*stack.Node{featA, featB}

	s := &stack.Stack{
		Trunk:     trunk,
		TrunkName: "main",
		Nodes: map[string]*stack.Node{
			"main":   trunk,
			"feat-a": featA,
			"feat-b": featB,
		},
	}
	cfg := &config.Config{Trunk: "main"}

	branches, err := computeRestackBranches(s, cfg, "main")
	if err != nil {
		t.Fatalf("computeRestackBranches failed: %v", err)
	}

	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}
}

func TestComputeRestackBranchesOnly(t *testing.T) {
	resetRestackFlags()
	restackOnly = true

	s := &stack.Stack{TrunkName: "main"}
	cfg := &config.Config{Trunk: "main"}

	branches, err := computeRestackBranches(s, cfg, "feat-a")
	if err != nil {
		t.Fatalf("computeRestackBranches failed: %v", err)
	}

	if len(branches) != 1 || branches[0] != "feat-a" {
		t.Fatalf("expected [feat-a], got %v", branches)
	}
}

func TestComputeRestackBranchesOnlyOnTrunk(t *testing.T) {
	resetRestackFlags()
	restackOnly = true

	s := &stack.Stack{TrunkName: "main"}
	cfg := &config.Config{Trunk: "main"}

	_, err := computeRestackBranches(s, cfg, "main")
	if err == nil {
		t.Fatal("expected error for --only on trunk")
	}
}

func TestComputeRestackBranchesUpstack(t *testing.T) {
	resetRestackFlags()
	restackUpstack = true

	trunk := &stack.Node{Name: "main", IsTrunk: true, Children: []*stack.Node{}}
	featA := &stack.Node{Name: "feat-a", Parent: trunk, Children: []*stack.Node{}}
	featA1 := &stack.Node{Name: "feat-a1", Parent: featA, Children: []*stack.Node{}}
	featA2 := &stack.Node{Name: "feat-a2", Parent: featA, Children: []*stack.Node{}}
	trunk.Children = []*stack.Node{featA}
	featA.Children = []*stack.Node{featA1, featA2}

	s := &stack.Stack{
		Trunk:     trunk,
		TrunkName: "main",
		Nodes: map[string]*stack.Node{
			"main":    trunk,
			"feat-a":  featA,
			"feat-a1": featA1,
			"feat-a2": featA2,
		},
	}
	cfg := &config.Config{Trunk: "main"}

	branches, err := computeRestackBranches(s, cfg, "feat-a")
	if err != nil {
		t.Fatalf("computeRestackBranches failed: %v", err)
	}

	// feat-a + sorted descendants (feat-a1, feat-a2)
	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d: %v", len(branches), branches)
	}
	if branches[0] != "feat-a" {
		t.Errorf("expected feat-a first, got %s", branches[0])
	}
}

func TestComputeRestackBranchesDownstack(t *testing.T) {
	resetRestackFlags()
	restackDownstack = true

	trunk := &stack.Node{Name: "main", IsTrunk: true, Children: []*stack.Node{}}
	featA := &stack.Node{Name: "feat-a", Parent: trunk, Children: []*stack.Node{}}
	featA1 := &stack.Node{Name: "feat-a1", Parent: featA, Children: []*stack.Node{}}
	trunk.Children = []*stack.Node{featA}
	featA.Children = []*stack.Node{featA1}

	s := &stack.Stack{
		Trunk:     trunk,
		TrunkName: "main",
		Nodes: map[string]*stack.Node{
			"main":    trunk,
			"feat-a":  featA,
			"feat-a1": featA1,
		},
	}
	cfg := &config.Config{Trunk: "main"}

	branches, err := computeRestackBranches(s, cfg, "feat-a1")
	if err != nil {
		t.Fatalf("computeRestackBranches failed: %v", err)
	}

	// ancestors of feat-a1 (skip trunk): feat-a, feat-a1
	expected := []string{"feat-a", "feat-a1"}
	if len(branches) != len(expected) {
		t.Fatalf("expected %d branches, got %d: %v", len(expected), len(branches), branches)
	}
	for i, b := range branches {
		if b != expected[i] {
			t.Errorf("branch[%d]: expected %s, got %s", i, expected[i], b)
		}
	}
}

func TestComputeRestackBranchesDownstackOnTrunk(t *testing.T) {
	resetRestackFlags()
	restackDownstack = true

	s := &stack.Stack{TrunkName: "main"}
	cfg := &config.Config{Trunk: "main"}

	_, err := computeRestackBranches(s, cfg, "main")
	if err == nil {
		t.Fatal("expected error for --downstack on trunk")
	}
}

func TestRestackMutuallyExclusiveFlags(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-flags", "main")

	restackOnly = true
	restackUpstack = true
	restackDownstack = false
	restackBranchFlag = ""

	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}

	resetRestackFlags()
}

func TestRestackBranchFlag(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-target", "main")
	repo.commitFile(t, "target.txt", "target", "target commit")

	// Add commit to main so feat-target needs rebase
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit")

	// Use --branch flag while on main
	restackOnly = true
	restackUpstack = false
	restackDownstack = false
	restackBranchFlag = "feat-target"

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack with --branch failed: %v", err)
	}

	// Verify feat-target was restacked
	mergeBase, err := repo.repo.RunGitCommand("merge-base", "feat-target", "main")
	if err != nil {
		t.Fatalf("merge-base failed: %v", err)
	}
	mainSHA, _ := repo.repo.GetBranchCommit("main")
	if mergeBase != mainSHA {
		t.Error("expected feat-target to be restacked onto main tip")
	}

	resetRestackFlags()
}

func TestRestackBranchFlagNotFound(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	restackOnly = false
	restackUpstack = false
	restackDownstack = false
	restackBranchFlag = "nonexistent"

	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent --branch")
	}

	resetRestackFlags()
}
