package cmd

import (
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

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
