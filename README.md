# gs - Git Stack

A fast, simple git stack management CLI tool for working with stacked diffs (stacked PRs).

## Features

- **Stack Management** - Create and manage parent-child relationships between branches
- **Smart Navigation** - Move up/down the stack, jump to top/bottom
- **Rebase Operations** - Automatic restacking when parent branches change
- **Interactive UI** - Prompts for branch selection, conflict resolution guidance
- **Platform Agnostic** - Works with any git hosting (GitHub, GitLab, etc.)
- **Metadata Sync** - Stack structure stored in git refs, shareable across developers and machines via push/fetch
- **MCP Server** - AI agent integration via Model Context Protocol (`gs mcp`)

## Installation

### npx (no install needed)

```bash
npx @gitstack/cli
```

### Homebrew (macOS / Linux)

```bash
brew install israelmalagutti/tap/git-stack
```

### Go

```bash
go install github.com/israelmalagutti/git-stack@latest
```

### Shell script

```bash
curl -fsSL https://raw.githubusercontent.com/israelmalagutti/git-stack/main/scripts/install.sh | bash
```

### From source

```bash
git clone https://github.com/israelmalagutti/git-stack.git
cd git-stack
make install
```

### MCP for Claude Code (zero-install)

```bash
claude mcp add gs -- npx @gitstack/cli mcp
```

## Quick Start

```bash
# Initialize gs in your repository
gs init

# Create a new stacked branch
gs create feat-auth

# Make changes and commit
git add . && git commit -m "Add authentication"

# Create another branch on top
gs create feat-auth-ui

# View the stack
gs log

# Navigate the stack
gs up          # Move to child branch
gs down        # Move to parent branch
gs top         # Jump to top of stack
gs bottom      # Jump to trunk

# Restack after parent changes
gs stack restack
```

## Commands

### Core Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `gs init` | | Initialize gs in a repository |
| `gs create <name>` | | Create a new stacked branch |
| `gs track [branch]` | | Track an existing branch |
| `gs checkout <branch>` | `co`, `switch` | Switch to a branch |
| `gs log` | | Visualize the stack structure |
| `gs info` | | Show current branch details |

### Navigation

| Command | Alias | Description |
|---------|-------|-------------|
| `gs up [n]` | `u` | Move up toward leaves |
| `gs down [n]` | `dn` | Move down toward trunk |
| `gs top` | `t` | Jump to top of stack |
| `gs bottom` | `b` | Jump to trunk |
| `gs parent` | | Show parent branch |
| `gs children` | | Show child branches |

### Stack Operations

| Command | Alias | Description |
|---------|-------|-------------|
| `gs stack restack` | | Rebase stack to maintain relationships |
| `gs modify` | `m` | Amend commit and restack children |
| `gs move [target]` | `mv` | Move branch to different parent |
| `gs fold` | | Fold current branch into parent |
| `gs delete [branch]` | `rm` | Delete branch from stack |
| `gs split` | | Split branch into multiple branches |
| `gs rename <name>` | | Rename the current branch |
| `gs sync` | | Fetch remote, clean stale branches, delete merged, and restack |
| `gs mcp` | | Start the MCP server for AI agent integration |

### Split Modes

```bash
gs split -c              # Split by selecting commits
gs split -u              # Interactive hunk selection
gs split -f "*.json"     # Split files matching pattern
gs split -n base         # Specify new branch name
```

## Development

### Prerequisites

- Go 1.21+
- Make

### Building

```bash
# Build for current platform
make build

# GoReleaser snapshot (all platforms, no publish)
make release-dry-run
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary with version injection |
| `make install` | Build and install to /usr/local/bin |
| `make uninstall` | Remove from /usr/local/bin |
| `make release-dry-run` | GoReleaser snapshot (all platforms, no publish) |
| `make clean` | Remove build artifacts |
| `make test` | Run tests |
| `make test-coverage` | Run tests with HTML coverage report |
| `make lint` | Run golangci-lint |
| `make version` | Show version info |

### Versioning

Version is injected at build time from git tags:

```bash
# Shows commit hash if no tag
gs --version
# gs version a1b2c3d

# After tagging
git tag v0.1.0
make build
gs --version
# gs version v0.1.0
```

### Creating a Release

1. Update `CHANGELOG.md`
2. Commit changes
3. Create and push a tag:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
4. GitHub Actions will automatically build and publish the release

## MCP Server

`gs mcp` starts a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, letting AI agents (Claude Code, Cursor, VS Code Copilot, etc.) manage stacks programmatically. See [docs/mcp.md](docs/mcp.md) for the full tool reference and architecture.

## Configuration

gs stores metadata in two locations (dual-write):

- **Git refs** (`refs/gs/meta/*`, `refs/gs/config`) — shareable via `git push`/`fetch`, syncs stack structure across developers and machines
- **Local JSON** (`.git/.gs_stack_metadata`) — fallback for offline/legacy use

On read, refs are tried first with automatic fallback to JSON. See [docs/branch-metadata-sync.md](docs/branch-metadata-sync.md) for the full design.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
