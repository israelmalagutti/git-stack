# CodeQL CI for git-stack: Why and How

## Why CodeQL Matters

CodeQL is GitHub's semantic code analysis engine. Unlike linters (which check style/patterns), CodeQL understands data flow — it can trace how user input moves through your program and flag real vulnerabilities.

### What it catches that golangci-lint doesn't

| Category | Example | golangci-lint | CodeQL |
|---|---|---|---|
| Command injection | Unsanitized input passed to `exec.Command` | No | Yes |
| Path traversal | User-controlled paths escaping intended directories | No | Yes |
| Hardcoded credentials | Secrets embedded in source code | Partial | Yes |
| Tainted data flow | Tracing untrusted input across function boundaries | No | Yes |
| SQL/NoSQL injection | Unsanitized queries (if applicable) | No | Yes |

### Why it matters for git-stack specifically

- `gs` shells out to `git` via `exec.Command` — CodeQL can verify that branch names, ref paths, and user inputs are properly sanitized before reaching the shell.
- The MCP server (`gs mcp`) accepts external tool calls over stdio — CodeQL can trace whether MCP inputs flow unsafely into git operations.
- As a CLI tool that manipulates the filesystem and git refs, path traversal and injection are the primary threat vectors.

### Cost

CodeQL is **free for public repositories** on GitHub. For private repos, it's included in GitHub Advanced Security (paid).

## How to Set It Up

### 1. Add the workflow file

Create `.github/workflows/codeql.yml`:

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    # Run weekly on Mondays at 06:00 UTC to catch new vulnerability patterns
    - cron: '0 6 * * 1'

jobs:
  analyze:
    name: Analyze
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      contents: read

    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: 'go.mod'

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: go
          # Use the extended query suite for more thorough analysis
          queries: security-extended

      - name: Build
        run: make build

      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
```

### 2. Key configuration choices

- **`security-extended`** vs `security-and-quality`: The extended suite focuses on security findings with low false-positive rates. The `security-and-quality` suite adds code quality checks but produces more noise. Start with `security-extended`.
- **Scheduled runs**: The weekly cron catches newly published vulnerability patterns even when no code changes. This is important because CodeQL's query database is updated regularly.
- **Build step**: CodeQL for Go needs to observe the build to understand the code. The `make build` step ensures it analyzes the actual compiled code, not just source files.

### 3. What to expect

- **First run**: May take 3-5 minutes. Subsequent runs use caching and are faster.
- **Results**: Appear in the repository's Security tab > Code scanning alerts.
- **PR checks**: CodeQL runs on every PR and blocks merge if new alerts are introduced (configurable).
- **False positives**: Can be dismissed directly in the Security tab with a reason (won't reappear).

### 4. Optional: Custom queries

If you want to add project-specific checks (e.g., "ensure all `exec.Command` calls go through our `internal/git` wrapper"), you can write custom CodeQL queries in a `.github/codeql/` directory. This is advanced and not needed initially.

### 5. Branch protection (recommended)

After enabling CodeQL, add it as a required status check in your branch protection rules for `main`:

Settings > Branches > Branch protection rules > Require status checks > Add "CodeQL"

This prevents merging PRs that introduce new security findings.

## TL;DR

1. Add the workflow file above
2. Merge to main
3. Check Security tab for initial findings
4. Add CodeQL as a required status check
5. Done — ongoing security scanning with zero maintenance
