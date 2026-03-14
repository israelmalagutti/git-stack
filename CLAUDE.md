# CLAUDE.md

## Project Overview

`gs` (git-stack) is a CLI tool for managing stacked branches in git, inspired by Graphite's `gt`.

## Key Documentation

- `docs/visualization-decisions.md` — Every major design decision and rationale behind `gs log` visualization. Read this before modifying the tree rendering code.

## Architecture

- `cmd/` — Cobra command definitions (one file per command)
- `internal/stack/` — Stack tree model (`stack.go`) and visualization (`visualize.go`)
- `internal/colors/` — ANSI color system and terminal output helpers
- `internal/config/` — Config and metadata persistence
- `internal/git/` — Git repository operations wrapper

## Build & Test

- Language: Go
- Build: `go build -o gs .`
- Test: `go test ./...`
- Lint: handled by CI (golangci-lint)

## Visualization Invariants

- Every branch-out branch must be able to branch out on its own, in its own column. The column-based layout guarantees that any branch — regardless of depth — can have children that fork into new columns to its right. No branch is "locked" into a terminal position; the tree supports arbitrary nesting.
- Oldest child inherits parent's column; newer children get new columns to the right.
- Trunk is always column 0 and rendered separately at the bottom.
