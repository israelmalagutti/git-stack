package stack

import (
	"strings"
	"testing"
	"time"

	"github.com/israelmalagutti/git-stack/internal/colors"
)

func TestRenderShortAndPath(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true}
	a := &Node{Name: "feat-a", Parent: trunk}
	b := &Node{Name: "feat-b", Parent: trunk}
	trunk.Children = []*Node{a, b}

	s := &Stack{
		Trunk: trunk,
		Nodes: map[string]*Node{
			"main":   trunk,
			"feat-a": a,
			"feat-b": b,
		},
		Current:   "feat-a",
		TrunkName: "main",
	}

	short := s.RenderShort(nil)
	if !strings.Contains(short, "main") {
		t.Fatalf("expected trunk in short render")
	}

	path := s.RenderPath("feat-a")
	if path == "" {
		t.Fatalf("expected path output")
	}
}

func TestRenderTreeNilRepo(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true}
	a := &Node{Name: "feat-a", Parent: trunk}
	b := &Node{Name: "feat-b", Parent: trunk}
	c := &Node{Name: "feat-c", Parent: a}
	trunk.Children = []*Node{a}
	a.Children = []*Node{c}
	trunk.Children = append(trunk.Children, b)

	s := &Stack{
		Trunk: trunk,
		Nodes: map[string]*Node{
			"main":   trunk,
			"feat-a": a,
			"feat-b": b,
			"feat-c": c,
		},
		Current:   "feat-a",
		TrunkName: "main",
	}

	out := s.RenderTree(nil, TreeOptions{})
	if out == "" {
		t.Fatalf("expected render output")
	}
}

func TestCommitHelpersWithRepo(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	metadata.TrackBranch("feat-commit", "main", "")
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}
	if err := repo.CreateBranch("feat-commit"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat-commit"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	s, err := BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}

	trunkCommits := getTrunkCommits(repo, "main", 1)
	if len(trunkCommits) == 0 {
		t.Fatalf("expected trunk commits")
	}

	node := s.GetNode("feat-commit")
	if node == nil {
		t.Fatalf("expected node")
	}
	branchCommits := s.getBranchCommits(repo, node)
	if len(branchCommits) == 0 {
		t.Fatalf("expected branch commits")
	}

	timeAgo := getTimeSinceLastCommit(repo, "main")
	if timeAgo == "" {
		t.Fatalf("expected time ago for trunk")
	}

	if ts := getCommitTimestamp(repo, "main"); ts <= 0 {
		t.Fatalf("expected commit timestamp")
	}
	if getTimeSinceLastCommit(repo, "missing") != "" {
		t.Fatalf("expected empty time for missing branch")
	}
	if getCommitTimestamp(repo, "missing") != 0 {
		t.Fatalf("expected zero timestamp for missing branch")
	}

	treeOut := s.RenderTree(repo, TreeOptions{ShowCommitSHA: true, ShowCommitMsg: true, Detailed: true})
	if treeOut == "" {
		t.Fatalf("expected render tree output")
	}

	shortOut := s.RenderShort(repo)
	if shortOut == "" {
		t.Fatalf("expected short render output")
	}

	if getTrunkCommits(repo, "missing", 1) != nil {
		t.Fatalf("expected nil trunk commits for missing branch")
	}
	if s.getBranchCommits(repo, &Node{Name: "orphan"}) != nil {
		t.Fatalf("expected nil branch commits for orphan node")
	}

	if s.RenderPath("missing") != "" {
		t.Fatalf("expected empty path for missing branch")
	}
}

func TestFormatPRLink(t *testing.T) {
	colors.SetEnabled(false)
	defer colors.SetEnabled(false)

	// No PR number → empty string
	node := &Node{Name: "feat-a"}
	if got := formatPRLink(node); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// PR number without URL → plain #N
	node.PRNumber = 42
	got := formatPRLink(node)
	if !strings.Contains(got, "#42") {
		t.Errorf("expected #42 in %q", got)
	}

	// PR number with URL → still contains #N (hyperlink escapes stripped when colors disabled)
	node.PRURL = "https://github.com/owner/repo/pull/42"
	got = formatPRLink(node)
	if !strings.Contains(got, "#42") {
		t.Errorf("expected #42 in %q", got)
	}
}

func TestSetPRURLs(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true}
	a := &Node{Name: "feat-a", Parent: trunk, PRNumber: 10}
	b := &Node{Name: "feat-b", Parent: trunk}
	trunk.Children = []*Node{a, b}

	s := &Stack{
		Trunk: trunk,
		Nodes: map[string]*Node{
			"main":   trunk,
			"feat-a": a,
			"feat-b": b,
		},
	}

	s.SetPRURLs("https://github.com/owner/repo")

	if a.PRURL != "https://github.com/owner/repo/pull/10" {
		t.Errorf("expected PR URL for feat-a, got %q", a.PRURL)
	}
	if b.PRURL != "" {
		t.Errorf("expected empty PR URL for feat-b, got %q", b.PRURL)
	}
	if trunk.PRURL != "" {
		t.Errorf("expected empty PR URL for trunk, got %q", trunk.PRURL)
	}
}

func TestRenderTreeWithPRLink(t *testing.T) {
	colors.SetEnabled(false)
	defer colors.SetEnabled(false)

	trunk := &Node{Name: "main", IsTrunk: true}
	a := &Node{Name: "feat-a", Parent: trunk, PRNumber: 7, PRURL: "https://github.com/o/r/pull/7"}
	trunk.Children = []*Node{a}

	s := &Stack{
		Trunk: trunk,
		Nodes: map[string]*Node{
			"main":   trunk,
			"feat-a": a,
		},
		TrunkName: "main",
	}

	tree := s.RenderTree(nil, TreeOptions{})
	if !strings.Contains(tree, "#7") {
		t.Errorf("expected #7 in tree output:\n%s", tree)
	}

	short := s.RenderShort(nil)
	if !strings.Contains(short, "#7") {
		t.Errorf("expected #7 in short output:\n%s", short)
	}
}

func TestSortChildrenByTime(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	if err := repo.CreateBranch("feat-old"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat-old"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "old commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	metadata.TrackBranch("feat-old", "main", "")

	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := repo.CreateBranch("feat-new"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat-new"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	time.Sleep(time.Second)
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "new commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	metadata.TrackBranch("feat-new", "main", "")

	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	s, err := BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}

	layout := &columnLayout{
		columns:        make(map[string]int),
		sortedCache:    make(map[string][]*Node),
		timestampCache: make(map[string]int64),
	}
	children := layout.getSortedChildren(repo, s.Trunk)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	// Layout sorts oldest-first for rendering
	if children[0].Name != "feat-old" {
		t.Fatalf("expected oldest branch first, got %s", children[0].Name)
	}
}
