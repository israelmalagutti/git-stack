package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// defaultRemote is the remote used for ref sync operations.
const defaultRemote = config.DefaultRemote

// ensureRefspec is a one-time check per process. Once the refspec is confirmed
// configured, we skip the check for all subsequent loadMetadata calls.
var refspecConfigured bool

// loadMetadata loads stack metadata using refs as the primary source,
// falling back to the JSON file. If metadata is found only in JSON,
// it is auto-migrated to refs for future access.
//
// On first call, ensures the fetch refspec for refs/gs/* is configured
// so that future git fetches include metadata from the remote. This
// handles upgrades from pre-ref versions without requiring gs init --reset.
func loadMetadata(repo *git.Repo) (*config.Metadata, error) {
	// Ensure the refspec is configured (idempotent, once per process).
	// This handles the upgrade case: Bob had an old gs version without refs,
	// updates, and never runs gs init again. Without this, git fetch would
	// never pull refs/gs/* and team sync would silently be one-way only.
	if !refspecConfigured {
		configureGSRefspec(repo)
		refspecConfigured = true
	}

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

// pushMetadataRefs delegates to config.PushMetadataRefs.
func pushMetadataRefs(repo *git.Repo, branches ...string) {
	config.PushMetadataRefs(repo, branches...)
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

// deleteRemoteMetadataRef delegates to config.DeleteRemoteMetadataRef.
func deleteRemoteMetadataRef(repo *git.Repo, branch string) {
	config.DeleteRemoteMetadataRef(repo, branch)
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
