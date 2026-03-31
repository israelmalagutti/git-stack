# Blob-Backed Ref Synchronization

Design rationale for how gs synchronizes metadata refs across remotes,
and why force-push semantics are both necessary and correct for this
storage model.

---

## 1. Background

gs stores stack metadata (parent-child branch relationships, configuration)
as git objects under a custom ref namespace (`refs/gs/*`). Each metadata
entry is a JSON blob written to the git object database via `git hash-object`
and pointed to by a ref updated via `git update-ref`.

This approach uses git's own transport layer for synchronization: standard
fetch/push refspecs propagate metadata to any remote without requiring an
external service, cloud backend, or provider-specific API.

## 2. The Blob-Ref Storage Model

A conventional git ref (e.g., a branch) points to a **commit object**.
Commits form a directed acyclic graph with parent pointers, establishing
an ancestry chain. Git uses this ancestry to determine whether a push is
a "fast-forward" (the remote ref is an ancestor of the local ref) and
rejects non-fast-forward pushes by default.

gs metadata refs differ in a fundamental way: they point to **blob objects**,
not commits. A blob has no parent pointer. Each metadata mutation creates
a new blob with the updated JSON content and atomically replaces the ref.
The previous blob is not referenced by the new one.

Formally:

```
Commit-backed ref:  ref -> C3 -> C2 -> C1    (ancestry exists)
Blob-backed ref:    ref -> B2                 (no ancestry; B1 is unreachable)
```

Because blobs lack ancestry, git cannot determine whether an update is a
fast-forward. From git's perspective, every push of a blob-backed ref is
non-fast-forward.

## 3. Implications for Push Semantics

Git's refspec syntax provides a mechanism for this: the `+` prefix on a
refspec instructs git to accept the update unconditionally, bypassing the
fast-forward check.

```
+refs/gs/*:refs/gs/*       # force-update: required for blob-backed refs
 refs/gs/*:refs/gs/*       # fast-forward only: always rejects blob updates
```

For gs, the force prefix is not a workaround for a conflict — it is the
only semantically correct mode of operation. Without it, the very first
metadata mutation after another user has pushed will fail, because the
local blob SHA will differ from the remote blob SHA with no common ancestor
to establish fast-forward eligibility.

Both fetch and push refspecs must use the `+` prefix. The fetch side
has used it since the initial ref-based implementation; the push side
was corrected to match.

## 4. Concurrency Model

The force-push model implies **last-writer-wins** semantics for any given
ref. Two users updating the same ref concurrently will result in one
update being silently overwritten.

This is acceptable for gs metadata for the following reasons:

**Granularity.** Each branch's metadata is stored in its own ref
(`refs/gs/meta/<branch>`). Two users modifying different branches
operate on different refs and cannot collide. The concurrent-update
scenario requires two users to modify metadata for the *same* branch
at the *same* time.

**Idempotency.** The most common metadata mutations (reparenting after
delete, updating parent revision after restack) produce deterministic
results given the same input state. Two users performing the same
operation on the same branch will write the same blob content.

**Recoverability.** Metadata is reconstructible. If a concurrent write
is lost, `gs repair` can detect and correct inconsistencies by
examining the actual git branch state. Metadata is a cache of
relationships that are ultimately derivable from the commit graph.

**Rarity.** In practice, branch ownership tends to be singular — one
developer owns a branch and its metadata. Cross-developer metadata
mutations on the same branch (e.g., both reparenting the same branch
simultaneously) are an edge case that does not justify the complexity
of a merge-based protocol.

## 5. Alternatives Considered

### 5.1 Commit-Backed Refs

Wrapping each metadata blob in a commit object would establish ancestry
and enable fast-forward pushes. However, this adds overhead (one commit
per metadata write), pollutes `git log --all` with metadata commits, and
introduces merge conflicts that would require a custom merge driver for
JSON metadata — all to handle a concurrency scenario that is rare in
practice.

### 5.2 Git Notes

Git notes (`refs/notes/*`) provide built-in merge strategies, but they
are keyed by commit SHA. Rebasing — the fundamental operation in stacked
workflows — changes commit SHAs, orphaning all associated notes. The
`notes.rewriteRef` configuration can mitigate this, but it must be
set on every developer's machine and only covers `git rebase` and
`git commit --amend`, not the manual reset-based workflows that stacking
tools sometimes employ.

### 5.3 External Sync Service

A centralized server could arbitrate metadata updates and provide
three-way merge. This eliminates the concurrency problem entirely but
introduces a service dependency, network latency on every operation,
and provider lock-in. gs's design principle is to work with any git
remote using only git's native transport. A sync service would
contradict this.

## 6. Future Considerations

If concurrent metadata conflicts become a practical problem at scale,
two evolutionary paths are available without breaking the current
storage model:

1. **Optimistic locking via `--force-with-lease`**: Replace the `+`
   refspec prefix with explicit `--force-with-lease` arguments that
   specify the expected remote SHA. This detects (but does not resolve)
   concurrent updates, allowing the tool to prompt the user rather than
   silently overwriting.

2. **Local read cache**: As the number of tracked branches grows, reading
   all refs on every invocation may become a performance bottleneck. A
   local SQLite or file-based cache that sits in front of the refs could
   amortize read costs while keeping refs as the authoritative store and
   sync mechanism.

Neither change would alter the push semantics or the ref namespace.

## 7. Summary

| Property | Blob-backed refs | Commit-backed refs |
|----------|------------------|--------------------|
| Fast-forward possible | Never | Yes |
| Force prefix required | Yes (`+`) | No |
| Ancestry / history | None | Full |
| Merge on conflict | Last-writer-wins | Three-way merge |
| Write overhead | 1 object + 1 ref update | 1 blob + 1 tree + 1 commit + 1 ref update |
| Complexity | Minimal | Significant |

For a tool-internal metadata namespace where writes are infrequent,
granularity is per-branch, and content is reconstructible, blob-backed
refs with force-push semantics are the appropriate trade-off. The
simplicity of the storage model outweighs the theoretical concurrency
limitation.
