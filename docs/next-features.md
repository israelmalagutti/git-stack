# Next Features: Submit, Land, Repair, and Beyond

Design decisions and implementation guidance for the features that build on top of
the ref-based metadata sync shipped in v0.5.0. This document is the single source
of truth for what to build next, why, and in what order.

**Prerequisite reading**: [Branch Metadata Sync](./branch-metadata-sync.md),
[Implementation Plan](./branch-metadata-sync-plan.md),
[GitHub Integration](../ai-context/github-integration.md)

---

## Table of Contents

1. [What We Have Today](#1-what-we-have-today)
2. [`gs submit` — Stacked PR Creation](#2-gs-submit--stacked-pr-creation)
3. [`gs land` — Branch Landing and Cleanup](#3-gs-land--branch-landing-and-cleanup)
4. [`gs repair` — Stack Health Validation](#4-gs-repair--stack-health-validation)
5. [PR Metadata in Refs](#5-pr-metadata-in-refs)
6. [Provider Abstraction Layer](#6-provider-abstraction-layer)
7. [Merge Queue Integration](#7-merge-queue-integration)
8. [`--force-with-lease` for Ref Pushes](#8---force-with-lease-for-ref-pushes)
9. [Implementation Order and Dependencies](#9-implementation-order-and-dependencies)
10. [Open Questions](#10-open-questions)

---

## 1. What We Have Today

### Shipped (Phases 1–5)

```
refs/gs/
├── config                     → {"version":"1.0.0","trunk":"main"}
└── meta/
    ├── feat--auth             → {"parent":"main","created":"..."}
    ├── feat--auth-ui          → {"parent":"feat/auth","created":"..."}
    └── feat--logging          → {"parent":"main","created":"..."}
```

- **Local ref storage**: every metadata mutation writes to `refs/gs/meta/*`
- **Remote sync**: every CLI command and MCP tool pushes/deletes refs on origin
- **Newcomer bootstrap**: `gs init` fetches remote refs and imports metadata
- **Dual-write**: JSON file kept as fallback for backward compatibility
- **Fetch refspec**: `+refs/gs/*:refs/gs/*` configured automatically by `gs init`

### What's Missing

| Feature | Why it matters | Depends on |
|---------|---------------|------------|
| `gs submit` | Teams need PRs with correct base branches | Provider layer |
| `gs land` | Landing a branch requires reparenting, ref cleanup, PR base updates | `gs submit` (needs PR numbers) |
| `gs repair` | No recovery when metadata diverges from reality | Nothing (independent) |
| PR metadata in refs | `gs log` can't show PR numbers or status | `gs submit` |
| Provider abstraction | GitHub-only is a blocker for GitLab/Bitbucket teams | Nothing (foundation) |
| Merge queues | Stacked PRs need dependency-aware landing | `gs land` + PR metadata |
| `--force-with-lease` | Concurrent pushes can silently overwrite | Nothing (improvement) |

---

## 2. `gs submit` — Stacked PR Creation

### The Problem

When a developer pushes a stack of branches, they must manually create PRs and set
the correct base branch for each one. If they get the base wrong, reviewers see the
entire stack's diff instead of just the branch's changes.

```
Alice's stack:                   PRs she needs to create:
main                             (none)
├── feat/auth                    PR #42: base=main
│   └── feat/auth-ui             PR #43: base=feat/auth  ← easy to get wrong
└── feat/logging                 PR #44: base=main
```

### The Solution

`gs submit` reads the parent from metadata and creates/updates PRs with the correct
base branch automatically.

### Command Design

```bash
gs submit                    # submit current branch as PR
gs submit --draft            # submit as draft
gs submit --stack            # submit all branches in stack (bottom-up)
gs submit -m "PR title"      # custom title (default: branch name)
```

### Flow

```
gs submit
├── Detect provider from remote URL (github.com → GitHub)
├── Check CLI is available (`gh auth status`)
├── Push current branch to remote
├── Check if PR already exists for this branch
│   ├── Yes → update PR (push was enough, optionally update title/body)
│   └── No  → create PR with base=parent from metadata
├── Store PR number in branch metadata ref
├── Push updated ref to remote
└── Print PR URL
```

### `gs stack submit`

Walks the stack bottom-up, submitting each branch in order:

```
gs stack submit
├── Resolve all branches from trunk to current (topological order)
├── For each branch (bottom-up):
│   ├── Push branch
│   ├── Create/update PR with base=parent
│   ├── Store PR number in metadata
│   └── Push ref
└── Print summary with all PR URLs
```

### MCP Tool: `gs_submit`

**Required.** AI agents need to create PRs programmatically as part of workflow
automation. The tool should:

- Accept `branch` (optional, defaults to current) and `draft` (boolean) parameters
- Return `{branch, pr_number, pr_url, action: "created"|"updated"}`
- Never prompt — return error if provider CLI is missing or unauthenticated
- Push refs after storing PR number

### Edge Cases

- **No `gh` CLI**: Return clear error with install instructions
- **Not authenticated**: Return error suggesting `gh auth login`
- **Non-GitHub remote**: Return error, suggest manual PR creation
- **PR already exists**: Update (force-push was the intent), don't create duplicate
- **Branch not pushed**: Auto-push before creating PR (default behavior)
- **Parent branch not pushed**: Push parent first, then create PR

---

## 3. `gs land` — Branch Landing and Cleanup

### The Problem

When a PR is merged (on GitHub, by clicking "Merge"), the developer must manually:
1. Delete the local branch
2. Delete the remote branch
3. Reparent children to the landed branch's parent
4. Update children's PR base branches on GitHub
5. Delete the metadata ref locally and remotely
6. Restack all children

This is error-prone and tedious, especially for mid-stack branches.

### The Solution

`gs land` automates the entire landing flow.

### Command Design

```bash
gs land                      # land current branch (must be merged)
gs land feat/auth            # land specific branch
gs land --stack              # land all merged branches in stack
gs land --no-delete-remote   # keep remote branch
```

### Flow

```
gs land feat/auth
├── Verify PR #42 is merged (via `gh pr view`)
│   └── If not merged → error "PR is not merged yet"
├── Reparent children:
│   └── feat/auth-ui.parent = main (was feat/auth)
├── Update children's PR base branches:
│   └── `gh pr edit 43 --base main`
├── Delete local branch: git branch -D feat/auth
├── Delete remote branch: git push origin --delete feat/auth
├── Delete metadata ref: refs/gs/meta/feat--auth (local + remote)
├── Update children refs: push refs/gs/meta/feat--auth-ui
├── Restack children onto new parent
└── Return to original branch (or parent if we were on the landed branch)
```

### `gs land --stack`

Detects all branches whose PRs have been merged and lands them in dependency order:

```
gs land --stack
├── For each tracked branch with a PR number:
│   ├── Check if PR is merged
│   └── Collect merged branches
├── Sort by depth (deepest first to avoid reparenting conflicts)
├── Land each in order
└── Print summary
```

### Integration with `gs sync`

`gs sync` should detect externally-merged PRs during its fetch phase:

```
gs sync
├── Fetch from remote (branches + refs)
├── For each tracked branch with PR metadata:
│   ├── Check if PR is merged
│   └── If merged → prompt: "PR #42 for feat/auth was merged. Land it? [y/n]"
├── Land confirmed branches
├── Restack remaining branches
└── Push updated refs
```

### MCP Tool: `gs_land`

**Required.** AI agents need to clean up after merges. The tool should:

- Accept `branch` (optional) and `stack` (boolean) parameters
- Return `{landed, reparented_children[], restacked[]}`
- Verify PR is merged before proceeding (safety check)
- Never delete branches without PR merge confirmation

---

## 4. `gs repair` — Stack Health Validation

### The Problem

Metadata can diverge from reality in several ways:
- A branch was deleted outside of gs (e.g., `git branch -D`)
- A ref was corrupted or manually edited
- Remote refs and local refs disagree
- A branch's parent ref points to a branch that no longer exists
- Orphaned refs exist for branches that were already untracked

There is no tool to detect or fix these issues.

### The Solution

`gs repair` scans all metadata, validates it against the actual git state, and
offers to fix inconsistencies.

### Command Design

```bash
gs repair                    # scan and offer fixes interactively
gs repair --dry-run          # report issues without fixing
gs repair --force            # fix all issues without prompting
```

### Checks

| Check | Detection | Fix |
|-------|-----------|-----|
| Orphaned ref | Ref exists but branch deleted | Delete ref |
| Missing parent | Branch tracked but parent doesn't exist | Reparent to trunk |
| Stale remote refs | Remote ref exists for deleted branch | Delete remote ref |
| Ref/JSON mismatch | Refs and JSON disagree | Prefer refs, update JSON |
| Circular parent chain | Branch A → B → A | Break cycle, reparent to trunk |
| Untracked with ref | Ref exists but branch not in metadata | Offer to import or delete |

### Flow

```
gs repair
├── Load all local refs (refs/gs/meta/*)
├── Load all local branches (git branch)
├── Load JSON metadata
├── For each ref:
│   ├── Does the branch exist? If not → "Orphaned ref for 'feat/old'"
│   ├── Does the parent exist? If not → "Missing parent 'feat/gone' for 'feat/child'"
│   └── Does ref match JSON? If not → "Ref/JSON mismatch for 'feat/auth'"
├── Check for circular parent chains
├── Report findings
├── For each issue, prompt for fix (unless --dry-run or --force)
└── Push updated refs if any fixes were applied
```

### MCP Tool: `gs_repair`

**Safe to expose.** The tool should:

- Default to `--dry-run` mode (report only, no mutations)
- Accept `fix` boolean parameter to actually apply fixes
- Return `{issues_found[], issues_fixed[], remaining[]}`
- Never delete branches — only refs and metadata entries

---

## 5. PR Metadata in Refs

### Extended BranchMetadata

```json
{
  "parent": "main",
  "created": "2026-03-27T14:30:00Z",
  "parentRevision": "abc123...",
  "pr": {
    "number": 42,
    "provider": "github"
  }
}
```

The `pr` field is optional. Set by `gs submit`, consumed by:
- **`gs log`** — shows `#42` next to branch name
- **`gs land`** — checks if PR is merged before cleaning up
- **`gs sync`** — detects externally-merged PRs

### Go Changes

```go
// internal/config/metadata.go
type BranchMetadata struct {
    Parent         string    `json:"parent"`
    Tracked        bool      `json:"tracked"`
    Created        time.Time `json:"created"`
    ParentRevision string    `json:"parentRevision,omitempty"`
    PR             *PRInfo   `json:"pr,omitempty"`
}

type PRInfo struct {
    Number   int    `json:"number"`
    Provider string `json:"provider"` // "github", "gitlab"
}
```

### `gs log` with PR Status

```
  ○ feat/auth-ui          #43 ⏳ Review pending
  │
──┤
  │
  ○ feat/auth             #42 ✓ Approved
  │
──┤
  │
  ● main
```

PR status is fetched on-demand from the provider CLI. If offline or provider
unavailable, only the PR number is shown (no status icon). Add `--no-pr` flag
to skip status fetching for speed.

### Why Not Store Full PR Status in Refs

PR status (reviews, CI, merge state) changes frequently and is authoritative on
the provider. Storing it in refs would create a stale cache. Instead, fetch on-demand
via the provider CLI and optionally cache in memory for the duration of a single command.

---

## 6. Provider Abstraction Layer

### Design

```go
// internal/provider/provider.go
type Provider interface {
    Name() string                                   // "github", "gitlab"
    CreatePR(opts PRCreateOpts) (*PRResult, error)
    UpdatePR(number int, opts PRUpdateOpts) error
    GetPRStatus(number int) (*PRStatus, error)
    MergePR(number int, opts PRMergeOpts) error
    UpdatePRBase(number int, newBase string) error
    CLIAvailable() bool
    CLIAuthenticated() bool
}

// internal/provider/detect.go
func DetectFromRemoteURL(url string) (Provider, error)
// → "github.com" → GitHubProvider
// → "gitlab.com" → GitLabProvider
// → other        → GenericProvider (no PR features)
```

### GitHub Provider (Phase 6)

Uses `gh` CLI for all operations:

```go
// internal/provider/github.go
type GitHubProvider struct{}

func (g *GitHubProvider) CreatePR(opts PRCreateOpts) (*PRResult, error) {
    args := []string{"pr", "create", "--base", opts.Base, "--title", opts.Title}
    if opts.Draft { args = append(args, "--draft") }
    output, err := exec.Command("gh", args...).Output()
    // parse PR number from output
}
```

### GitLab Provider (Future)

Uses `glab` CLI. Same interface, different implementation.

### Generic Provider (Fallback)

No PR features. Stack structure still syncs via refs. Users create PRs manually.
`gs submit` returns a clear message: "Provider not supported for PR creation.
Push completed — create the PR manually with base branch: main"

### Why `gh`/`glab` CLI Instead of Direct API

- No auth implementation needed (CLI handles OAuth)
- Battle-tested, handles rate limiting, pagination, retries
- Users likely already have it installed and authenticated
- Simpler implementation, less code to maintain
- Trade-off: ~100ms slower per call (process spawn) — acceptable

---

## 7. Merge Queue Integration

### The Problem

GitHub merge queues serialize PR merges, running CI on each before committing.
Stacked PRs need dependency-aware enqueuing: child PRs should only enter the queue
after their parent lands.

### Design

```bash
gs land --queue              # enqueue current branch's PR
gs land --queue --stack      # enqueue all PRs in dependency order
```

### Flow

```
gs land --queue feat/auth
├── Verify PR #42 exists and is approved
├── Enable auto-merge on PR #42 (`gh pr merge --auto`)
├── Monitor: when #42 merges →
│   ├── Reparent feat/auth-ui to main
│   ├── Update PR #43 base to main
│   ├── Restack feat/auth-ui
│   └── Optionally enqueue #43
└── Clean up feat/auth refs
```

### Provider Support

| Provider | Merge Queue | gs Integration |
|----------|-------------|----------------|
| GitHub | Native (branch protection) | `gh pr merge --auto`, monitor queue |
| GitLab | Merge trains | `glab mr merge --when-pipeline-succeeds` |
| Others | Manual | gs handles reparenting; user merges manually |

### Implementation Note

This is Phase 7 — the most complex feature. It depends on `gs land` (Phase 7.1)
and PR metadata (Phase 6) being stable. Consider shipping `gs land` without
`--queue` first, then adding queue support as a follow-up.

---

## 8. `--force-with-lease` for Ref Pushes

### Current State

`PushRefs` in `internal/git/refs.go` uses plain `git push`. If two team members
push the same branch's metadata ref simultaneously, last write wins silently.

### The Risk

In practice, metadata conflicts are rare — metadata only changes on track, reparent,
delete, or land. Two developers rarely modify the same branch's parent simultaneously.
But as team adoption grows, this becomes a real risk.

### Proposed Fix

```go
func (r *Repo) PushRefs(remote string, refspecs ...string) error {
    args := []string{"push", "--force-with-lease", remote}
    args = append(args, refspecs...)
    _, err := r.RunGitCommand(args...)
    if err != nil {
        return fmt.Errorf("failed to push refs to %s: %w", remote, err)
    }
    return nil
}
```

If push fails due to non-fast-forward (someone else updated the ref), the caller
should fetch, re-read metadata, and retry. This requires a retry loop in the
push helpers.

### When to Implement

Not urgent. Ship as part of Phase 6 or as a standalone improvement before heavy
team adoption. The current last-write-wins behavior is acceptable for small teams
(2-5 developers).

---

## 9. Implementation Order and Dependencies

```
Independent:
  gs repair ─────────────────────────────────── can ship anytime
  --force-with-lease ────────────────────────── can ship anytime

Sequential:
  Phase 6.1: Provider abstraction layer
       │
       ▼
  Phase 6.2: gs submit (single branch)
       │
       ├──► Phase 6.3: PR metadata in refs
       │         │
       │         ▼
       │    Phase 6.4: gs log PR status display
       │
       ▼
  Phase 6.5: gs stack submit
       │
       ▼
  Phase 7.1: gs land (basic)
       │
       ├──► Phase 7.2: gs sync detects merged PRs
       │
       ▼
  Phase 7.3: Merge queue integration (--queue)
       │
       ▼
  Phase 8: JSON cleanup (remove dual-write)
```

### Recommended Build Order

1. **`gs repair`** — independent, immediately useful, low risk
2. **Provider abstraction** — foundation for submit/land
3. **`gs submit`** — highest user demand
4. **PR metadata in refs** — unlocks log display and land
5. **`gs land`** — completes the core team workflow
6. **`--force-with-lease`** — safety improvement before wider adoption
7. **Merge queue integration** — advanced, lower priority
8. **JSON cleanup** — only after everything is stable

### Effort Estimates

| Feature | Complexity | Files Changed | New Files |
|---------|-----------|--------------|-----------|
| `gs repair` | Low-Medium | cmd/repair.go, mcptools/write.go | 1 cmd, 1 tool handler |
| Provider layer | Medium | — | internal/provider/*.go (3-4 files) |
| `gs submit` | Medium | cmd/submit.go, mcptools/write.go, config/metadata.go | 1 cmd, 1 tool |
| PR metadata | Low | config/metadata.go, stack/visualize.go | — |
| `gs land` | Medium-High | cmd/land.go, cmd/sync.go, mcptools/write.go | 1 cmd, 1 tool |
| `--force-with-lease` | Low | internal/git/refs.go, cmd/metadata_loader.go | — |
| Merge queues | High | cmd/land.go, provider/github.go | — |
| JSON cleanup | Low | config/*.go, cmd/*.go | — |

---

## 10. Open Questions

These are decisions that should be resolved before or during implementation.

### `gs submit`

1. **Should `gs submit` auto-push the branch?**
   Current thinking: Yes, with `--no-push` to opt out.

2. **What if the parent branch isn't pushed yet?**
   Option A: Error and tell user to push parent first.
   Option B: Auto-push parent (cascading push up the stack).
   Leaning toward B for UX, but need to be careful about force-pushing parents
   that aren't ready.

3. **PR title default**: branch name or first commit message?
   Graphite uses first commit. GitHub defaults to branch name for single-commit PRs.
   Suggest: first commit message if single commit, branch name otherwise.

### `gs land`

4. **Should `gs land` work without a PR?**
   Some teams merge without PRs (direct push to main). Should `gs land` support
   "the branch was merged into trunk" without a PR number?
   Leaning toward yes — check if branch is merged via `git merge-base`.

5. **What if children have open PRs?**
   Landing a parent changes children's base branch. Should we auto-update the PR
   base via `gh pr edit --base`? Yes — this is the whole point.

6. **Remote branch deletion**: default on or off?
   Graphite deletes remote branches by default. GitHub can be configured to
   auto-delete on merge. Suggest: don't delete remote (GitHub already handles it).

### `gs repair`

7. **Should repair auto-run during `gs sync`?**
   Lightweight validation (orphaned refs, missing parents) could run during sync.
   Full repair (with prompts) stays as standalone command.

### Provider Layer

8. **How to handle multi-remote repos?**
   Current code hardcodes "origin". Some repos use "upstream" for the main remote.
   Phase 6 should add `gs config set remote <name>` or auto-detect from push config.

9. **Should we support GitHub Enterprise?**
   Yes — `gh` CLI handles GHE transparently via `gh auth login --hostname`.
   Provider detection just needs to not hardcode `github.com`.

---

**Created**: 2026-03-27
**Status**: Design document — implementation not yet started
**Related**: [Branch Metadata Sync](./branch-metadata-sync.md),
[Implementation Plan](./branch-metadata-sync-plan.md),
[GitHub Integration](../ai-context/github-integration.md),
[Roadmap](../ai-context/roadmap.md)
