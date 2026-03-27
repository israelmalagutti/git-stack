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

### Mutation tools

#### `gs_create`
Create a new stacked branch.
- **Parameters**: `name` (string, required), `commit_message` (string, optional)

#### `gs_restack`
Rebase branch and descendants onto parent.
- **Parameters**: `branch` (string, optional), `scope` (enum: only/upstack/downstack/all)

#### `gs_modify`
Amend current branch and restack children.
- **Parameters**: `message` (string, optional)

#### `gs_delete`
Delete a branch, reparenting children.
- **Parameters**: `branch` (string, required), `force` (bool, optional)

#### `gs_move`
Move a branch to a new parent.
- **Parameters**: `branch` (string, optional), `onto` (string, required)

#### `gs_fold`
Fold current branch into its parent.
- **Parameters**: `keep` (bool, optional)

#### `gs_rename`
Rename the current branch.
- **Parameters**: `new_name` (string, required)

#### `gs_track`
Start tracking an existing branch.
- **Parameters**: `branch` (string, required), `parent` (string, required)

#### `gs_untrack`
Stop tracking a branch.
- **Parameters**: `branch` (string, required)

### Sync tools (post-GitHub integration)

#### `gs_sync`
Fetch remote, sync trunk, restack.
- **Parameters**: `force` (bool, optional), `restack` (bool, optional)

#### `gs_submit`
Create/update PRs for branches.
- **Parameters**: `branch` (string, optional), `scope` (enum: current/stack)
