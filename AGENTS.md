# Agent Instructions

Instructions for AI agents working on this repository.

## Key Documentation

- `CLAUDE.md` — Project overview, architecture, build commands, and invariants
- `docs/mcp.md` — MCP server design, tool reference, and architecture. Read before working on `cmd/mcp.go` or `internal/mcptools/`
- `ai-context/mcp-server.md` — Full MCP implementation plan with phases and design decisions
- `docs/visualization-decisions.md` — Visualization design rationale. Read before modifying tree rendering
- `docs/branch-metadata-sync.md` — Ref-backed metadata sync design: storage format, sync protocol, team workflows. Read before modifying `internal/config/ref_metadata.go` or `internal/git/refs.go`

## MCP Server (`gs mcp`)

The MCP server is the machine interface for AI agents to interact with git-stack. It exposes the same stack operations as the CLI but returns structured JSON instead of terminal output.

- Entry point: `cmd/mcp.go`
- Tool handlers: `internal/mcptools/`
- Protocol: stdio JSON-RPC 2.0 (MCP standard)
- Tools never use interactive prompts — they return options for the agent to choose
- All stdout is reserved for MCP messages; logging goes to stderr
- Each tool call loads fresh repo/config/metadata state (no caching)

## Conventions

- Go, built with `go build -o gs .`, tested with `go test ./...`
- One Cobra command per file in `cmd/`
- Internal packages under `internal/` are not importable externally
- Metadata dual-write: local JSON (`.git/.gs_stack_metadata`) + git refs (`refs/gs/meta/*`). Reads try refs first, fall back to JSON. See `docs/branch-metadata-sync.md`
- Commits follow conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`)
