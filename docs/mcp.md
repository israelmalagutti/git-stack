# MCP Server (`gs mcp`)

## What is MCP?

MCP (Model Context Protocol) is an open standard created by Anthropic that defines how AI agents communicate with external tools. It works like a USB-C port for AI — a single protocol that lets any compatible agent (Claude Code, Cursor, VS Code Copilot, etc.) discover and call tools exposed by any compatible server.

As of early 2026, MCP is the dominant integration protocol for AI coding agents. Graphite, GitHub, Postgres, and dozens of other tools ship MCP servers. The specification lives at [modelcontextprotocol.io](https://modelcontextprotocol.io).

### How it works

1. An AI agent starts your MCP server as a subprocess (e.g., `gs mcp`)
2. They communicate over **stdio** using JSON-RPC 2.0 (one JSON message per line)
3. The agent discovers available **tools** via a `tools/list` call
4. The agent calls tools by name with structured arguments
5. The server returns structured JSON results

There is no HTTP, no daemon, no port. The server lives for the duration of the agent session and exits when stdin closes.

### Core concepts

| Concept | Description |
|---------|-------------|
| **Server** | A process that exposes tools (our `gs mcp` command) |
| **Tool** | A function the agent can call (e.g., `gs_status`, `gs_create`) |
| **Transport** | How messages flow — we use **stdio** (stdin/stdout) |
| **Capabilities** | What the server supports — we only expose **tools** (not resources or prompts) |

## Why MCP for gs?

### The problem

`gs` output is designed for human eyes — ANSI colors, Unicode box-drawing, column-based tree art. When an AI agent runs `gs log`, it gets a wall of escape codes that it has to parse heuristically. This is fragile, lossy, and wastes tokens.

### What MCP solves

MCP gives AI agents a **native machine interface** to the same data. Instead of parsing:

```
│ ○ feat/auth-tests
│ │   def5678 add test helpers
│ ◉ feat/auth
│     abc1234 add auth middleware
main
```

An agent calls `gs_status` and gets:

```json
{
  "trunk": "main",
  "current_branch": "feat/auth",
  "branches": [
    {"name": "feat/auth", "parent": "main", "children": ["feat/auth-tests"], "depth": 1},
    {"name": "feat/auth-tests", "parent": "feat/auth", "children": [], "depth": 2}
  ]
}
```

This separation means:
- **The CLI stays human-first** — no `--json` flags polluting the interface
- **The MCP server is machine-first** — structured data, no parsing, no ANSI
- **Same engine underneath** — both use `stack.BuildStack()`, `git.Repo`, and `config.Metadata`

### What AI agents can do with this

- Inspect the full stack tree before making changes
- Create stacked branches as part of a multi-PR workflow
- Restack/rebase automatically after modifying a branch mid-stack
- Navigate the stack to understand where they are working
- Delete, move, fold, and rename branches programmatically
- All without parsing terminal output or guessing at state

## Anatomy of `gs mcp`

### Entry point

`gs mcp` is a Cobra subcommand that starts the MCP server over stdio:

```
gs mcp
```

The AI client (e.g., Claude Code) launches this as a subprocess. Configuration example for `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "git-stack": {
      "command": "gs",
      "args": ["mcp"]
    }
  }
}
```

### Architecture

```
cmd/mcp.go                 Cobra subcommand — creates server, registers tools, calls ServeStdio
internal/mcptools/
  tools.go                 Tool registration — wires all tools to the MCP server
  read.go                  Read-only tools (gs_status, gs_branch_info, gs_log, gs_diff)
  write.go                 Mutation tools (gs_create, gs_delete, gs_move, etc.)
  helpers.go               Shared initialization — loads repo, config, metadata, builds stack
```

Every tool call independently loads fresh repo/config/metadata state. No caching between calls — correctness over performance (the overhead is <10ms).

### Stdout discipline

`gs mcp` writes **only** MCP JSON-RPC messages to stdout. All logging goes to stderr. This is critical — any stray `fmt.Println` breaks the protocol. The MCP tools call internal functions (`stack.BuildStack`, `repo.Rebase`, etc.) directly and format results as JSON, never reusing CLI command code that prints to stdout.

### No interactive prompts

CLI commands use `survey/v2` for interactive prompts (branch selection, confirmations). MCP tools never prompt. When a choice is needed, they return the options in the response for the agent to decide:

```json
{"error": "ambiguous_navigation", "direction": "up", "children": ["feat/a", "feat/b"]}
```

## Tools reference

### Read-only tools

#### `gs_status`
**Start here.** Returns the full stack state as structured JSON. Use this as your first call to orient yourself. Includes a summary of which branches need restacking.
- **Parameters**: none
- **Returns**: `{trunk, current_branch, initialized, summary: {total_branches, needs_restack[]}, branches[]}`
- **When to use**: First call to understand the stack. Prefer over `gs_log` unless you need commit history.

#### `gs_branch_info`
Get detailed info about a single branch including its commits and whether it needs restacking.
- **Parameters**: `branch` (string, required)
- **Returns**: `{name, parent, children[], commits[], depth, is_current, is_trunk, needs_restack}`
- **When to use**: After `gs_status` to inspect a specific branch. If `needs_restack` is true, call `gs_restack` with scope `"only"`.

#### `gs_log`
Get the stack tree with optional commit history for every branch. Superset of `gs_status`.
- **Parameters**: `include_commits` (bool, optional — default: false)
- **Returns**: `{trunk, current_branch, branches[]}`
- **When to use**: When you need to see what commits each branch contains (e.g., before a large restack). Without `include_commits`, response is nearly identical to `gs_status`.

#### `gs_diff`
Get the unified diff for a branch compared to its parent. Shows only changes introduced by this branch.
- **Parameters**: `branch` (string, optional — defaults to current)
- **Returns**: `{branch, parent, diff}`
- **When to use**: Before deciding to modify, fold, or submit a branch. Cannot diff trunk.
- **Next steps**: `gs_modify` to amend, `gs_fold` to squash into parent, `gs_create` to add a follow-up branch.

### Navigation tools

#### `gs_checkout`
Switch to a specific branch by name. Works with any git branch, including untracked ones.
- **Parameters**: `branch` (string, required)
- **Returns**: `{previous_branch, current_branch}`
- **When to use**: When you know the exact branch name. For relative movement, use `gs_navigate`.

#### `gs_navigate`
Move through the stack along parent-child edges.
- **Parameters**: `direction` (enum: up/down/top/bottom), `steps` (int, optional — default: 1, only for up/down)
- **Returns**: `{previous_branch, current_branch, steps_taken}` or `{error: "ambiguous_navigation", options[]}` when multiple children exist
- **Direction meanings**: `down` = toward trunk, `up` = toward leaves, `bottom` = jump to trunk, `top` = jump to leaf
- **When to use**: For relative stack movement. Use `gs_checkout` when you know the target branch name.

### Mutation tools — branch lifecycle

#### `gs_create`
Create a new stacked branch on top of the **current** branch. Switch to the desired parent first.
- **Parameters**: `name` (string, required), `commit_message` (string, optional)
- **Returns**: `{branch, parent, commit_created}`
- **When to use**: Starting new work that depends on the current branch. Call repeatedly to build a stack.

#### `gs_delete`
Delete a branch from the stack and git. Children are automatically reparented.
- **Parameters**: `branch` (string, required)
- **Returns**: `{deleted, reparented_children[], new_parent, checked_out?}`
- **When to use**: Cleaning up merged or abandoned branches. Follow with `gs_restack` scope `"all"` to rebase reparented children.

#### `gs_track`
Start tracking an existing git branch in the stack. Use to adopt branches created outside gs.
- **Parameters**: `branch` (string, required), `parent` (string, required)
- **Returns**: `{branch, parent}`
- **When to use**: After someone creates a branch with plain `git checkout -b`. Follow with `gs_restack` scope `"only"` to align it.

#### `gs_untrack`
Stop tracking a branch. The git branch is **not** deleted.
- **Parameters**: `branch` (string, required)
- **Returns**: `{branch, warnings[]}`
- **When to use**: Removing a branch from gs without deleting it. **Warning**: children become orphaned — reparent with `gs_move` first, or use `gs_delete` instead.

#### `gs_rename`
Rename the **current** branch. Updates all metadata references automatically.
- **Parameters**: `new_name` (string, required)
- **Returns**: `{old_name, new_name}`
- **When to use**: Before submitting a PR, or to fix naming. Switch to the branch first with `gs_checkout`.

### Mutation tools — stack operations

#### `gs_restack`
Rebase branches to align with their declared parents. The key operation for stack consistency.
- **Parameters**: `branch` (string, optional — defaults to current), `scope` (enum: only/upstack/downstack/all — default: all)
- **Returns**: `{restacked[], skipped[], conflict?}`
- **Scope meanings**: `only` = single branch, `upstack` = branch + descendants, `downstack` = ancestors to trunk, `all` = entire stack
- **When to use**: After `gs_modify`, `gs_move`, `gs_delete`, or pulling upstream changes. **Prerequisite**: clean working tree.

#### `gs_modify`
Amend the current branch's last commit (or create a new one) and restack direct children.
- **Parameters**: `message` (string, optional), `new_commit` (bool, default: false), `stage_all` (bool, default: false)
- **Returns**: `{branch, action, restacked_children[]}`
- **When to use**: After making changes to the current branch. Only direct children are restacked — for deeper stacks, follow with `gs_restack` scope `"upstack"`.

#### `gs_move`
Move a branch to a new parent, rebasing onto the target.
- **Parameters**: `branch` (string, optional — defaults to current), `onto` (string, required)
- **Returns**: `{branch, old_parent, new_parent}`
- **When to use**: Reorganizing the stack. Descendants are NOT automatically restacked — follow with `gs_restack` scope `"upstack"`.

#### `gs_fold`
Squash-merge the current branch into its parent. Children are reparented. Branch is deleted unless `keep=true`.
- **Parameters**: `keep` (bool, default: false)
- **Returns**: `{folded, into, kept, reparented_children[]}`
- **When to use**: Collapsing completed work into the parent. **Destructive** — individual commit history is lost. Follow with `gs_restack` scope `"upstack"` on the parent to rebase reparented children.

### Sync tools (post-GitHub integration)

#### `gs_sync`
Fetch remote, sync trunk, restack.
- **Parameters**: `force` (bool, optional), `restack` (bool, optional)

#### `gs_submit`
Create/update PRs for branches.
- **Parameters**: `branch` (string, optional), `scope` (enum: current/stack)

## Common workflow patterns

These patterns show how tools chain together for typical operations. AI agents should follow these sequences.

### Create a new stack

```
gs_checkout branch="main"
# stage changes
gs_create name="feat/auth" commit_message="add auth middleware"
# stage more changes
gs_create name="feat/auth-tests" commit_message="add auth tests"
gs_status  # verify the stack
```

### Modify a branch mid-stack

```
gs_checkout branch="feat/auth"
# make changes
gs_modify stage_all=true message="update auth middleware"
gs_restack scope="upstack" branch="feat/auth"  # propagate to descendants
```

### Clean up after a branch is merged upstream

```
gs_status  # identify which branch was merged
gs_delete branch="feat/auth"  # reparents children to its parent
gs_restack scope="all"  # rebase everything onto updated trunk
```

### Reorganize the stack

```
gs_status  # understand current layout
gs_move branch="feat/tests" onto="main"  # move branch to new parent
gs_restack scope="upstack" branch="feat/tests"  # rebase descendants
```

### Resolve a restack conflict

```
gs_restack  # → returns conflict info
# resolve conflicts: edit files, git add, git rebase --continue
gs_restack  # retry — skips already-restacked branches
```

### Fold completed work into parent

```
gs_checkout branch="feat/auth-wip"
gs_diff  # review changes
gs_fold  # squash into parent, reparent children
gs_restack scope="upstack"  # rebase reparented children
```
