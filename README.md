# gs - Git Stack

A fast, simple git stack management CLI tool for working with stacked diffs (stacked PRs).

## Features

- **Stack Management** - Create and manage parent-child relationships between branches
- **Smart Navigation** - Move up/down the stack, jump to top/bottom
- **Rebase Operations** - Automatic restacking when parent branches change
- **Interactive UI** - Prompts for branch selection, conflict resolution guidance
- **Platform Agnostic** - Works with any git hosting (GitHub, GitLab, etc.)

## Installation

### From Release (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/israelmalagutti/git-stack/main/scripts/install.sh | bash
```

Or specify a version:

```bash
GS_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/israelmalagutti/git-stack/main/scripts/install.sh | bash
```

### From Source

```bash
git clone https://github.com/israelmalagutti/git-stack.git
cd git-stack
make install
```

### Manual Download

Download the appropriate binary from the [releases page](https://github.com/israelmalagutti/git-stack/releases) and add it to your PATH.

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
| `gs sync` | | Sync metadata with git branches |

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

# Build for all platforms
make build-all

# Create release archives with checksums
make release
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary with version injection |
| `make build-all` | Cross-platform builds (Linux/macOS/Windows) |
| `make release` | Build all + create archives + checksums |
| `make install` | Build and install to /usr/local/bin |
| `make uninstall` | Remove from /usr/local/bin |
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

## Configuration

gs stores configuration in `.gs/config.json` at the repository root:

```json
{
  "trunk": "main",
  "version": "1.0.0"
}
```

Branch metadata is stored in `.gs/metadata.json`:

```json
{
  "branches": {
    "feat-auth": {
      "parent": "main"
    },
    "feat-auth-ui": {
      "parent": "feat-auth"
    }
  }
}
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
