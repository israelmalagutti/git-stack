package config

import (
	"fmt"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/git"
)

// DefaultRemote is the remote name used for ref sync operations.
const DefaultRemote = "origin"

// PushMetadataRefs pushes the specified branches' metadata refs to the remote.
// If no branches are specified, pushes all refs. Best-effort: silently skips
// if no remote exists. Batches multiple branches into a single git push.
func PushMetadataRefs(repo *git.Repo, branches ...string) {
	if !repo.HasRemote(DefaultRemote) {
		return
	}

	if len(branches) == 0 {
		if err := PushAllRefs(repo, DefaultRemote); err != nil {
			fmt.Printf("⚠ Could not push metadata refs to %s: %v\n", DefaultRemote, err)
		}
		return
	}

	// Batch multiple branches into a single git push
	refspecs := make([]string, 0, len(branches))
	for _, branch := range branches {
		ref := "refs/gs/meta/" + git.EncodeBranchRef(branch)
		refspecs = append(refspecs, ref+":"+ref)
	}

	if err := repo.PushRefs(DefaultRemote, refspecs...); err != nil {
		failed := strings.Join(branches, ", ")
		fmt.Printf("⚠ Could not push metadata refs for [%s] to %s: %v\n", failed, DefaultRemote, err)
	}
}

// DeleteRemoteMetadataRef deletes a branch's metadata ref from the remote.
// Best-effort: silently skips if no remote exists.
func DeleteRemoteMetadataRef(repo *git.Repo, branch string) {
	if !repo.HasRemote(DefaultRemote) {
		return
	}
	_ = DeleteRemoteBranchMeta(repo, DefaultRemote, branch)
}
