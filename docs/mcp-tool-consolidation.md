# MCP Tool Consolidation

Reduce the MCP tool surface from 18 tools to 11 by merging overlapping tools and
dropping niche operations that agents don't use. The goal is a clean, focused API
where every tool has a single clear purpose and refs sync works 100% out of the box.

**Guiding principle**: Keep core functionality, make it bulletproof with ref sync,
and remove anything an agent doesn't need in a typical stacking workflow.

---

## Table of Contents

1. [Why Do This](#1-why-do-this)
2. [Current State (18 tools)](#2-current-state-18-tools)
3. [Target State (11 tools)](#3-target-state-11-tools)
4. [Merges](#4-merges)
5. [Drops](#5-drops)
6. [Ref Sync Audit](#6-ref-sync-audit)
7. [Implementation Order](#7-implementation-order)

---

## 1. Why Do This

### The Problem

The MCP surface grew organically — every CLI command got a matching tool. This
creates three issues:

1. **Agent confusion**: With 18 tools, an agent must decide between `gs_status` vs
   `gs_log`, `gs_checkout` vs `gs_navigate`, `gs_branch_info` vs `gs_diff`. The
   overlap is not obvious from descriptions alone, leading to redundant calls and
   wasted tokens.

2. **Maintenance burden**: Every tool needs ref sync wiring, error propagation, and
   parity with the CLI. With 18 tools, bugs hide in rarely-used handlers (`gs_fold`,
   `gs_untrack`, `gs_rename`) that get less testing attention.

3. **Niche tools pollute the tool list**: Tools like `gs_track`, `gs_untrack`,
   `gs_rename`, and `gs_fold` serve human-interactive workflows. Agents create
   branches with `gs_create` (auto-tracks), delete with `gs_delete` (auto-reparents),
   and name branches correctly from the start (no rename). Exposing these as MCP
   tools adds surface area without agent value.

### What Graphite Teaches Us

Graphite's CLI has ~15 commands but their MCP/API surface is much smaller. Their
philosophy: the CLI is for humans (interactive prompts, flexible options), the API
is for machines (minimal, predictable, no ambiguity). We should apply the same
principle.

### The Core Workflow

An agent's typical flow uses 5-6 tools:

```
gs_log → gs_checkout → gs_create → (edit files) → gs_modify → gs_submit
```

Everything else is either a variation of these (navigate = checkout by direction)
or a rare operation (fold, rename, untrack).

---

## 2. Current State (18 tools)

### Read (4 tools)

| Tool | Purpose |
|------|---------|
| `gs_status` | Stack tree + summary (needs_restack list) |
| `gs_log` | Stack tree + optional commit history |
| `gs_branch_info` | Single branch details (commits, needs_restack) |
| `gs_diff` | Unified diff of branch vs parent |

**Overlap**: `gs_status` and `gs_log` without commits return nearly identical data.
`gs_branch_info` and `gs_diff` are both "inspect one branch" tools.

### Navigate (2 tools)

| Tool | Purpose |
|------|---------|
| `gs_checkout` | Switch to branch by name |
| `gs_navigate` | Move up/down/top/bottom in stack |

**Overlap**: Both change the current branch. The only difference is absolute (name)
vs relative (direction).

### Write (12 tools)

| Tool | Purpose | Agent usage |
|------|---------|-------------|
| `gs_create` | Create stacked branch | **High** — every workflow |
| `gs_modify` | Amend/commit + restack | **High** — every workflow |
| `gs_restack` | Rebase onto parents | **High** — after modifications |
| `gs_submit` | Create/update PR | **High** — every workflow |
| `gs_land` | Land merged branch | **Medium** — after merge |
| `gs_delete` | Delete + reparent | **Medium** — cleanup |
| `gs_move` | Reparent branch | **Low** — rare rearrangement |
| `gs_repair` | Fix metadata issues | **Low** — diagnostic |
| `gs_track` | Adopt existing branch | **Rare** — one-time setup |
| `gs_untrack` | Stop tracking branch | **Rare** — niche |
| `gs_rename` | Rename current branch | **Rare** — agents name correctly |
| `gs_fold` | Squash into parent | **Rare** — agents don't squash |

---

## 3. Target State (11 tools)

### Read (3 tools)

| Tool | What it does |
|------|-------------|
| `gs_log` | Stack tree with summary + optional commits. Replaces `gs_status`. |
| `gs_inspect` | Single branch: metadata, commits, needs_restack, optional diff. Replaces `gs_branch_info` + `gs_diff`. |
| `gs_repair` | Metadata health scan. Dry-run default. |

### Navigate (1 tool)

| Tool | What it does |
|------|-------------|
| `gs_checkout` | Switch branch by name OR direction. Replaces `gs_navigate`. |

### Write (7 tools)

| Tool | What it does |
|------|-------------|
| `gs_create` | Create stacked branch. |
| `gs_modify` | Amend/commit + restack children. |
| `gs_restack` | Rebase branches onto parents. |
| `gs_move` | Reparent a branch. |
| `gs_delete` | Delete branch + reparent children. |
| `gs_submit` | Create/update PR. |
| `gs_land` | Land merged branch + cleanup. |

---

## 4. Merges

### 4.1 Merge `gs_status` into `gs_log`

**What**: Remove `gs_status` as a separate tool. Make `gs_log` always return the
summary data (total branches, needs_restack list) that `gs_status` currently
provides. Commits remain optional via `include_commits` param.

**Why**: `gs_status` and `gs_log` without commits return 95% identical data. The
only thing `gs_status` adds is the `summary.needs_restack` array. Including that
in `gs_log` eliminates the need for a separate tool and removes the "which one do
I call first?" question.

**How**:
- Add `summary` field (total_branches, needs_restack) to `gs_log` response
- Update `gs_log` description to say "Start here" (currently on `gs_status`)
- Remove `gs_status` tool registration from `registerReadTools`
- Remove `handleStatus` function and `statusTool` definition
- Update `docs/mcp.md` tool reference
- Update any agent instructions that reference `gs_status`
- All existing `gs_log` tests continue to pass; add test for summary field

**Files**: `internal/mcptools/read.go`

### 4.2 Merge `gs_branch_info` + `gs_diff` into `gs_inspect`

**What**: Replace both `gs_branch_info` and `gs_diff` with a single `gs_inspect`
tool that returns branch metadata (commits, needs_restack, parent, children) and
optionally includes the unified diff.

**Why**: An agent inspecting a branch almost always wants both pieces of information.
Two separate calls waste a round-trip. A single `gs_inspect` with an `include_diff`
parameter gives the agent everything in one call.

**How**:
- Create new `gs_inspect` tool combining `gs_branch_info` response fields with
  optional `diff` field
- Parameters: `branch` (required), `include_diff` (boolean, default false)
- Response: all fields from `gs_branch_info` plus `diff` (string, only when requested)
- Remove `gs_branch_info` and `gs_diff` tool registrations
- Remove their handler functions and tool definitions
- Update `docs/mcp.md`

**Files**: `internal/mcptools/read.go`

### 4.3 Merge `gs_navigate` into `gs_checkout`

**What**: Add optional `direction` and `steps` parameters to `gs_checkout`. If
`branch` is provided, checkout by name (current behavior). If `direction` is
provided, navigate relatively (current `gs_navigate` behavior). Exactly one of
`branch` or `direction` must be provided.

**Why**: Both tools do the same thing — change the current branch. The distinction
between "absolute" and "relative" navigation is a UX detail for the CLI (separate
`up`/`down`/`checkout` commands). For MCP agents, one tool that says "go to this
branch" is cleaner than two that split the concept.

**How**:
- Add `direction` (enum: up/down/top/bottom) and `steps` (number) params to
  `gs_checkout`
- Validation: error if both `branch` and `direction` are provided, or if neither
- Move `handleNavigate` logic into `handleCheckout` under a direction branch
- Remove `gs_navigate` tool registration
- Remove `navigateTool` definition and `handleNavigate` function
- Update `docs/mcp.md`
- Keep the `gs_navigate` ambiguous-navigation error behavior (return options list
  when multiple children exist)

**Files**: `internal/mcptools/write.go`

---

## 5. Drops

### 5.1 Drop `gs_track` from MCP

**What**: Remove `gs_track` from MCP tool registration. Keep the CLI command.

**Why**: Tracking an existing branch is a one-time setup operation that humans do
interactively (select parent from a list). Agents create branches with `gs_create`
which auto-tracks. An agent would never need to adopt a branch that was created
outside of gs — that's a human-driven recovery workflow.

**How**:
- Remove `s.AddTool(trackTool, handleTrack)` from `registerWriteTools`
- Keep `trackTool`, `handleTrack`, and response types in the file (dead code
  removal can happen in a follow-up, or remove now if preferred)
- Update `docs/mcp.md` to remove `gs_track` from tool reference
- No ref sync changes needed — CLI `gs track` still pushes refs

**Files**: `internal/mcptools/write.go`

### 5.2 Drop `gs_untrack` from MCP

**What**: Remove `gs_untrack` from MCP tool registration. Keep the CLI command.

**Why**: If an agent wants to remove a branch, `gs_delete` is the right tool — it
handles reparenting, ref cleanup, and remote deletion. `gs_untrack` is a weaker
operation (stop tracking but leave the git branch) that's only useful for humans
who want to selectively manage what gs knows about. Agents don't need this nuance.

**How**:
- Remove `s.AddTool(untrackTool, handleUntrack)` from `registerWriteTools`
- Remove handler and types (or keep as dead code)
- Update `docs/mcp.md`

**Files**: `internal/mcptools/write.go`

### 5.3 Drop `gs_rename` from MCP

**What**: Remove `gs_rename` from MCP tool registration. Keep the CLI command.

**Why**: Agents name branches correctly at creation time via `gs_create`. Renaming
is a human-driven correction for typos or naming convention changes. In the rare
case an agent needs to rename, it can delete and recreate (which is what rename
does internally anyway — untrack old name, track new name).

**How**:
- Remove `s.AddTool(renameTool, handleRename)` from `registerWriteTools`
- Remove handler and types
- Update `docs/mcp.md`

**Files**: `internal/mcptools/write.go`

### 5.4 Drop `gs_fold` from MCP

**What**: Remove `gs_fold` from MCP tool registration. Keep the CLI command.

**Why**: Folding (squash-merge into parent) is a human-driven workflow decision —
"I realize this branch should have been part of its parent." Agents don't make this
kind of retroactive structural decision. If an agent wants to combine work, it
creates the changes on the right branch from the start. The `gs_modify` tool
(amend + restack) covers the agent use case of updating a branch.

**How**:
- Remove `s.AddTool(foldTool, handleFold)` from `registerWriteTools`
- Remove handler and types
- Update `docs/mcp.md`

**Files**: `internal/mcptools/write.go`

---

## 6. Ref Sync Audit

Before consolidation, verify that every remaining tool correctly syncs refs. This
is the most important constraint — the core functionality must work 100% with ref
sync out of the box.

### Every write tool must:

1. Call `SaveWithRefs` after metadata mutations — **and propagate errors** (not `_ =`)
2. Call `pushMetadataRefs` for affected branches after save
3. Call `deleteRemoteMetadataRef` when untracking/deleting a branch
4. Use batched push (single `git push` with multiple refspecs) for efficiency

### Audit checklist for each remaining write tool:

| Tool | SaveWithRefs | Push refs | Delete remote ref | Batched |
|------|-------------|-----------|-------------------|---------|
| `gs_create` | Verify error propagated | Verify push after save | N/A | N/A (single branch) |
| `gs_modify` | Verify error propagated | Verify push after save | N/A | Verify batched if multiple children |
| `gs_restack` | Verify error propagated | Verify push after save | N/A | Verify batched |
| `gs_move` | Verify error propagated | Verify push after save | N/A | N/A (single branch) |
| `gs_delete` | Verify error propagated | Verify push for reparented children | Verify delete for deleted branch | Verify batched children push |
| `gs_submit` | Verify error propagated | Verify push after PR number stored | N/A | N/A (single branch) |
| `gs_land` | Verify error propagated | Verify push for reparented children | Verify delete for landed branch | Verify batched children push |
| `gs_repair` | Verify error propagated | Verify push after fixes | Verify delete for orphaned refs | Verify batched |
| `gs_checkout` (with direction merge) | No metadata mutation | No push needed | N/A | N/A |

### Ref sync for merged tools:

- `gs_log` (absorbing `gs_status`): Read-only — no ref sync needed
- `gs_inspect` (absorbing `gs_branch_info` + `gs_diff`): Read-only — no ref sync needed
- `gs_checkout` (absorbing `gs_navigate`): No metadata mutation — no ref sync needed

---

## 7. Implementation Order

### Phase 1: Ref sync verification (do this first)

Before changing any tool surface, run the ref sync audit from section 6 against
the current codebase. Fix any remaining gaps. This ensures the foundation is solid
before restructuring on top of it.

- Verify all 7 write tools propagate `SaveWithRefs` errors (PR #17 addressed this)
- Verify all write tools push refs after mutations
- Verify delete/land tools clean up remote refs
- Run full test suite

### Phase 2: Merges (one commit each)

Do the merges in this order — each is independent and testable:

1. **Merge `gs_status` into `gs_log`** — smallest change, just add summary field
2. **Merge `gs_branch_info` + `gs_diff` into `gs_inspect`** — combine two read tools
3. **Merge `gs_navigate` into `gs_checkout`** — combine navigation modes

Each merge should:
- Be a single commit
- Include updated tool descriptions
- Include test updates
- Update `docs/mcp.md`
- Remove old tool registrations

### Phase 3: Drops (one commit)

Drop all four tools in a single commit since they're all the same change pattern
(remove registration + handler):

- Drop `gs_track`
- Drop `gs_untrack`
- Drop `gs_rename`
- Drop `gs_fold`

### Phase 4: Documentation

- Update `docs/mcp.md` with new tool reference (11 tools)
- Update `AGENTS.md` if it references specific tool names
- Update `CLAUDE.md` if it references specific tool names
- Update tool descriptions in the remaining handlers to cross-reference correctly
  (e.g., `gs_inspect` description should mention it replaces `gs_branch_info`)

---

**Created**: 2026-03-28
**Status**: Design document — implementation not yet started
**Related**: [MCP Server Architecture](./mcp.md),
[Next Features](./next-features.md),
[Branch Metadata Sync](./branch-metadata-sync.md)
