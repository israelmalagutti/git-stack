# Agent Instructions

Instructions for AI agents working on this repository.

## Development Process

Before implementing any new feature:

- **MCP tool evaluation**: Determine whether the feature needs an MCP tool in `internal/mcptools/`. If it does, implement it alongside the CLI command — not as a follow-up. Ensure the tool is safe: no destructive side effects without explicit parameters, returns structured JSON, never uses interactive prompts, and pushes refs to remote after any metadata mutation. Flag any proposed tool that could cause data loss without confirmation.
- **Test coverage**: Target at least 90% test coverage for new code. Write integration tests with real git repos (temp dirs + bare remotes) rather than mocks. When adding a new command, verify both the CLI path and the MCP tool path are tested.

## Key Documentation

- `CLAUDE.md` — Project overview, architecture, build commands, and invariants
- `docs/mcp.md` — MCP server design, tool reference, and architecture. Read before working on `cmd/mcp.go` or `internal/mcptools/`
- `ai-context/mcp-server.md` — Full MCP implementation plan with phases and design decisions
- `docs/visualization-decisions.md` — Visualization design rationale. Read before modifying tree rendering
- `docs/branch-metadata-sync.md` — Ref-backed metadata sync design: storage format, sync protocol, team workflows. Read before modifying `internal/config/ref_metadata.go` or `internal/git/refs.go`
- `docs/next-features.md` — Design decisions for upcoming features: `gs submit`, `gs land`, `gs repair`, PR metadata, provider abstraction, merge queues. Read before implementing any new command

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
