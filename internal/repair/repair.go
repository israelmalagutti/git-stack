package repair

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// IssueKind categorizes the type of metadata inconsistency found.
type IssueKind string

const (
	OrphanedRef     IssueKind = "orphaned_ref"      // ref exists but git branch was deleted
	MissingParent   IssueKind = "missing_parent"     // tracked branch's parent doesn't exist
	CircularParent  IssueKind = "circular_parent"    // parent chain forms a cycle
	RefJSONMismatch IssueKind = "ref_json_mismatch"  // ref and JSON metadata disagree on parent
	RemoteDeleted   IssueKind = "remote_deleted"     // branch exists locally but remote was deleted
)

// Issue represents a single metadata inconsistency.
type Issue struct {
	Kind        IssueKind
	Branch      string
	Description string
	Fix         string // human-readable description of what the fix does
}

// DetectIssues scans refs and metadata for inconsistencies against the actual git state.
func DetectIssues(repo *git.Repo, metadata *config.Metadata, cfg *config.Config) ([]Issue, error) {
	var issues []Issue

	// Load refs for comparison
	refBranches, err := config.ReadAllRefMeta(repo)
	if err != nil {
		// If refs can't be read, treat as empty (no ref-based checks)
		refBranches = make(map[string]*config.BranchMetadata)
	}

	// 1. Orphaned refs: ref exists but git branch doesn't
	for branch := range refBranches {
		if branch == cfg.Trunk {
			continue
		}
		if !repo.BranchExists(branch) {
			// Also check if it's in metadata (might just be a stale ref)
			issues = append(issues, Issue{
				Kind:        OrphanedRef,
				Branch:      branch,
				Description: fmt.Sprintf("ref exists for '%s' but the git branch was deleted", branch),
				Fix:         fmt.Sprintf("delete ref and untrack '%s'", branch),
			})
		}
	}

	// 2. Orphaned metadata entries: tracked in metadata but git branch doesn't exist
	for branch := range metadata.Branches {
		if branch == cfg.Trunk {
			continue
		}
		if !repo.BranchExists(branch) {
			// Only add if not already caught by orphaned ref check
			alreadyCaught := false
			for _, iss := range issues {
				if iss.Branch == branch && iss.Kind == OrphanedRef {
					alreadyCaught = true
					break
				}
			}
			if !alreadyCaught {
				issues = append(issues, Issue{
					Kind:        OrphanedRef,
					Branch:      branch,
					Description: fmt.Sprintf("'%s' is tracked but the git branch was deleted", branch),
					Fix:         fmt.Sprintf("untrack '%s' and delete its ref", branch),
				})
			}
		}
	}

	// 3. Missing parent: tracked branch's parent doesn't exist
	for branch := range metadata.Branches {
		parent, ok := metadata.GetParent(branch)
		if !ok || parent == "" {
			continue
		}
		if parent == cfg.Trunk {
			continue
		}
		if !repo.BranchExists(parent) {
			issues = append(issues, Issue{
				Kind:        MissingParent,
				Branch:      branch,
				Description: fmt.Sprintf("'%s' has parent '%s' which no longer exists", branch, parent),
				Fix:         fmt.Sprintf("reparent '%s' to trunk ('%s')", branch, cfg.Trunk),
			})
		}
	}

	// 4. Circular parent chain detection
	for branch := range metadata.Branches {
		if detectCycle(metadata, branch, cfg.Trunk) {
			parent, _ := metadata.GetParent(branch)
			issues = append(issues, Issue{
				Kind:        CircularParent,
				Branch:      branch,
				Description: fmt.Sprintf("'%s' is part of a circular parent chain (parent: '%s')", branch, parent),
				Fix:         fmt.Sprintf("reparent '%s' to trunk ('%s') to break the cycle", branch, cfg.Trunk),
			})
			break // report one cycle, not every node in it
		}
	}

	// 5. Remote deleted: branch exists locally but remote counterpart is gone
	if repo.HasRemote("origin") {
		for branch := range metadata.Branches {
			if branch == cfg.Trunk {
				continue
			}
			if !repo.BranchExists(branch) {
				continue // already caught by orphaned checks
			}
			if !repo.HasRemoteBranch(branch, "origin") {
				issues = append(issues, Issue{
					Kind:        RemoteDeleted,
					Branch:      branch,
					Description: fmt.Sprintf("'%s' exists locally but has no remote branch (likely merged/deleted upstream)", branch),
					Fix:         fmt.Sprintf("delete local branch '%s' and untrack", branch),
				})
			}
		}
	}

	// 6. Ref/JSON parent mismatch
	for branch, refMeta := range refBranches {
		jsonMeta, exists := metadata.Branches[branch]
		if !exists {
			continue // handled by orphaned ref check
		}
		if refMeta.Parent != jsonMeta.Parent {
			issues = append(issues, Issue{
				Kind:        RefJSONMismatch,
				Branch:      branch,
				Description: fmt.Sprintf("'%s' has parent '%s' in refs but '%s' in JSON", branch, refMeta.Parent, jsonMeta.Parent),
				Fix:         fmt.Sprintf("update '%s' parent to '%s' (from refs)", branch, refMeta.Parent),
			})
		}
	}

	return issues, nil
}

// detectCycle checks if walking the parent chain from branch leads to a cycle.
func detectCycle(metadata *config.Metadata, branch, trunk string) bool {
	visited := make(map[string]bool)
	current := branch
	for {
		if visited[current] {
			return true
		}
		visited[current] = true
		parent, ok := metadata.GetParent(current)
		if !ok || parent == "" || parent == trunk {
			return false
		}
		// If parent is not tracked, chain ends
		if !metadata.IsTracked(parent) {
			return false
		}
		current = parent
	}
}

// ApplyFix fixes a single issue by mutating metadata in place.
// The caller is responsible for saving metadata and pushing refs afterward.
func ApplyFix(repo *git.Repo, metadata *config.Metadata, cfg *config.Config, issue Issue) error {
	switch issue.Kind {
	case OrphanedRef:
		metadata.UntrackBranch(issue.Branch)
		// Best-effort ref cleanup (SaveWithRefs will also clean orphaned refs)
		_ = config.DeleteRefBranchMeta(repo, issue.Branch)
		return nil

	case MissingParent:
		return metadata.UpdateParent(issue.Branch, cfg.Trunk)

	case CircularParent:
		return metadata.UpdateParent(issue.Branch, cfg.Trunk)

	case RemoteDeleted:
		// Reparent children to this branch's parent before removing
		parent, _ := metadata.GetParent(issue.Branch)
		if parent == "" {
			parent = cfg.Trunk
		}
		for child := range metadata.Branches {
			childParent, ok := metadata.GetParent(child)
			if ok && childParent == issue.Branch {
				_ = metadata.UpdateParent(child, parent)
			}
		}
		// Delete local git branch and untrack
		_ = repo.DeleteBranch(issue.Branch, true)
		metadata.UntrackBranch(issue.Branch)
		_ = config.DeleteRefBranchMeta(repo, issue.Branch)
		return nil

	case RefJSONMismatch:
		refMeta, err := config.ReadRefBranchMeta(repo, issue.Branch)
		if err != nil {
			return fmt.Errorf("failed to read ref for '%s': %w", issue.Branch, err)
		}
		// Refs win: update in-memory metadata to match ref
		metadata.Branches[issue.Branch].Parent = refMeta.Parent
		return nil

	default:
		return fmt.Errorf("unknown issue kind: %s", issue.Kind)
	}
}
