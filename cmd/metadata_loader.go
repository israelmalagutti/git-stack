package cmd

import (
	"fmt"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// defaultRemote is the remote used for ref sync operations.
const defaultRemote = "origin"

// loadMetadata loads stack metadata using refs as the primary source,
// falling back to the JSON file. If metadata is found only in JSON,
// it is auto-migrated to refs for future access.
func loadMetadata(repo *git.Repo) (*config.Metadata, error) {
	jsonPath := repo.GetMetadataPath()

	meta, source, err := config.LoadMetadataWithRefs(repo, jsonPath)
	if err != nil {
		return nil, err
	}

	// Auto-migrate: if we loaded from JSON, write refs so future loads are ref-first
	if source == config.SourceJSON {
		if migrateErr := meta.SaveWithRefs(repo, jsonPath); migrateErr != nil {
			fmt.Printf("⚠ Auto-migration to refs failed: %v\n", migrateErr)
		}
	}

	return meta, nil
}

func repoHasRemote(repo *git.Repo) bool {
	return repo.HasRemote(defaultRemote)
}

// pushMetadataRefs pushes the specified branches' metadata refs to the remote.
// If no branches are specified, pushes all refs. Best-effort: silently skips
// if no remote exists.
func pushMetadataRefs(repo *git.Repo, branches ...string) {
	if !repoHasRemote(repo) {
		return
	}

	if len(branches) == 0 {
		if err := config.PushAllRefs(repo, defaultRemote); err != nil {
			fmt.Printf("⚠ Could not push metadata refs to %s: %v\n", defaultRemote, err)
		}
		return
	}

	// Batch multiple branches into a single git push
	refspecs := make([]string, 0, len(branches))
	for _, branch := range branches {
		ref := "refs/gs/meta/" + git.EncodeBranchRef(branch)
		refspecs = append(refspecs, ref+":"+ref)
	}

	if err := repo.PushRefs(defaultRemote, refspecs...); err != nil {
		failed := strings.Join(branches, ", ")
		fmt.Printf("⚠ Could not push metadata refs for [%s] to %s: %v\n", failed, defaultRemote, err)
	}
}

// pushConfigRef pushes refs/gs/config to the remote. Best-effort.
func pushConfigRef(repo *git.Repo) {
	if !repoHasRemote(repo) {
		return
	}
	if err := config.PushConfig(repo, defaultRemote); err != nil {
		fmt.Printf("⚠ Could not push config ref to %s: %v\n", defaultRemote, err)
	}
}

// deleteRemoteMetadataRef deletes a branch's metadata ref from the remote. Best-effort.
func deleteRemoteMetadataRef(repo *git.Repo, branch string) {
	if !repoHasRemote(repo) {
		return
	}
	// Best-effort: ignore errors (ref may not exist on remote)
	_ = config.DeleteRemoteBranchMeta(repo, defaultRemote, branch)
}

// fetchMetadataRefs fetches all refs/gs/* from the remote. Best-effort.
// Returns true if fetch succeeded.
func fetchMetadataRefs(repo *git.Repo) bool {
	if !repoHasRemote(repo) {
		return false
	}
	err := config.FetchAllRefs(repo, defaultRemote)
	return err == nil
}

// configureGSRefspec sets up the fetch refspec for gs refs on the default remote.
// Best-effort.
func configureGSRefspec(repo *git.Repo) {
	if !repoHasRemote(repo) {
		return
	}
	_ = config.ConfigureRemoteRefspec(repo, defaultRemote)
}
