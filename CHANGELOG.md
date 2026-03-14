# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `gs split` command with three modes: by-commit, by-hunk, by-file
- `gs up` / `gs down` commands for stack navigation
- `gs top` / `gs bottom` commands to jump to stack ends
- `gs move` command with `--source` and `--target` flags
- `gs fold` command to fold branch into parent
- `gs delete` command to delete branches from stack
- `gs modify` command for amending commits
- `gs stack restack` command for rebasing stacks
- `gs sync` command with cycle detection
- Informative restack messaging with progress indicators
- Version injection from git tags
- Cross-platform builds (Linux, macOS, Windows)
- GitHub Actions CI/CD workflows
- Installation script for downloading releases

### Fixed
- Handle trunk branch properly in all commands
- Fix nil pointer in move command interactive mode
- Handle Ctrl+C cancellation gracefully

### Changed
- Silence usage/help output on errors (show only with `-h`)

## [0.1.0] - Initial Release

### Added
- `gs init` - Initialize gs in a repository
- `gs create` - Create stacked branches
- `gs track` - Track existing branches
- `gs checkout` - Smart branch switching with aliases
- `gs log` - Visualize stack structure
- `gs info` - Show branch details
- `gs parent` / `gs children` - Show relationships
- Configuration and metadata storage
- Interactive prompts with survey library
