package cmd

import (
	"fmt"

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
		_ = meta.SaveWithRefs(repo, jsonPath)
	}

	return meta, nil
}

// pushMetadataRefs pushes the specified branches' metadata refs to the remote.
// If no branches are specified, pushes all refs. Best-effort: silently skips
// if no remote exists and logs warnings on failure.
func pushMetadataRefs(repo *git.Repo, branches ...string) {
	if !repo.HasRemote(defaultRemote) {
		return
	}

	var err error
	if len(branches) == 0 {
		err = config.PushAllRefs(repo, defaultRemote)
	} else {
		for _, branch := range branches {
			if pushErr := config.PushBranchMeta(repo, defaultRemote, branch); pushErr != nil {
				err = pushErr
			}
		}
	}

	if err != nil {
		fmt.Printf("⚠ Could not push metadata refs to %s: %v\n", defaultRemote, err)
	}
}

// pushConfigRef pushes refs/gs/config to the remote. Best-effort.
func pushConfigRef(repo *git.Repo) {
	if !repo.HasRemote(defaultRemote) {
		return
	}
	if err := config.PushConfig(repo, defaultRemote); err != nil {
		fmt.Printf("⚠ Could not push config ref to %s: %v\n", defaultRemote, err)
	}
}

// deleteRemoteMetadataRef deletes a branch's metadata ref from the remote. Best-effort.
func deleteRemoteMetadataRef(repo *git.Repo, branch string) {
	if !repo.HasRemote(defaultRemote) {
		return
	}
	// Best-effort: ignore errors (ref may not exist on remote)
	_ = config.DeleteRemoteBranchMeta(repo, defaultRemote, branch)
}

// fetchMetadataRefs fetches all refs/gs/* from the remote. Best-effort.
// Returns true if fetch succeeded.
func fetchMetadataRefs(repo *git.Repo) bool {
	if !repo.HasRemote(defaultRemote) {
		return false
	}
	err := config.FetchAllRefs(repo, defaultRemote)
	return err == nil
}

// configureGSRefspec sets up the fetch refspec for gs refs on the default remote.
// Best-effort.
func configureGSRefspec(repo *git.Repo) {
	if !repo.HasRemote(defaultRemote) {
		return
	}
	_ = config.ConfigureRemoteRefspec(repo, defaultRemote)
}
