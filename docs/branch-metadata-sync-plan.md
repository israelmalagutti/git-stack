# Branch Metadata Sync — Implementation Plan

Step-by-step plan for implementing shareable stack metadata via custom git refs.
Each phase is scoped to one or more atomic commits that can be reviewed independently.

**Prerequisite reading**: [Branch Metadata Synchronization](./branch-metadata-sync.md)

---

## Phase Overview

```
Phase 1: Ref storage layer              (foundation — no behavior change)
Phase 2: Dual-write migration           (existing commands write to both stores)
Phase 3: Ref-first reads                (refs become primary, JSON becomes fallback)
Phase 4: Sync protocol                  (push/fetch refspecs, gs sync)
Phase 5: Newcomer bootstrap             (gs init imports from remote refs)
Phase 6: PR metadata in refs            (gs submit stores PR number)
Phase 7: Merge queue awareness          (dependency-aware landing)
Phase 8: Cleanup and local-only removal (drop JSON fallback)
```

---

## Phase 1: Ref Storage Layer

**Goal:** Build the low-level read/write layer for `refs/gs/*` without changing any
existing behavior. All current commands continue using JSON files.

### Commit 1.1 — `internal/git`: ref read/write primitives

Add methods to the `Repo` struct for storing and retrieving blobs via refs:

```go
// Write a JSON blob to a ref
func (r *Repo) WriteRef(refName string, data []byte) error
// → git hash-object -w --stdin
// → git update-ref refs/gs/<refName> <sha>

// Read a JSON blob from a ref
func (r *Repo) ReadRef(refName string) ([]byte, error)
// → git cat-file -p refs/gs/<refName>

// Delete a ref
func (r *Repo) DeleteRef(refName string) error
// → git update-ref -d refs/gs/<refName>

// List all refs under a prefix
func (r *Repo) ListRefs(prefix string) ([]string, error)
// → git for-each-ref refs/gs/<prefix> --format='%(refname)'
```

**Files changed:** `internal/git/refs.go` (new)
**Tests:** Unit tests with a temp git repo. Write blob, read it back, list refs, delete ref.

### Commit 1.2 — `internal/config`: ref-backed metadata types

Add a `RefMetadata` layer that can marshal/unmarshal the per-branch and config blobs
to/from refs, using the primitives from 1.1:

```go
// Write one branch's metadata to refs/gs/meta/<branch>
func WriteRefBranchMeta(repo *git.Repo, branch string, meta *BranchMetadata) error

// Read one branch's metadata from refs/gs/meta/<branch>
func ReadRefBranchMeta(repo *git.Repo, branch string) (*BranchMetadata, error)

// Read all branch metadata from refs/gs/meta/*
func ReadAllRefMeta(repo *git.Repo) (map[string]*BranchMetadata, error)

// Delete one branch's metadata ref
func DeleteRefBranchMeta(repo *git.Repo, branch string) error

// Write/read config ref (trunk, version)
func WriteRefConfig(repo *git.Repo, cfg *Config) error
func ReadRefConfig(repo *git.Repo) (*Config, error)
```

**Files changed:** `internal/config/ref_metadata.go` (new)
**Tests:** Integration tests — create branches, write metadata via refs, read back,
verify round-trip fidelity.

### Commit 1.3 — `internal/config`: metadata loading with ref awareness

Add a `LoadMetadataWithRefs` function that can load from refs, with fallback to JSON.
This commit does not change any callers — it only adds the capability.

```go
// LoadMetadataWithRefs tries refs first, falls back to JSON file
func LoadMetadataWithRefs(repo *git.Repo, jsonPath string) (*Metadata, MetadataSource, error)

type MetadataSource int
const (
    SourceJSON MetadataSource = iota
    SourceRefs
    SourceEmpty
)
```

**Files changed:** `internal/config/ref_metadata.go` (extend)
**Tests:** Test all three source scenarios: refs exist, only JSON exists, neither exists.

---

## Phase 2: Dual-Write Migration

**Goal:** Every command that modifies metadata now writes to both the JSON file and
git refs. Reads still come from JSON. No user-visible behavior change.

### Commit 2.1 — `internal/config`: dual-write save function

Add a `SaveWithRefs` method that writes to both stores:

```go
func (m *Metadata) SaveWithRefs(repo *git.Repo, jsonPath string) error {
    // 1. Save to JSON (existing behavior)
    if err := m.Save(jsonPath); err != nil {
        return err
    }
    // 2. Write each branch to refs/gs/meta/<branch>
    for name, meta := range m.Branches {
        if err := WriteRefBranchMeta(repo, name, meta); err != nil {
            return err
        }
    }
    // 3. Clean up refs for branches no longer in metadata
    // (compare ref list vs metadata keys, delete orphans)
    return nil
}
```

**Files changed:** `internal/config/ref_metadata.go` (extend)
**Tests:** Verify both stores contain identical data after save.

### Commit 2.2 — Update all commands to dual-write

Change every command that calls `metadata.Save()` to call `metadata.SaveWithRefs()`:

- `cmd/create.go` — `gs create`
- `cmd/track.go` — `gs track`
- `cmd/untrack.go` — `gs untrack`
- `cmd/delete.go` — `gs delete`
- `cmd/move.go` — `gs move`
- `cmd/fold.go` — `gs fold`
- `cmd/rename.go` — `gs rename`
- `cmd/stack_restack.go` — `gs restack` (updates parentRevision)
- `cmd/modify.go` — `gs modify`
- `cmd/sync.go` — `gs sync`
- `cmd/continue.go` — `gs continue`

**Approach:** Find all call sites of `metadata.Save(repo.GetMetadataPath())` and
replace with `metadata.SaveWithRefs(repo, repo.GetMetadataPath())`. The `Repo` instance
is already available in every command.

**Files changed:** All `cmd/*.go` files that save metadata.
**Tests:** Run existing test suite — all tests must pass unchanged (behavior is identical).

### Commit 2.3 — Config dual-write

Update `gs init` to also write `refs/gs/config` alongside `.gs_config`:

```go
// In cmd/init.go, after saving .gs_config
if err := config.WriteRefConfig(repo, cfg); err != nil {
    // Non-fatal: warn but continue (refs may not be available in bare repos, etc.)
    colors.Warning("Could not write config to git refs: %v", err)
}
```

**Files changed:** `cmd/init.go`, `internal/config/ref_metadata.go`
**Tests:** Verify `gs init` creates both `.gs_config` and `refs/gs/config`.

---

## Phase 3: Ref-First Reads

**Goal:** Commands read metadata from refs as the primary source, falling back to JSON
if refs are not present. This is the switch-over commit.

### Commit 3.1 — Switch metadata loading to ref-first

Update the shared metadata loading path to use `LoadMetadataWithRefs`:

Every command follows the pattern:
```go
metadata, err := config.LoadMetadata(repo.GetMetadataPath())
```

Change to:
```go
metadata, source, err := config.LoadMetadataWithRefs(repo, repo.GetMetadataPath())
if source == config.SourceJSON {
    // Auto-migrate: write refs from JSON data
    metadata.SaveWithRefs(repo, repo.GetMetadataPath())
}
```

This can be centralized in a helper:
```go
// internal/config/load.go
func LoadAndMigrate(repo *git.Repo) (*Metadata, error)
```

All commands call `LoadAndMigrate` instead of `LoadMetadata` directly.

**Files changed:** `internal/config/load.go` (new), all `cmd/*.go` files.
**Tests:**
- Ref-only repo → loads from refs.
- JSON-only repo → loads from JSON, auto-migrates to refs.
- Both exist → prefers refs.
- Empty repo → returns empty metadata.

### Commit 3.2 — Config loading ref-first

Same pattern for config:
```go
cfg, err := config.LoadConfigWithRefs(repo, repo.GetConfigPath())
```

**Files changed:** `internal/config/load.go` (extend), `cmd/*.go` files.
**Tests:** Verify config loads from refs, falls back to JSON.

---

## Phase 4: Sync Protocol

**Goal:** Enable sharing metadata via `git push`/`git fetch` with explicit commands.

### Commit 4.1 — `internal/git`: refspec management

Add methods to configure fetch refspecs for `refs/gs/*`:

```go
// EnsureGSRefspec ensures the fetch refspec for refs/gs/* is configured
func (r *Repo) EnsureGSRefspec(remote string) error
// → checks if +refs/gs/*:refs/gs/* exists in [remote "origin"].fetch
// → adds it if missing via git config --add

// PushGSRefs pushes all refs/gs/* to a remote
func (r *Repo) PushGSRefs(remote string) error
// → git push <remote> 'refs/gs/*:refs/gs/*'

// PushGSRef pushes a single ref
func (r *Repo) PushGSRef(remote, refName string) error
// → git push <remote> refs/gs/<refName>

// FetchGSRefs fetches all refs/gs/* from a remote
func (r *Repo) FetchGSRefs(remote string) error
// → git fetch <remote> 'refs/gs/*:refs/gs/*'
```

**Files changed:** `internal/git/refs.go` (extend)
**Tests:** Push refs to a bare remote, fetch into a second clone, verify refs match.

### Commit 4.2 — `gs init`: configure refspec on initialization

Update `gs init` to:
1. Configure the fetch refspec for `refs/gs/*`.
2. Attempt to fetch existing `refs/gs/*` from the remote.
3. If remote has `refs/gs/config`, use its trunk value as default.

```go
// In cmd/init.go
repo.EnsureGSRefspec("origin")
repo.FetchGSRefs("origin")
remoteCfg, err := config.ReadRefConfig(repo)
if err == nil {
    // Suggest remote's trunk as default
    defaultTrunk = remoteCfg.Trunk
}
```

**Files changed:** `cmd/init.go`
**Tests:** Clone a repo with existing refs, run `gs init`, verify metadata imported.

### Commit 4.3 — `gs sync`: push and fetch refs

Extend the existing `gs sync` command to include ref synchronization:

```bash
gs sync              # fetches branches + refs, cleans up merged branches
gs sync --push       # also pushes refs/gs/* to remote
```

The sync flow:
1. `git fetch` (includes `refs/gs/*` via configured refspec)
2. Load metadata from refs
3. Detect branches whose remote was deleted (PR merged) → offer to clean up
4. If `--push`: push local `refs/gs/*` to remote

**Files changed:** `cmd/sync.go`
**Tests:** Two-clone sync scenario — push from clone A, fetch in clone B, verify metadata.

### Commit 4.4 — Auto-push refs on metadata mutation

When `SaveWithRefs` is called, optionally push the changed refs to remote. This is
controlled by a config flag:

```json
// refs/gs/config
{
  "version": "1.0.0",
  "trunk": "main",
  "initialized": "2026-03-27T14:00:00Z",
  "autoSync": false
}
```

When `autoSync` is true, every metadata write also pushes the changed ref(s). Default
is false — explicit `gs sync --push` is required.

**Files changed:** `internal/config/ref_metadata.go`, `internal/config/config.go`
**Tests:** Verify auto-push behavior when enabled. Verify no push when disabled.

---

## Phase 5: Newcomer Bootstrap

**Goal:** A developer who clones a gs-enabled repo can run `gs init` and immediately
see the full stack.

### Commit 5.1 — `gs init`: detect and import remote metadata

Enhance `gs init` to handle the "newcomer" case:

```
gs init
├── Is there a local .gs_config? → already initialized, skip
├── Fetch refs/gs/* from origin
├── Is there a remote refs/gs/config?
│   ├── Yes → import trunk, import all branch metadata
│   │         → "Imported 5 tracked branches from remote."
│   └── No  → normal init flow (prompt for trunk)
└── Configure fetch refspec
```

**Files changed:** `cmd/init.go`
**Tests:**
- Fresh clone of a gs-enabled repo → `gs init` imports everything.
- Fresh clone of a non-gs repo → `gs init` behaves as today.
- Re-running `gs init` on an already-initialized repo → no-op or `--reset` behavior.

### Commit 5.2 — `gs log`: graceful handling of partial metadata

After import, some branches in refs may not exist locally (not checked out yet).
`gs log` should handle this gracefully:

- Show branches that exist locally with full visualization.
- Show branches that exist only in refs as "remote-only" with a dimmed indicator.
- Don't crash if a ref references a branch that has been deleted on the remote.

```
  ○ feat/auth-ui          (#43)
  │
  │
──┤
  │
  ○ feat/auth             (#42)
  │   ◌ feat/auth-tests   (remote only)
  │
──┤
  │
  ● main
```

**Files changed:** `internal/stack/stack.go`, `internal/stack/visualize.go`
**Tests:** Build stack with a mix of local and remote-only branches, verify rendering.

---

## Phase 6: PR Metadata in Refs

**Goal:** `gs submit` stores PR number in the branch's metadata ref. `gs log` displays it.

### Commit 6.1 — Extend `BranchMetadata` with PR field

```go
type BranchMetadata struct {
    Parent         string    `json:"parent"`
    Tracked        bool      `json:"tracked"`
    Created        time.Time `json:"created"`
    ParentRevision string    `json:"parentRevision,omitempty"`
    PR             *PRInfo   `json:"pr,omitempty"`
}

type PRInfo struct {
    Number   int    `json:"number"`
    Provider string `json:"provider"` // "github", "gitlab", etc.
}
```

**Files changed:** `internal/config/metadata.go`
**Tests:** Verify JSON round-trip with and without PR field.

### Commit 6.2 — `internal/provider`: provider detection and abstraction

Create a provider abstraction that detects the git hosting provider from the remote URL
and exposes PR operations:

```go
// internal/provider/provider.go
type Provider interface {
    Name() string                              // "github", "gitlab"
    CreatePR(opts PRCreateOpts) (*PRResult, error)
    UpdatePR(number int, opts PRUpdateOpts) error
    GetPRStatus(number int) (*PRStatus, error)
    MergePR(number int, opts PRMergeOpts) error
    CLIAvailable() bool                        // is gh/glab installed?
    CLIAuthenticated() bool                    // is the user logged in?
}

// internal/provider/detect.go
func DetectProvider(remoteURL string) (Provider, error)
// → parses remote URL
// → returns GitHubProvider, GitLabProvider, or GenericProvider
```

For Phase 6, only implement `GitHubProvider` (using `gh` CLI). Other providers are
stubs that return "not implemented" errors.

**Files changed:** `internal/provider/` (new package)
**Tests:** URL detection tests. Mock `gh` CLI responses for PR operations.

### Commit 6.3 — `gs submit`: create/update PR and store number

Implement the `gs submit` command:

1. Detect provider from remote.
2. Check CLI is available and authenticated.
3. Push the current branch to remote.
4. Create or update the PR (base branch = parent from metadata).
5. Store PR number in the branch's metadata ref.
6. Push the updated ref.

```bash
gs submit                    # submit current branch
gs submit --draft            # submit as draft
gs submit --stack            # submit all branches in stack (bottom-up)
```

**Files changed:** `cmd/submit.go` (new), `internal/provider/github.go`
**Tests:** Mock `gh` CLI. Verify PR creation, metadata update, ref push.

### Commit 6.4 — `gs log`: display PR numbers and status

When a branch has a PR number in its metadata, `gs log` shows it:

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

PR status is fetched on-demand from the provider. If offline or the provider CLI is
not available, only the PR number is shown (no status icon).

Add a `--no-pr` flag to skip PR status fetching for speed.

**Files changed:** `cmd/log.go`, `internal/stack/visualize.go`, `internal/provider/github.go`
**Tests:** Verify rendering with/without PR info. Verify graceful degradation offline.

---

## Phase 7: Merge Queue Awareness

**Goal:** `gs land` handles merged PRs, reparents children, and integrates with
provider merge queues.

### Commit 7.1 — `gs land`: basic landing flow

Implement the core landing command:

1. Verify the PR is merged (via provider API).
2. Reparent all children to the landed branch's parent.
3. Update children's PR base branches on the provider.
4. Delete the landed branch locally and remotely.
5. Delete the branch's metadata ref.
6. Push all updated refs.
7. Restack children.

```bash
gs land                     # land current branch
gs land feat/auth           # land specific branch
gs land --stack             # land all merged branches in stack (bottom-up)
```

**Files changed:** `cmd/land.go` (new)
**Tests:** Full landing flow with mock provider. Verify reparenting, ref cleanup, restack.

### Commit 7.2 — `gs sync`: detect and land merged branches

Extend `gs sync` to detect branches whose PRs have been merged (outside of gs):

```
gs sync
├── Fetch branches + refs
├── For each tracked branch with a PR number:
│   ├── Check PR status via provider
│   ├── If merged → offer to land (reparent + cleanup)
│   └── If closed → offer to untrack
└── Push updated refs
```

**Files changed:** `cmd/sync.go`
**Tests:** Simulate externally-merged PR. Verify sync detects it and offers landing.

### Commit 7.3 — Merge queue integration

Add `--queue` flag to `gs land` that uses the provider's merge queue instead of
direct merge:

```bash
gs land --queue             # enqueue current branch's PR
gs land --queue --stack     # enqueue all PRs in stack order
```

For GitHub:
- Enable auto-merge on the PR.
- Add to merge queue if branch protection requires it.
- Monitor queue position in `gs log`.

**Files changed:** `cmd/land.go` (extend), `internal/provider/github.go` (extend)
**Tests:** Mock merge queue API. Verify enqueue behavior.

---

## Phase 8: Cleanup

**Goal:** Remove the JSON fallback and local-only code paths. Refs are the sole
metadata store.

### Commit 8.1 — Remove JSON dual-write

Remove all `Save(jsonPath)` calls. `SaveWithRefs` becomes `Save`:

- Delete `metadata.Save(path string)` (JSON-only save)
- Rename `SaveWithRefs` → `Save`
- Remove JSON loading fallback from `LoadAndMigrate`
- Delete `.gs_stack_metadata` on first run after migration

**Files changed:** `internal/config/metadata.go`, `internal/config/ref_metadata.go`,
`internal/config/load.go`, all `cmd/*.go` files.
**Tests:** Full test suite passes with refs only. No JSON files created.

### Commit 8.2 — Remove JSON config fallback

Same as 8.1 but for `.gs_config`:

- `gs init` writes only to `refs/gs/config`
- Config loading reads only from refs
- Delete `.gs_config` on first run after migration

**Files changed:** `internal/config/config.go`, `cmd/init.go`
**Tests:** Full test suite passes without JSON config files.

### Commit 8.3 — Clean up migration code

Remove:
- `LoadMetadataWithRefs` (no longer needs fallback logic)
- `MetadataSource` type
- `--migrate` flag from `gs sync`
- Any `.gw_` → `.gs_` migration code (from the earlier rename)

**Files changed:** `internal/config/load.go`, `cmd/sync.go`
**Tests:** Full test suite. Verify clean startup on fresh repos.

### Commit 8.4 — Update documentation

- Update `CLAUDE.md` architecture section to reflect ref-based storage.
- Update `ai-context/roadmap.md` to mark Phase 8/9 as complete.
- Update `ai-context/team-workflow-sync.md` to reflect the chosen approach.
- Remove outdated references to `.gs_stack_metadata` and `.gs_config`.

**Files changed:** `CLAUDE.md`, `ai-context/roadmap.md`, `ai-context/team-workflow-sync.md`

---

## Dependency Graph

```
Phase 1 ──► Phase 2 ──► Phase 3 ──► Phase 4 ──► Phase 5
                                        │
                                        ▼
                                    Phase 6 ──► Phase 7
                                                   │
                                                   ▼
                                               Phase 8
```

- Phases 1–5 are strictly sequential (each builds on the previous).
- Phase 6 (PR metadata) depends on Phase 4 (sync protocol) but can be developed in
  parallel with Phase 5 if the newcomer bootstrap is not needed for PR features.
- Phase 7 (merge queues) depends on Phase 6 (needs PR numbers in refs).
- Phase 8 (cleanup) should only happen after Phase 7 is stable and deployed.

---

## Risk Register

| Risk | Mitigation |
|------|------------|
| Provider rejects `refs/gs/*` pushes | Phase 4.2 validates on `gs init`. Fall back to local-only with warning. |
| Many refs slow git operations | Monitor. If >500 branches, consider packing refs or a local SQLite cache. |
| Two developers overwrite same ref | Use `--force-with-lease`. Last-writer-wins is acceptable for metadata. |
| `gh` CLI not installed | Graceful degradation. Sync works (git-native). PR features warn and skip. |
| Migration breaks existing workflows | Dual-write period (Phase 2–3) ensures old behavior continues working. |
| Rebases orphan metadata | N/A — refs are keyed by branch name, not commit SHA. This is the core advantage. |

---

## Commit Checklist Template

For each commit:
- [ ] Implementation matches the commit description
- [ ] New code has unit tests
- [ ] Existing tests pass unchanged (unless intentionally modified)
- [ ] No JSON secrets or credentials in ref blobs
- [ ] `go vet` and `go build` pass
- [ ] Commit message references this plan (e.g., "Phase 2.1: dual-write save")

---

**Created**: 2026-03-27
**Status**: Implementation plan — not yet started
**Related**: [Branch Metadata Synchronization](./branch-metadata-sync.md), [GitHub Integration](../ai-context/github-integration.md)
