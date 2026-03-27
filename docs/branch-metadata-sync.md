# Branch Metadata Synchronization

How gs shares stack structure across developers and machines, provider-agnostically,
so that any team member can run `gs log` and immediately understand the stack.

---

## Table of Contents

1. [The Problem](#1-the-problem)
2. [Design Goals](#2-design-goals)
3. [How It Works Today](#3-how-it-works-today)
4. [The Chosen Approach: Custom Git Refs](#4-the-chosen-approach-custom-git-refs)
5. [Ref Storage Format](#5-ref-storage-format)
6. [Synchronization Protocol](#6-synchronization-protocol)
7. [Team Workflow: The Full Picture](#7-team-workflow-the-full-picture)
8. [Provider Compatibility](#8-provider-compatibility)
9. [PR Metadata Layer](#9-pr-metadata-layer)
10. [Merge Queue Integration](#10-merge-queue-integration)
11. [Conflict Resolution](#11-conflict-resolution)
12. [Security Considerations](#12-security-considerations)
13. [Migration from Local-Only Metadata](#13-migration-from-local-only-metadata)
14. [Alternatives Considered](#14-alternatives-considered)
15. [Glossary](#15-glossary)

---

## 1. The Problem

gs stores stack metadata (which branch is parent of which) in `.git/.gs_stack_metadata`.
This file is local. It never leaves the machine. When Alice pushes her stack to a remote
and Bob clones, Bob sees the branches but has no idea how they relate to each other.

```
Alice's machine                          Bob's machine
──────────────                           ─────────────
main                                     main
├── feat/auth         ── git push ──►    feat/auth        (untracked)
│   └── feat/auth-ui                     feat/auth-ui     (untracked)
└── feat/logging                         feat/logging     (untracked)

.git/.gs_stack_metadata  ✓               .git/.gs_stack_metadata  (empty)
```

Bob must manually `gs track` each branch and guess the correct parent ordering.
If he gets it wrong, `gs log` shows a broken tree. `gs restack` does the wrong thing.

This problem blocks every team feature: `gs submit` (stacked PRs need correct base
branches), `gs land` (children need reparenting), merge queues (need to know the
full dependency chain), and basic `gs log` on a shared repo.

---

## 2. Design Goals

1. **Provider-agnostic.** Must work with GitHub, GitLab, Bitbucket, Gitea, self-hosted —
   anything that speaks git. No platform API required for core sync.

2. **Offline-first.** All local operations (`gs log`, `gs create`, `gs restack`) work
   without network. Sync is an explicit action.

3. **Newcomer-friendly.** A developer who clones a repo and runs `gs init` should be
   able to see the full stack with `gs log` after a single fetch, without needing to
   understand git internals.

4. **Old-timer-friendly.** The mechanism uses standard git primitives. Experienced
   developers can inspect, debug, and manually fix metadata with `git cat-file`,
   `git update-ref`, and `git for-each-ref`.

5. **No server.** No Graphite-style hosted backend. No accounts. No tokens beyond what
   git already has.

6. **Survives rebases and force-pushes.** Stack metadata is keyed by branch name, not
   commit SHA. Rewriting history does not orphan metadata.

7. **Minimal footprint.** One small JSON blob per tracked branch. No pollution of commit
   history, working tree, or branch names.

---

## 3. How It Works Today

### Current storage

```
.git/
├── .gs_config              # {"version":"1.0.0","trunk":"main","initialized":"..."}
├── .gs_stack_metadata      # {"branches":{"feat/auth":{"parent":"main",...},...}}
└── .gs_continue_state      # (temporary, only during conflict resolution)
```

### Current data model

```json
{
  "branches": {
    "feat/auth": {
      "parent": "main",
      "tracked": true,
      "created": "2026-03-27T14:30:00Z",
      "parentRevision": "abc123..."
    },
    "feat/auth-ui": {
      "parent": "feat/auth",
      "tracked": true,
      "created": "2026-03-27T14:35:00Z",
      "parentRevision": "def456..."
    }
  }
}
```

### Why this isn't enough

- **Not shareable.** `.git/` contents are never pushed or fetched.
- **Single file.** Every branch mutation rewrites the entire file. No granularity.
- **No merge strategy.** If two tools or worktrees write simultaneously, last write wins.

---

## 4. The Chosen Approach: Custom Git Refs

Store each branch's metadata as a small JSON blob in git's object database, pointed to
by a ref under the `refs/gs/` namespace.

### Why git refs

Git refs are the only mechanism that is:
- **Built into git** — no dependencies, works everywhere
- **Pushable and fetchable** — standard refspec transport
- **Independent of commit history** — survives rebases and force-pushes
- **Inspectable** — `git cat-file`, `git for-each-ref`, `git log refs/gs/...`
- **Granular** — one ref per branch, no single-file bottleneck
- **Provider-agnostic** — any git remote that accepts pushes will store custom refs

### How refs work as a key-value store

Git's object database can store arbitrary content. The combination of `hash-object`
and `update-ref` turns git into a key-value store:

```bash
# Write: store JSON as a blob, get back a SHA
SHA=$(echo '{"parent":"main","created":"2026-03-27T14:30:00Z"}' | git hash-object -w --stdin)

# Point a ref at that blob
git update-ref refs/gs/meta/feat/auth $SHA

# Read: retrieve the JSON
git cat-file -p refs/gs/meta/feat/auth
# → {"parent":"main","created":"2026-03-27T14:30:00Z"}

# List all metadata refs
git for-each-ref refs/gs/meta/ --format='%(refname:short) %(objectname:short)'
```

This is the same technique Graphite originally used with `refs/branch-metadata/` and
the same foundation Gerrit uses for `refs/meta/config` and `refs/changes/*`.

---

## 5. Ref Storage Format

### Namespace layout

```
refs/gs/
├── config                    # repo-level gs config (trunk, version)
└── meta/
    ├── feat/auth             # metadata for branch "feat/auth"
    ├── feat/auth-ui          # metadata for branch "feat/auth-ui"
    └── feat/logging          # metadata for branch "feat/logging"
```

### Per-branch metadata blob

```json
{
  "parent": "main",
  "created": "2026-03-27T14:30:00Z",
  "parentRevision": "abc123def456..."
}
```

Fields:
- **`parent`** — the branch this one stacks on top of. For direct children of trunk,
  this is the trunk branch name.
- **`created`** — timestamp of when the branch was first tracked. Used for ordering
  siblings (oldest child inherits parent's column in `gs log`).
- **`parentRevision`** — SHA of the parent branch tip at the time of last restack.
  Used for `git rebase --onto` precision. Optional — omitted if unknown.

### Config ref blob

```json
{
  "version": "1.0.0",
  "trunk": "main",
  "initialized": "2026-03-27T14:00:00Z"
}
```

Stored at `refs/gs/config`. Shared across the team so everyone agrees on trunk.

### Why one ref per branch (not one big blob)

- **Granular sync.** Push only the refs that changed, not the entire metadata file.
- **No merge conflicts.** Two developers can update different branches' metadata
  simultaneously without collision.
- **Git-native cleanup.** When a branch is deleted, delete its ref. No orphaned entries
  in a monolithic file.
- **Scalability.** Git packs refs efficiently. Thousands of small blobs pack better than
  one large JSON file that changes on every operation.

---

## 6. Synchronization Protocol

### Push (share your metadata)

```bash
# Push all gs metadata to origin
git push origin 'refs/gs/*:refs/gs/*'

# Or push only a specific branch's metadata
git push origin refs/gs/meta/feat/auth
```

`gs submit` and `gs sync --push` do this automatically.

### Fetch (get others' metadata)

```bash
# Fetch all gs metadata from origin
git fetch origin 'refs/gs/*:refs/gs/*'
```

`gs init` and `gs sync` configure the fetch refspec so future `git fetch` includes
gs metadata automatically:

```ini
# Added to .git/config by gs init / gs sync
[remote "origin"]
    fetch = +refs/gs/*:refs/gs/*
```

After this, a regular `git fetch` or `git pull` brings in metadata alongside branches.

### The newcomer experience

```bash
# Bob clones a repo where Alice has been using gs
git clone https://github.com/org/repo.git
cd repo

# Bob initializes gs — this fetches refs/gs/* and imports metadata
gs init
# → Detected existing stack metadata from remote.
# → Trunk: main (from remote config)
# → Imported 3 tracked branches.

gs log
# → Shows Alice's full stack tree, correctly structured
```

`gs init` detects the presence of `refs/gs/config` on the remote and uses it to
bootstrap. No manual tracking needed.

### The returning-developer experience

```bash
# Alice left work yesterday. Today she pulls.
git fetch
# → refs/gs/* are fetched alongside branches (refspec already configured)

gs log
# → Shows any new branches Bob added to the stack
```

---

## 7. Team Workflow: The Full Picture

### Scenario: Alice creates a stack, Bob joins it

```
1. Alice:  gs create feat/auth          (creates branch + ref)
2. Alice:  gs create feat/auth-ui       (stacks on feat/auth)
3. Alice:  gs submit --stack            (pushes branches + refs + creates PRs)

4. Bob:    git fetch                    (gets branches + refs/gs/*)
5. Bob:    gs log                       (shows full tree)
6. Bob:    gs checkout feat/auth-ui     (navigates the stack)
7. Bob:    gs create feat/auth-tests    (adds to the stack)
8. Bob:    gs submit                    (pushes his branch + its ref)

9. Alice:  git fetch                    (gets Bob's new branch + ref)
10. Alice: gs log                       (shows Bob's branch in the tree)
```

### Scenario: Mid-stack edit and restack

```
1. Alice:  gs checkout feat/auth
2. Alice:  gs modify                    (amends feat/auth, restacks children)
3. Alice:  git push --force-with-lease  (pushes rewritten branches)
4. Alice:  gs sync --push               (pushes updated refs/gs/*)

5. Bob:    git fetch
6. Bob:    gs log                       (sees updated parentRevision)
7. Bob:    gs restack                   (rebases his branches onto new base)
```

### Scenario: Landing a branch

```
1. Alice:  gs land feat/auth            (merged on GitHub)
           → deletes feat/auth locally and remotely
           → reparents feat/auth-ui to main
           → restacks feat/auth-ui
           → deletes refs/gs/meta/feat/auth
           → updates refs/gs/meta/feat/auth-ui (parent → main)
           → pushes ref changes

2. Bob:    git fetch
3. Bob:    gs log                       (feat/auth gone, feat/auth-ui now on main)
```

---

## 8. Provider Compatibility

### How custom refs behave on major providers

| Provider | Push `refs/gs/*` | Fetch `refs/gs/*` | Notes |
|----------|-----------------|-------------------|-------|
| GitHub | Yes | Yes | Refs exist but are hidden from UI. Fully functional via git protocol. |
| GitLab | Yes | Yes | Custom refs work. GitLab may display them under "Tags" or ignore them in UI. |
| Bitbucket | Yes | Yes | Custom ref namespaces are supported in Bitbucket Server and Cloud. |
| Gitea / Forgejo | Yes | Yes | Full git protocol support, no restrictions on ref namespaces. |
| Self-hosted git | Yes | Yes | Bare git repos accept any ref namespace. |
| Azure DevOps | Yes | Yes | Supports custom refs via git protocol. |

### Potential restrictions

Some providers may restrict ref namespaces via server-side hooks or policies:
- Corporate GitHub Enterprise instances may have `pre-receive` hooks that reject
  non-standard refs. Workaround: ask the admin to allow `refs/gs/*`.
- Some CI systems may try to process custom refs as branches. The `refs/gs/` namespace
  (not under `refs/heads/`) avoids this.

### Testing strategy

Before relying on custom refs with a specific provider, gs can validate:

```bash
# gs init runs this check silently
git push origin refs/gs/config:refs/gs/config
# If this fails → fall back to local-only mode with a warning
```

---

## 9. PR Metadata Layer

Custom refs handle the **structural** metadata (parent/child relationships). PR
information is a separate concern layered on top.

### Where PR data lives

PR metadata is **not** stored in refs. It is:
1. **Stored locally** in the per-branch ref blob (PR number only, as a lightweight cache)
2. **Fetched on-demand** from the provider API when detailed status is needed

### Extended per-branch blob (with PR)

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

The `pr` field is optional. It is set by `gs submit` and used by:
- **`gs log`** — to show `#42` next to the branch name, and fetch review/CI status
  on-demand from the provider.
- **`gs land`** — to check if the PR is merged before cleaning up.
- **`gs sync`** — to detect PRs that were merged outside of gs and clean up.

### Why not store full PR status in refs

PR status (reviews, CI, merge state) changes frequently and is authoritative on the
provider. Storing it in refs would create a stale cache that must be constantly
updated. Instead, gs fetches status on-demand via the provider CLI (`gh`, `glab`, etc.)
and optionally caches it in memory for the duration of a single command.

### Provider abstraction

```
gs submit
    │
    ├── Detects provider from remote URL
    │   ├── github.com → uses `gh` CLI
    │   ├── gitlab.com → uses `glab` CLI
    │   └── others     → generic git push (no PR creation)
    │
    ├── Creates/updates PR with correct base branch (from parent metadata)
    ├── Stores PR number in ref blob
    └── Pushes updated ref
```

The core sync mechanism (refs) works everywhere. PR features degrade gracefully:
- **GitHub/GitLab** — full PR creation, status, merge queue
- **Other providers** — manual PR creation, but stack structure still syncs

---

## 10. Merge Queue Integration

Merge queues require knowing the full dependency chain: which PRs must land before
others. The stack metadata in refs provides exactly this.

### How it works

```
main (trunk)
├── feat/auth (#42)          ← must land first
│   └── feat/auth-ui (#43)   ← depends on #42
└── feat/logging (#44)       ← independent
```

When submitting to a merge queue:

1. **`gs land --queue feat/auth`** enqueues #42.
2. When #42 merges, gs detects the landing (via `gs sync` or webhook) and:
   - Reparents `feat/auth-ui` to `main`
   - Restacks `feat/auth-ui`
   - Updates the PR base branch for #43 to `main`
   - Optionally enqueues #43 automatically

### Provider-specific merge queue support

| Provider | Merge Queue | gs Integration |
|----------|-------------|----------------|
| GitHub | Native (branch protection) | `gh` CLI: enable auto-merge, monitor queue position |
| GitLab | Merge trains | `glab` CLI: add to merge train |
| Others | Manual | gs handles reparenting; user merges manually |

### Dependency-aware landing

When landing a mid-stack branch, gs must update all descendants:

```
Before landing feat/auth:          After landing feat/auth:
main                               main
├── feat/auth                      ├── feat/auth-ui (reparented to main)
│   └── feat/auth-ui               └── feat/logging
└── feat/logging
```

The refs for `feat/auth-ui` are updated (parent → main) and pushed.
The PR for `feat/auth-ui` has its base branch updated on the provider.
The ref for `feat/auth` is deleted.

---

## 11. Conflict Resolution

### When conflicts can happen

Two developers update the **same branch's** metadata ref simultaneously. This is
rare in practice (metadata changes when tracking, reparenting, or landing — not
on every commit) but must be handled.

### Resolution strategy: last-writer-wins with safety checks

Git refs use last-writer-wins semantics (same as branch pushes). This is acceptable
because:

1. **Metadata conflicts are rare.** Two developers rarely modify the same branch's
   parent relationship simultaneously.
2. **Metadata is recoverable.** If a ref is overwritten, the correct parent can be
   reconstructed from the git DAG (merge-base analysis) or from PR base branches.
3. **gs validates on read.** When loading metadata, gs checks that the parent branch
   actually exists. If it doesn't, gs warns and offers to repair.

### Force-push safety

```bash
# gs uses --force-with-lease for ref pushes when possible
git push origin --force-with-lease refs/gs/meta/feat/auth
```

If someone else updated the ref since our last fetch, the push fails and gs fetches
the latest state before retrying.

### Repair command

```bash
gs repair
# → Scans all tracked branches
# → Validates parent references
# → Detects orphaned refs (branch deleted, ref remains)
# → Offers to fix inconsistencies
```

---

## 12. Security Considerations

### What is stored

Only structural metadata: branch names, parent names, timestamps, commit SHAs, and
optionally a PR number. **No secrets, tokens, credentials, or code.**

### Visibility

Custom refs follow the same access control as the repository itself. Anyone who can
clone the repo can read `refs/gs/*`. Anyone who can push can write to `refs/gs/*`.

This is appropriate because:
- Branch names are already visible to anyone with repo access.
- Parent relationships are derivable from the git DAG (just less conveniently).
- PR numbers are public within the repo.

### Tampering

A malicious actor with push access could modify refs to misrepresent stack structure.
This is no worse than modifying branch contents (which they can already do). gs
validates metadata on read and warns about inconsistencies.

### Data at rest

Refs are stored in git's object database. They are subject to the same encryption,
access control, and retention policies as the rest of the repository.

---

## 13. Migration from Local-Only Metadata

### Dual-write period

During migration, gs writes to **both** the local JSON file and git refs. This ensures
backward compatibility:

- Old gs versions read from `.gs_stack_metadata` (still updated).
- New gs versions read from refs (preferred) with fallback to local JSON.
- After a transition period, the local JSON file becomes a read cache only.

### Migration command

```bash
gs sync --migrate
# → Reads .git/.gs_stack_metadata
# → Creates refs/gs/config from .gs_config
# → Creates refs/gs/meta/<branch> for each tracked branch
# → Configures fetch refspec for refs/gs/*
# → Pushes all refs to origin
# → Prints summary of migrated branches
```

### Backward compatibility matrix

| Writer version | Reader version | Works? |
|---------------|----------------|--------|
| Old (JSON only) | Old (JSON only) | Yes (no change) |
| New (refs + JSON) | Old (JSON only) | Yes (reads JSON fallback) |
| New (refs + JSON) | New (refs + JSON) | Yes (reads refs, preferred) |
| Old (JSON only) | New (refs + JSON) | Yes (falls back to JSON) |

---

## 14. Alternatives Considered

### Committed metadata file (`.gs/stacks.json`)

Store a JSON file in the working tree, committed to the repo.

**Why not:**
- Creates merge conflicts on every branch that modifies the stack.
- Pollutes commit history with metadata noise.
- Every branch in a stack has a different version of the file, creating confusion about
  which version is authoritative.
- Force-pushes and rebases create divergent copies.

### Git notes

Attach metadata to commits via `refs/notes/gs-stack`.

**Why not:**
- Notes are keyed by **commit SHA**. Rebases change SHAs, orphaning notes.
- Requires `notes.rewriteRef` configured on every developer's machine (no default).
- Notes attach to commits, not branches — awkward fit for branch-level relationships.
- Fragile in a workflow that rebases constantly (which stacking tools do by definition).

### Commit trailers (`gs-parent: main` in commit messages)

Embed metadata in commit messages using git trailer convention.

**Why not:**
- Modifying metadata requires **rewriting the commit** (changes SHA, cascading
  force-pushes through the stack).
- Pollutes commit messages with tooling metadata that should not be part of the
  permanent record.
- Cannot represent branch-level metadata — only commit-level.
- Used by ghstack and spr, but both struggle with the rewrite cascade problem.

### Branch naming convention (`stack/main/feat-auth/feat-auth-ui`)

Encode parent in the branch name.

**Why not:**
- Ugly, unwieldy branch names that break conventions and tab completion.
- Renaming a parent requires renaming all descendants.
- Limited expressiveness (cannot encode timestamps, PR numbers, etc.).
- Breaks existing branch name conventions teams may have.

### Remote server / hosted backend

Store metadata on a Graphite-style server.

**Why not:**
- Violates "no server" design goal.
- Adds infrastructure dependency, accounts, tokens.
- Not provider-agnostic (would need to build integrations per platform).
- Graphite exists for people who want this. gs serves a different philosophy.

### GitHub PR metadata only

Reconstruct stacks from PR base branches and descriptions.

**Why not:**
- Cannot represent stacks before PRs exist (local development phase).
- Requires network for every metadata read.
- Platform-specific (GitHub only).
- Fragile — manual edits to PR descriptions break machine-readable markers.
- Acceptable as a **supplementary** layer, not the primary source of truth.

---

## 15. Glossary

| Term | Meaning |
|------|---------|
| **Trunk** | The base branch (usually `main` or `master`). Depth 0. Cannot be deleted or renamed by gs. |
| **Parent** | The branch a given branch was created on top of. Stored in the metadata ref. |
| **Ref** | A git reference — a named pointer to an object in git's database. `refs/heads/*` are branches. `refs/gs/*` are gs metadata. |
| **Refspec** | A mapping between local and remote refs, like `+refs/gs/*:refs/gs/*`. Configured in `.git/config` under `[remote "origin"]`. |
| **Blob** | A git object that stores arbitrary content. Metadata JSON is stored as blobs. |
| **hash-object** | Git command that stores content in the object database and returns its SHA. |
| **update-ref** | Git command that creates or updates a ref to point at an object. |
| **for-each-ref** | Git command that lists refs matching a pattern. Used to enumerate all metadata. |
| **Restack** | Rebasing a branch onto the current tip of its parent. Required after parent is modified. |
| **Landing** | Merging a PR and cleaning up the branch and its metadata. Children are reparented to the landed branch's parent. |
| **Merge queue** | A provider feature that serializes PR merges, running CI on each before committing. Requires knowledge of PR dependencies. |

---

**Created**: 2026-03-27
**Status**: Design document — not yet implemented
**Related**: [Implementation Plan](./branch-metadata-sync-plan.md), [GitHub Integration](../ai-context/github-integration.md), [Visualization Decisions](./visualization-decisions.md)
