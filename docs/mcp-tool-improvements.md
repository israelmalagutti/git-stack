# MCP Tool Improvements Plan

Based on [Anthropic's best practices for writing effective tools for AI agents](https://www.anthropic.com/engineering/writing-tools-for-agents).

## Current State: 15 Tools (4 read, 11 write)

The tools work correctly but their descriptions only say **what** they do — not **when** to use them, **how** they chain together, or **what to do next**. Error messages are plain strings with no actionable guidance.

---

## 1. Tool Description Improvements

### Read Tools

#### gs_status

**Current:**
> Get the full stack state: all branches, their parents, children, current branch, and trunk. Returns structured JSON representing the entire stack tree.

**Proposed:**
> Start here. Returns the full stack state as structured JSON: all tracked branches, their parent-child relationships, current branch, and trunk name.
>
> Use this as your first call to orient yourself in a repository. It gives you the complete picture of the stack tree in one call.
>
> Prefer gs_status over gs_log unless you specifically need commit history on every branch. For detailed info on a single branch (including commits and needs-restack status), follow up with gs_branch_info.
>
> Returns: {trunk, current_branch, initialized, branches: [{name, parent, children[], commit_sha, depth, is_current, is_trunk}]}

**Why:** Agents don't know this should be their first call. The distinction from gs_log is unclear.

---

#### gs_branch_info

**Current:**
> Get detailed information about a specific branch: metadata, commits unique to this branch, parent, children, depth, and whether it needs restacking.

**Proposed:**
> Get detailed information about a single branch: its commits, parent, children, stack depth, and whether it needs restacking.
>
> Use this after gs_status when you need to inspect a specific branch more closely — for example, to see its commits before deciding whether to fold or modify it, or to check if it needs restacking.
>
> If needs_restack is true, the branch has diverged from its parent and you should call gs_restack with scope "only" on that branch.
>
> Returns: {name, parent, children[], commit_sha, commits: [{sha, message}], depth, is_current, is_trunk, needs_restack}

**Why:** Adds chaining hint (needs_restack → gs_restack). Explains when to use it vs gs_status.

---

#### gs_log

**Current:**
> Get the stack tree as structured data. This is the machine-readable equivalent of 'gs log'. Returns branches in topological order with their relationships.

**Proposed:**
> Get the stack tree with optional commit history for every branch. This is a superset of gs_status — use it when you need to see what commits each branch contains.
>
> Call with include_commits=true to get the full commit list per branch. Without it, the response is nearly identical to gs_status.
>
> Prefer gs_status for a quick overview. Use gs_log with include_commits=true when you need to understand the content of the entire stack (e.g., before a large restack or to find which branch contains a specific change).
>
> Returns: {trunk, current_branch, branches: [{name, parent, children[], commit_sha, depth, is_current, is_trunk, commits?: [{sha, message}]}]}

**Why:** The distinction from gs_status was completely unclear. Now agents know: gs_status for quick overview, gs_log for commit details.

---

#### gs_diff

**Current:**
> Get the unified diff for a branch relative to its parent. Shows only the changes unique to this branch.

**Proposed:**
> Get the unified diff for a branch compared to its parent branch. Shows only the changes introduced by this branch, not inherited changes from the parent.
>
> Use this to review what a branch actually changes before deciding to modify, fold, or submit it. Defaults to the current branch if no branch name is provided.
>
> Cannot diff the trunk branch (it has no parent). For large branches, the diff may be very long — consider using gs_branch_info first to check the commit count.
>
> After reviewing, common next steps: gs_modify to amend, gs_fold to squash into parent, or gs_create to add a follow-up branch.
>
> Returns: {branch, parent, diff} where diff is a standard unified diff string.

**Why:** Adds workflow chaining (review → modify/fold/create) and practical warnings (trunk, large diffs).

---

### Navigation Tools

#### gs_checkout

**Current:**
> Switch to a branch in the stack.

**Proposed:**
> Switch to a specific branch by name. Works with any git branch, including branches not tracked by gs.
>
> Use this when you know the exact branch name. For relative navigation within the stack (move to parent, child, or leaf), use gs_navigate instead — it understands the stack structure and moves along parent-child edges.
>
> Returns: {previous_branch, current_branch}

**Why:** Original is 7 words. Agents can't distinguish this from gs_navigate. Now the distinction is explicit.

---

#### gs_navigate

**Current:**
> Move up/down/top/bottom in the stack. Returns the new current branch. If navigation is ambiguous (multiple children when going up), returns the list of options instead of prompting.

**Proposed:**
> Move through the stack along parent-child edges. Direction meanings:
> - "down": toward trunk (to the parent branch)
> - "up": toward leaves (to a child branch)
> - "bottom": jump directly to trunk
> - "top": jump to the leaf of the current stack line
>
> The steps parameter (default 1) only applies to "up" and "down" directions.
>
> IMPORTANT: If the current branch has multiple children, "up" navigation is ambiguous. Instead of guessing, the tool returns {error: "ambiguous_navigation", options: [...]} listing the child branch names. Use gs_checkout to pick one.
>
> Use gs_navigate for relative movement within a stack. Use gs_checkout when you know the exact branch name.
>
> Returns on success: {previous_branch, current_branch, steps_taken}
> Returns on ambiguity: {error: "ambiguous_navigation", direction, options[], message}

**Why:** "Up" meaning toward leaves is counterintuitive. Direction semantics must be explicit. The dual response shape is documented.

---

### Mutation Tools

#### gs_create

**Current:**
> Create a new stacked branch on top of the current branch. Optionally commit staged changes with a message.

**Proposed:**
> Create a new stacked branch on top of the CURRENT branch. The current branch becomes the new branch's parent.
>
> IMPORTANT: Switch to the desired parent branch FIRST using gs_checkout or gs_navigate before calling gs_create.
>
> If commit_message is provided and there are staged changes in the working tree, those changes are committed on the new branch. If commit_message is provided but there are no staged changes, the commit silently does not happen (check commit_created in the response).
>
> Common workflow — creating a stack of branches:
> 1. Work on main, stage changes, call gs_create with name and commit_message
> 2. Stage more changes, call gs_create again for the next branch
> Now you have a 2-deep stack.
>
> Returns: {branch, parent, commit_created}

**Why:** The "current branch = parent" implicit rule is the #1 source of confusion. Made explicit. Silent commit failure documented.

---

#### gs_delete

**Current:**
> Delete a branch from the stack. Children are reparented to the deleted branch's parent. If deleting the current branch, checks out the parent first.

**Proposed:**
> Delete a branch from the stack and from git. Children of the deleted branch are automatically reparented to the deleted branch's parent, preserving the stack structure.
>
> If you delete the branch you're currently on, gs automatically checks out the parent branch first.
>
> Use this to clean up branches that have been merged upstream, or to remove abandoned work. After deleting, consider calling gs_restack with scope "all" to ensure reparented children are properly rebased onto their new parent.
>
> Cannot delete the trunk branch. Always force-deletes (equivalent to git branch -D).
>
> Returns: {deleted, reparented_children[], new_parent, checked_out?}

**Why:** Adds chaining hint (delete → restack reparented children). Documents force-delete behavior.

---

#### gs_track

**Current:**
> Start tracking an existing git branch in the stack by specifying its parent.

**Proposed:**
> Start tracking an existing git branch in the gs stack by declaring its parent. Use this to adopt branches that were created outside of gs (e.g., with plain git checkout -b).
>
> Both the branch and the parent must already exist as git branches. The branch must not already be tracked.
>
> After tracking, call gs_restack with scope "only" on the newly tracked branch to rebase it onto its declared parent if needed.
>
> Returns: {branch, parent}

**Why:** Explains the use case (adopting external branches) and the follow-up action (restack).

---

#### gs_untrack

**Current:**
> Stop tracking a branch in the stack. The git branch is not deleted.

**Proposed:**
> Stop tracking a branch in the gs stack. The git branch itself is NOT deleted — it just stops appearing in gs_status and gs_log.
>
> WARNING: If this branch has children in the stack, those children will become orphaned (their parent reference will point to a branch gs no longer knows about). Consider using gs_move to reparent children before untracking, or use gs_delete instead which handles reparenting automatically.
>
> Cannot untrack the trunk branch.
>
> Returns: {branch}

**Why:** The orphan risk is currently undocumented and will silently break the stack.

---

#### gs_rename

**Current:**
> Rename the current branch and update gs tracking metadata.

**Proposed:**
> Rename the CURRENT branch (both the git branch and all gs tracking metadata). All parent/child references from other branches are updated automatically.
>
> Only works on the current branch. To rename a different branch, call gs_checkout first to switch to it.
>
> Cannot rename the trunk branch. The new name must not collide with an existing branch.
>
> Returns: {old_name, new_name}

**Why:** "Only works on current branch" is non-obvious and critical for correct usage.

---

#### gs_restack

**Current:**
> Rebase branches to maintain parent-child relationships. Rebases each branch onto its parent using precise --onto when possible. Returns the list of restacked branches or conflict info.

**Proposed:**
> Rebase branches to align them with their declared parents in the stack. This is the key operation for keeping a stack consistent after modifications.
>
> PREREQUISITE: Working tree must be clean (no uncommitted changes). Commit or stash changes first.
>
> Scope controls which branches are restacked:
> - "only": just the specified branch onto its parent
> - "upstack": the branch and all its descendants (children, grandchildren, etc.)
> - "downstack": ancestors of the branch (between trunk and the branch)
> - "all" (default): the entire stack
>
> If a rebase conflict occurs, the tool stops and returns conflict info. Resolve conflicts in the working tree using git commands (edit files, git add, git rebase --continue), then call gs_restack again.
>
> Common triggers for restacking: after gs_modify, gs_move, gs_delete, or pulling upstream changes.
>
> Returns: {restacked[], skipped[], conflict?}

**Why:** Scope values were undocumented. The old conflict message referenced a non-existent `gs continue` tool. Prerequisites made explicit.

---

#### gs_modify

**Current:**
> Amend the current branch's last commit (or create a new commit) and restack children. Stage changes with --all before committing if needed.

**Proposed:**
> Amend the last commit on the current branch (or create a new commit) and automatically restack direct children.
>
> Parameters:
> - message: commit message (required for new commits, optional for amends — omit to keep existing message)
> - new_commit: if true, creates a new commit instead of amending (default: false)
> - stage_all: if true, stages all working tree changes before committing (default: false)
>
> Typical workflow: make code changes, then call gs_modify with stage_all=true and a message to amend them into the current branch. Children are automatically rebased to incorporate your changes.
>
> NOTE: Only direct children are restacked. If the stack is deeper, call gs_restack with scope "upstack" afterward to propagate changes through grandchildren and beyond.
>
> Returns: {branch, action ("amended" or "committed"), restacked_children[]}

**Why:** `all` renamed to `stage_all` for clarity. The "only direct children" limitation is critical for deep stacks. CLI flag references removed.

---

#### gs_move

**Current:**
> Move a branch to a new parent. Rebases the branch onto the new parent and restacks descendants.

**Proposed:**
> Move a branch to a new parent, rebasing it onto the target branch. This changes where the branch sits in the stack tree.
>
> The branch is rebased onto the new parent. Descendants of the moved branch are NOT automatically restacked — call gs_restack with branch set to the moved branch and scope "upstack" afterward to propagate the move through the subtree.
>
> Cannot move a branch onto itself or onto one of its own descendants (would create a cycle). Cannot move trunk.
>
> Returns: {branch, old_parent, new_parent}

**Why:** Current description is inaccurate — it claims to restack descendants but does not. This causes agents to skip necessary restack calls.

---

#### gs_fold

**Current:**
> Fold the current branch into its parent using squash merge. Children are reparented to the parent. The branch is deleted unless keep is true.

**Proposed:**
> Squash-merge the current branch into its parent, combining all commits into a single commit on the parent. The branch is deleted afterward unless keep=true.
>
> Children of the folded branch are reparented to the parent. After folding, call gs_restack with scope "upstack" on the parent to rebase reparented children onto the updated parent.
>
> Use this when a branch's changes are complete and you want to collapse them into the parent before continuing work higher in the stack. This is destructive — individual commit history on the folded branch is lost.
>
> Cannot fold the trunk branch.
>
> Returns: {folded, into, kept, reparented_children[]}

**Why:** Adds workflow context (when to fold), chaining (fold → restack), and warns about history loss.

---

## 2. Parameter Naming Improvements

| Tool | Current | Proposed | Rationale |
|------|---------|----------|-----------|
| `gs_modify` | `all` | `stage_all` | `all` is ambiguous — could mean "all branches". `stage_all` makes the git-add behavior explicit. |
| `gs_delete` | `force` | Remove entirely | The parameter is defined but never read in the handler. Dead code confuses agents. |

All other parameter names are already clear enough.

---

## 3. Error Response Improvements

### 3.1 Structured Errors

Replace all `errResult(string)` calls with structured JSON errors:

```go
func structuredError(code, message, hint string) *mcp.CallToolResult {
    data := map[string]string{
        "error":   code,
        "message": message,
        "hint":    hint,
    }
    b, _ := json.Marshal(data)
    return mcp.NewToolResultError(string(b))
}
```

### 3.2 Specific Improvements

**"Branch not tracked" errors** — suggest calling gs_status:
```json
{
  "error": "branch_not_tracked",
  "branch": "feat/foo",
  "message": "branch 'feat/foo' is not tracked by gs",
  "hint": "Call gs_status to see all tracked branches. If this branch exists in git, use gs_track to add it."
}
```

**"Uncommitted changes" errors** — suggest gs_modify or git stash:
```json
{
  "error": "dirty_working_tree",
  "message": "Cannot restack with uncommitted changes.",
  "hint": "Commit changes with gs_modify (with stage_all=true), or stash them before calling gs_restack."
}
```

**Fix the `gs continue` reference** in restack conflict messages:
```
Current: "resolve conflicts, then run gs continue"
Fixed:   "Resolve conflicts with git commands (edit files, git add, git rebase --continue), then call gs_restack again."
```

**Explain silent commit failures** in gs_create — add a `commit_note` field:
```json
{
  "branch": "feat/foo",
  "parent": "main",
  "commit_created": false,
  "commit_note": "commit_message was provided but no staged changes were found."
}
```

---

## 4. Response Format Improvements

### 4.1 Add `needs_restack` summary to gs_status

Currently agents must call gs_branch_info on every branch to find restacking needs. Add a summary:

```json
{
  "trunk": "main",
  "current_branch": "feat/auth",
  "summary": {
    "total_branches": 5,
    "needs_restack": ["feat/auth-tests", "feat/logging"]
  },
  "branches": [...]
}
```

### 4.2 Add `current_branch` to all mutation responses

Some mutation tools (create, delete, fold) change the current branch but don't always report it. Add `current_branch` to every mutation response.

### 4.3 Add warnings to gs_untrack

When untracking a branch that has children:
```json
{
  "branch": "feat/auth",
  "warnings": ["Branch had 2 children (feat/auth-tests, feat/auth-docs) that are now orphaned. Use gs_move to reparent them."]
}
```

---

## 5. Cross-Cutting: Workflow Patterns

Add to CLAUDE.md and/or docs/mcp.md for agent reference:

### Create a stack
```
gs_checkout → gs_create → (stage changes) → gs_create → gs_status
```

### Modify mid-stack
```
gs_checkout → (make changes) → gs_modify(stage_all=true) → gs_restack(scope="upstack")
```

### Merge cascade (branch merged upstream)
```
gs_status → gs_delete(merged branch) → gs_restack(scope="all")
```

### Reorganize
```
gs_status → gs_move(branch, onto) → gs_restack(scope="upstack")
```

### Resolve conflict
```
gs_restack → (conflict) → (git: edit, add, rebase --continue) → gs_restack
```

---

## 6. Priority Ordering

### High Impact
1. **Improve gs_status/gs_log descriptions** to disambiguate them
2. **Add workflow context and chaining hints** to all descriptions
3. **Fix `gs continue` reference** in restack conflict messages
4. **Structured error responses** with `{error, message, hint}` JSON
5. **Fix gs_create silent commit failure** — add explanation field

### Medium Impact
6. **Remove dead `force` parameter** on gs_delete
7. **Rename `all` → `stage_all`** on gs_modify
8. **Add `needs_restack` summary** to gs_status response
9. **Add `current_branch`** to all mutation responses
10. **Add orphan warnings** to gs_untrack
11. **Fix gs_move description** — it claims to restack descendants but doesn't

### Lower Impact
12. Add `next_steps` hints to mutation responses
13. Add workflow documentation to CLAUDE.md
14. Consider merging gs_status and gs_log into one tool

---

## Files to Modify

- `internal/mcptools/tools.go` — tool registration descriptions
- `internal/mcptools/read.go` — read tool handlers, error handling, response formats
- `internal/mcptools/write.go` — write tool handlers, error handling, response formats, dead `force` param
- `internal/mcptools/helpers.go` — add structured error helper, needs_restack computation
- `CLAUDE.md` — add stacking mental model section
- `docs/mcp.md` — update tool reference and add workflow patterns
