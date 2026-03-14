# Visualization Design Decisions (`gs log`)

This document captures every major design decision behind the `gs log` visualization
and the reasoning that drove each choice.

---

## Table of Contents

1. [Column-Based Layout](#1-column-based-layout)
2. [Oldest-First Ordering](#2-oldest-first-ordering)
3. [Primary Child Column Inheritance](#3-primary-child-column-inheritance)
4. [Flatten-Then-Render Pipeline](#4-flatten-then-render-pipeline)
5. [Active Column Tracking](#5-active-column-tracking)
6. [Merge Line Before Trunk](#6-merge-line-before-trunk)
7. [Diff-Based Branch Commits](#7-diff-based-branch-commits)
8. [Trunk Rendered Separately](#8-trunk-rendered-separately)
9. [Visual Indicators](#9-visual-indicators)
10. [Color System](#10-color-system)
11. [Commit Display Formatting](#11-commit-display-formatting)
12. [Two Render Modes](#12-two-render-modes)
13. [Fallback Sort for Testing](#13-fallback-sort-for-testing)
14. [Recursive Branch-Out Capability](#14-recursive-branch-out-capability)

---

## 1. Column-Based Layout

**Decision:** Each branch is assigned a column index. The tree is rendered as
parallel vertical tracks, not as indented text.

**Why:**
- Indentation-based trees waste horizontal space and become unreadable at depth 4+.
  A stack 6 levels deep would push branch names far to the right.
- Columns allow the primary development line to stay at column 0 as a straight
  vertical line, while forks branch rightward into new columns.
- This mirrors how graphical git tools (GitKraken, Tower, tig) render DAGs —
  users already have this mental model.
- Parallel branches are visually distinct: each occupies its own vertical track.

**Implementation:** `columnLayout` struct in `visualize.go:35-38`. The `assignColumns`
method recursively assigns column indices starting from column 0 at trunk.

**Example output structure:**
```
○ feature-a           (column 1)
│
│
│   ○ feature-b       (column 2)
│   │
│   │
├───┘
│
○ main                (column 0)
│
│
│
```

---

## 2. Oldest-First Ordering

**Decision:** Children of a branch are sorted by commit timestamp ascending
(oldest first). The oldest child appears at the top of the output.

**Why:**
- The oldest branch was created first and is typically the most established
  line of work.
- Reading bottom-to-top from trunk, you see branches in chronological creation
  order — matching how a developer actually built the stack over time.
- Newer branches (more recent work, still in flux) appear closer to trunk,
  making them easy to find.
- This matches Graphite's `gt log` ordering convention.

**Implementation:** `sortChildrenOldestFirst` in `visualize.go:74-91` uses
`git log -1 --format=%ct <branch>` to get Unix timestamps, then sorts ascending.

---

## 3. Primary Child Column Inheritance

**Decision:** The oldest child inherits its parent's column. All subsequent
(newer) children get new columns to the right.

**Why:**
- Creates visual continuity — the "main line" of a stack reads as one
  straight vertical line from top to bottom.
- The oldest child is typically the first branch in a feature stack and
  represents the primary development direction.
- Secondary branches fork visually rightward, making it obvious they diverge
  from the primary path.
- Avoids visual noise: if every child got a new column, the output would be
  an explosion of columns even for simple linear stacks.

**Implementation:** `assignColumns` in `visualize.go:42-59`:
```go
if i == 0 {
    // Primary (oldest) child inherits parent's column
    cl.assignColumns(child, col, repo)
} else {
    // Secondary children get new columns
    cl.maxColumn++
    cl.assignColumns(child, cl.maxColumn, repo)
}
```

---

## 4. Flatten-Then-Render Pipeline

**Decision:** The tree is first flattened into a linear list (`[]flattenedNode`),
then rendered by iterating that list. Layout and rendering are separate phases.

**Why:**
- Separating layout from rendering makes each phase simpler and independently
  testable.
- The flatten step produces a deterministic order: primary subtree above, current
  node, then secondary subtrees below. This mirrors the visual top-to-bottom order.
- Recursive inline rendering would require threading complex state (active columns,
  last-needed tracking) through recursive calls, making the code brittle.
- Having a flat list enables the `colLastNeeded` pre-computation (see next section),
  which would be impossible with single-pass recursive rendering.

**Implementation:** `flattenBranch` (per-subtree, `visualize.go:100-120`) and
`flattenTree` (whole stack, `visualize.go:125-155`).

**Render order for a node:**
1. Recursively flatten primary (oldest) child's subtree → appears above
2. This node itself → appears in the middle
3. Recursively flatten secondary children's subtrees → appear below

---

## 5. Active Column Tracking

**Decision:** Track which columns currently have an active vertical line (`|`)
passing through them, and pre-compute `colLastNeeded` — the last flat-list
index where each column still needs a vertical line.

**Why:**
- Without tracking, vertical lines would either persist forever (visual clutter
  for finished branches) or stop too early (breaking the visual connection
  between a branch and its descendants).
- `colLastNeeded` accounts for descendants in other columns: if branch A is in
  column 1 and its child B is in column 2, column 1 must stay active until
  B is rendered (so the vertical line connects them).
- The algorithm walks up each node's ancestor chain to mark all ancestor columns
  as needed at that index. This correctly handles multi-level nesting.

**Implementation:** `flattenTree` in `visualize.go:136-154` computes `colLastNeeded`.
Column deactivation happens in `RenderTree` at `visualize.go:266-271`:
```go
for c := col; c <= layout.maxColumn; c++ {
    if activeCols[c] && c > 0 && colLastNeeded[c] <= i {
        activeCols[c] = false
    }
}
```

**Why deactivate from `col` to `maxColumn` (not just `col`):** A secondary subtree
may finish rendering, and its column plus any of its own sub-columns should all
be deactivated at that point.

---

## 6. Merge Line Before Trunk

**Decision:** A horizontal merge line (`└───┴───┘`) is drawn between the last
branch and trunk, joining all columns back to column 0.

**Why:**
- Provides a clear visual boundary between the branch tree and the trunk.
- Without it, branches would visually "float" above trunk with no connection,
  making the parent-child relationship to trunk ambiguous.
- The `┴` character at each intermediate column and `┘` at the rightmost column
  visually "closes" each column track, signaling to the reader that those
  branches all merge down to trunk.

**Implementation:** `RenderTree` at `visualize.go:274-288`.

**Example:**
```
○ feature-a
│
│
│   ○ feature-b
│   │
├───┘
│
○ main
│
│
│
```

---

## 7. Diff-Based Branch Commits

**Decision:** For non-trunk branches, commits are shown using
`git log parent..branch` — only commits unique to that branch.

**Why:**
- The user wants to see what work was done ON this branch specifically.
- Showing all commits (including the parent's) would be redundant — the parent's
  commits are already visible on the parent's own line.
- This matches the mental model of stacked diffs: each branch represents an
  incremental change on top of its parent.
- The `--reverse` flag shows commits in chronological order (oldest first), so
  the reader sees the progression of work.

**Implementation:** `getBranchCommits` in `visualize.go:538-569`:
```go
repo.RunGitCommand("log", "--oneline", "--reverse",
    fmt.Sprintf("%s..%s", node.Parent.Name, node.Name))
```

---

## 8. Trunk Rendered Separately

**Decision:** Trunk is not part of the flattened node list. It is rendered
separately after the merge line, showing its last 3 commits.

**Why:**
- Trunk has no parent to diff against, so `parent..branch` doesn't apply.
  Instead, it shows the last N commits as context.
- Trunk is the root of the tree and serves a different visual role — it's the
  "ground level" that all branches grow from.
- Separating trunk avoids special-casing it inside the main rendering loop
  (no "if trunk, skip column logic" conditionals).
- The 3-commit limit keeps trunk's display compact — trunk may have thousands
  of commits, and showing them all would overwhelm the output.

**Implementation:** `renderTrunkWithCommits` in `visualize.go:297-366`.

---

## 9. Visual Indicators

### Branch Markers
**Decision:** Use `◉` (filled circle) for the current branch and `○` (empty circle)
for all others.

**Why:**
- Provides an instant visual scan — you can spot where you are without reading
  branch names.
- The filled/empty distinction is universally understood and works in both
  color and monochrome terminals.
- Borrowed from Graphite's CLI convention, which users of stacking tools
  are already familiar with.

### Connector Lines
**Decision:** A `│` connector line is always rendered after each node (between
the node and the next element below it).

**Why:**
- Ensures visual continuity — without connectors, nodes would appear as
  disconnected dots.
- The connector extends the column's vertical track, maintaining the tree structure
  even when commits are displayed between nodes.

### Time-Ago Display
**Decision:** Show relative time (e.g., "2 hours ago") next to each branch name,
separated by ` · `.

**Why:**
- Helps identify stale branches at a glance without running separate git commands.
- Relative time is more intuitive than absolute timestamps for this use case.
- The ` · ` separator (middle dot) is a common UI pattern for metadata that's
  secondary to the main content.

---

## 10. Color System

### ANSI 16-Color Palette
**Decision:** Use only the 16 standard ANSI colors (codes 30-37, 90-97).

**Why:**
- These colors are defined by the terminal theme, so they adapt automatically
  to both light and dark backgrounds.
- 256-color or truecolor values are hardcoded RGB — they can be invisible or
  ugly on certain themes.
- This is the same approach git itself uses, ensuring visual consistency.

### Semantic Color Assignments
| Element | Color | Why |
|---------|-------|-----|
| Current branch name | Green + Bold | Matches git's convention for current branch |
| Non-current branches | Muted (bright black) | De-emphasized to reduce visual noise |
| Current indicator (◉) | Cycling palette by depth | Adds depth perception to the tree |
| First commit SHA | Yellow | Highlights the tip commit (most recent/relevant) |
| Other commit SHAs | Muted | De-emphasized — the tip is what matters most |
| Connector lines (│─└) | Muted | Structural elements shouldn't compete with content |

### Cycling Palette
**Decision:** The indicator color cycles through `[Cyan, Green, Yellow, Blue,
Magenta, BrightCyan, BrightGreen, BrightYellow, BrightBlue]` based on stack depth.

**Why:**
- Adds visual depth perception — you can estimate how deep a branch is by its color.
- Nine colors before repeating means stacks up to 9 levels deep are distinguishable,
  which covers virtually all real-world use cases.

### NO_COLOR and Pipe Detection
**Decision:** Colors are automatically disabled when:
- `NO_COLOR` env var is set (https://no-color.org/ standard)
- `TERM=dumb`
- stdout is not a terminal (piped to another command)

**Why:**
- `NO_COLOR` is a cross-tool standard — respecting it ensures interoperability.
- ANSI escape codes in piped output break downstream tools (grep, awk, etc.).
- Dumb terminals can't render escape codes and would show raw `\033[...` text.

---

## 11. Commit Display Formatting

### SHA Truncation (7 characters)
**Decision:** Commit SHAs are truncated to 7 characters.

**Why:**
- Standard git convention (`git log --oneline` uses 7 by default).
- 7 hex chars = ~268 million combinations, sufficient to uniquely identify
  commits in any reasonably-sized repository.
- Full 40-char SHAs would waste horizontal space for no practical benefit.

### Message Truncation (50 characters)
**Decision:** Commit messages are truncated to 50 characters (47 + "...").

**Why:**
- Keeps the tree output compact and prevents long messages from wrapping and
  breaking the visual alignment.
- 50 characters is the conventional limit for git commit subject lines anyway.
- The `...` suffix signals truncation clearly.

---

## 12. Two Render Modes

| Mode | Flag | Shows | Use Case |
|------|------|-------|----------|
| Default (`RenderTree`) | none | Branches + commits + time | Full context when reviewing stack |
| Short (`RenderShort`) | `--short` | Branches only | Quick overview, checking structure |

**Why two modes:**
- Sometimes you just need to know "what branches exist and where am I" — commits
  are noise in that context.
- The default mode with commits is essential for code review workflows where you
  need to see what each branch contains.
- Matches the pattern of other CLI tools (e.g., `ls` vs `ls -l`, `git log`
  vs `git log --oneline`).

---

## 13. Fallback Sort for Testing

**Decision:** When `repo` is nil, children are sorted alphabetically instead of
by commit timestamp.

**Why:**
- In tests, there is no real git repository, so commit timestamps don't exist.
- Alphabetical sort provides deterministic, predictable output for test assertions.
- The `sortedChildren` function in `visualize.go:63-71` encapsulates this fallback:
  ```go
  if repo != nil {
      return sortChildrenOldestFirst(repo, node.Children)
  }
  return node.SortedChildren() // alphabetical
  ```

---

## 14. Recursive Branch-Out Capability

**Decision:** Every branch in the tree — regardless of its depth or position —
must be able to have its own children that branch out into their own columns.
No branch is ever a "leaf-only" position in the layout.

**Why:**
- Stacked workflows are inherently recursive. A developer might create
  `feature-a` off `main`, then `feature-a-part2` off `feature-a`, then
  `feature-a-part2-fix` off `feature-a-part2`. The visualization must handle
  this gracefully at any depth.
- Each branch-out gets its own column to the right, so parallel children
  of the same parent never collide visually. A branch at column 3 can have
  children at column 3 (primary/oldest) and column 4, 5, ... (secondary).
- The `assignColumns` algorithm is recursive with no depth limit — it simply
  increments `maxColumn` for each new fork, guaranteeing a unique column for
  every secondary child at every level.

**Invariant:** For any node N at column C with children [C0, C1, C2, ...]:
- C0 (oldest) inherits column C
- C1 gets column `maxColumn + 1`
- C2 gets column `maxColumn + 2`
- Each of C0, C1, C2 can themselves have children that follow the same rule

**Example — 3 levels of branching:**
```
○ C0
│
│
│   ○ C1-3   (column 2, child of C1-2)
│   │
│   │
│   ○ C1-2   (column 1, child of C1)
│   │
│   │
│   │    ○ C1-B
│   ├────┘
│   ○ C1     (column 0, child of main)
│   │
│   │
│   │   ○ C2
│   │   │
│   │   │
│   │   │   ○ C3
│   │   │   │
├───┴───┴───┘
│
○ main
│
│
│
```

Every branch owns its vertical track and can spawn new tracks to the right,
no matter how deep the tree goes.

---

## Visual Reference

Below is an annotated example showing all visual elements:

```
○ feature-a · 2 hours ago          ← oldest child, column 0, empty circle
│ a1b2c3d - Add login endpoint     ← first commit (yellow SHA)
│ e4f5g6h - Fix auth bug           ← subsequent commit (muted SHA)
│                                   ← connector line
│   ○ feature-b · 30 minutes ago   ← secondary child, column 1
│   │ i7j8k9l - Add tests          ← branch-unique commits
│   │                               ← connector line
└───┘                               ← merge line (closes all columns)
○ main · 1 day ago                  ← trunk, rendered separately
│ m0n1o2p - Merge PR #42           ← last 3 trunk commits
│ q3r4s5t - Release v1.2.0
│ u6v7w8x - Update deps
│
```

**Reading direction:** Bottom-to-top traces the stack from trunk to leaf.
Left-to-right traces from primary line to secondary forks.
