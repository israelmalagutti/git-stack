# Design: Global `--json`, `--debug`, and `--no-interactive` Flags

**Date:** 2026-04-08
**Status:** Approved

## Motivation

The `gs` codebase has two parallel codepaths: `cmd/` (human CLI) and `internal/mcptools/` (MCP JSON handlers). They duplicate business logic — identical `repoState` structs, near-identical `loadRepoState()` functions, and operations that drift apart over time. This also means bugs fixed in one path may not be fixed in the other.

The fix: make every command **JSON-first**. Each command produces a typed result struct. Human output is derived from that struct. MCP handlers become thin wrappers that call `gs <command> --json` and translate the output into MCP protocol.

This gives us one codepath for each operation. One way to read, one way to change, one way to delete.

Graphite's `gt` CLI has `--debug`, `--quiet`, and `--no-interactive` but no `--json` — making structured output a differentiator for `gs`.

## Global Flags

Three new persistent flags on the root command:

| Flag | Type | Default | Effect |
|------|------|---------|--------|
| `--json` | bool | false | stdout becomes a single JSON object per command |
| `--debug` | bool | false | timestamped debug lines written to stderr |
| `--no-interactive` | bool | false | skip all prompts, use sensible defaults or fail |

**Composition rules:**

- `--json` implies `--no-interactive` — never block on a prompt when caller expects JSON
- `--json` disables color output automatically
- `--debug` always writes to stderr, never pollutes stdout — works with or without `--json`
- All three are orthogonal and compose freely

## Command Result Architecture

Every command produces a **result struct** — the single source of truth for its output.

### Example result structs

```go
// gs status
type statusResult struct {
    Trunk         string       `json:"trunk"`
    CurrentBranch string       `json:"current_branch"`
    Branches      []branchInfo `json:"branches"`
    NeedsRestack  []string     `json:"needs_restack,omitempty"`
}

// gs checkout
type checkoutResult struct {
    PreviousBranch string `json:"previous_branch"`
    CurrentBranch  string `json:"current_branch"`
}

// gs delete
type deleteResult struct {
    Deleted    string   `json:"deleted"`
    Reparented []string `json:"reparented,omitempty"`
}

// gs sync
type syncResult struct {
    TrunkUpdated          bool     `json:"trunk_updated"`
    BranchesCleaned       []string `json:"branches_cleaned,omitempty"`
    BranchesRestacked     []string `json:"branches_restacked,omitempty"`
    BranchesFailedRestack []string `json:"branches_failed_restack,omitempty"`
}
```

### Command execution pattern

```go
func runStatus(cmd *cobra.Command, args []string) error {
    // 1. Business logic — produces result struct
    result, err := executeStatus()
    if err != nil {
        return err  // error formatting handled globally
    }

    // 2. Output — format decided at the boundary
    if jsonMode(cmd) {
        return printJSON(cmd, result)
    }
    return printStatusHuman(cmd, result)
}
```

### Error contract

When a command fails in `--json` mode, the root error handler emits:

```json
{"error": "branch 'feat/gone' not found", "command": "checkout"}
```

This is handled once in `root.go`, not per-command.

### Multi-step commands

Commands like `gs sync` that perform multiple operations accumulate results into their struct. In human mode, each step prints as it goes. In JSON mode, the struct prints once at the end.

## Debug Output

`--debug` writes timestamped lines to stderr via a lightweight logger. Independent of `--json`.

```
stderr:  2026-04-08T12:00:00Z [debug] loading metadata from refs
stderr:  2026-04-08T12:00:00Z [debug] restack: feat/auth is 2 commits behind main
stderr:  2026-04-08T12:00:00Z [debug] git: rebase --onto main abc123 feat/auth
stdout:  {"branches_restacked": ["feat/auth"], ...}
```

**What gets logged at debug level:**

- Git commands being executed (full `git <args>` invocation)
- Metadata load/save operations
- Rebase decisions (why a branch does/doesn't need restack)
- Branch resolution steps (parent lookup, SHA comparisons)

**Implementation:**

```go
func debugf(cmd *cobra.Command, format string, args ...interface{}) {
    if !debugMode(cmd) {
        return
    }
    fmt.Fprintf(os.Stderr, "%s [debug] %s\n",
        time.Now().Format(time.RFC3339),
        fmt.Sprintf(format, args...))
}
```

No external logging library. No log levels beyond on/off. Just stderr with timestamps.

## Non-Interactive Mode

`--no-interactive` (or implied by `--json`) changes how commands handle prompts:

| Prompt Type | Interactive (default) | Non-interactive |
|---|---|---|
| Branch selection (checkout, move) | Show picker | Require branch as argument, error if missing |
| Confirmation (delete, fold, land) | "Are you sure?" | Proceed without asking |
| Multi-select (create with untracked files) | Checkbox picker | Skip — don't stage anything |
| Text input (commit message) | Open editor / prompt | Require `-m` flag, error if missing |

**Key principle:** non-interactive never invents data. If it needs information the user didn't provide via flags/args, it errors:

```json
{"error": "branch argument required in non-interactive mode", "command": "checkout"}
```

## MCP Migration Path

With `--json` in place, MCP handlers migrate from duplicated business logic to thin protocol translation.

### Before (current)

```
MCP request → mcptools/handleStatus() → loadRepoState() → build stack → format JSON → MCP response
                                         ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                         duplicated from cmd/
```

### After

```
MCP request → mcptools/handleStatus() → exec("gs status --json") → parse JSON → MCP response
```

### Migration strategy

Each handler migrates independently — not a big bang:

1. **Read-only handlers first** — `gs_status`, `gs_log`, `gs_branch_info`, `gs_diff` — lowest risk, easiest to verify output matches
2. **Write handlers next** — `gs_checkout`, `gs_create`, `gs_delete`, etc. — verify mutations + JSON output
3. **Remove duplicated code** — delete `loadRepoState()` and `repoState` from mcptools/ once all handlers are migrated
4. **Shrink mcptools/** to: exec helper, JSON parsing, MCP protocol wrapping

### What stays in mcptools/

- MCP protocol registration (tool names, descriptions, parameter schemas)
- The exec-and-parse glue
- Any MCP-specific behavior (e.g., `gs_repair` dry-run default)

### What goes away

- Duplicated `repoState` struct and `loadRepoState()`
- Duplicated `pushMetadataRefs()`, `deleteRemoteMetadataRef()`
- All direct imports of `internal/git`, `internal/config`, `internal/stack` from mcptools

The JSON result structs become the **contract** between CLI and MCP.

## Rollout Order

Each step is independently shippable. A command without `--json` support yet ignores the flag and prints human output.

1. **Infrastructure** — add global flags to `root.go`, add `printJSON()` and `debugf()` helpers, add centralized JSON error handler
2. **Proof of concept** — `gs status` — read-only, has well-defined MCP JSON to match against
3. **Remaining read commands** — `gs log`, `gs diff`, `gs info`
4. **Write commands** — `gs checkout`, `gs create`, `gs delete`, etc. — each also gets `--no-interactive` fallback behavior
5. **MCP migration** — once a command has `--json`, migrate its MCP handler to shell-out
6. **Remove dead code** — delete duplicated `repoState`, `loadRepoState()`, and direct internal package imports from mcptools

## Backwards Compatibility

Zero breakage. All flags default to current behavior:

- No `--json` → exact same colored human output as today
- No `--debug` → no stderr debug lines
- No `--no-interactive` → prompts work as they do now
