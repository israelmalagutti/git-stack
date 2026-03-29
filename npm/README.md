# @gitstack/cli

npm wrapper for [gs](https://github.com/israelmalagutti/git-stack) — a fast CLI tool for managing stacked branches in git.

## Usage

```bash
# Run without installing
npx @gitstack/cli

# Or install globally
npm install -g @gitstack/cli
gs log
```

## MCP Server for Claude Code

Add the git-stack MCP server to Claude Code with zero install:

```bash
claude mcp add gs -- npx @gitstack/cli mcp
```

## How it works

This package is a thin wrapper that downloads the pre-built `gs` Go binary from [GitHub Releases](https://github.com/israelmalagutti/git-stack/releases) for your platform. The binary is cached after the first download.

Supported platforms: Linux, macOS, Windows (x64 and arm64).

## Documentation

See the [git-stack repository](https://github.com/israelmalagutti/git-stack) for full documentation.

## License

MIT
