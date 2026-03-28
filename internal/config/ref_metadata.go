package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/git"
)

// MetadataSource indicates where metadata was loaded from.
type MetadataSource int

const (
	// SourceEmpty means no metadata was found anywhere.
	SourceEmpty MetadataSource = iota
	// SourceJSON means metadata was loaded from the JSON file.
	SourceJSON
	// SourceRefs means metadata was loaded from git refs.
	SourceRefs
)

const metaRefPrefix = "meta/"

// WriteRefBranchMeta writes one branch's metadata to refs/gs/meta/<encoded-branch>.
func WriteRefBranchMeta(repo *git.Repo, branch string, meta *BranchMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal branch metadata for %s: %w", branch, err)
	}

	refName := metaRefPrefix + git.EncodeBranchRef(branch)
	return repo.WriteRef(refName, data)
}

// ReadRefBranchMeta reads one branch's metadata from refs/gs/meta/<encoded-branch>.
func ReadRefBranchMeta(repo *git.Repo, branch string) (*BranchMetadata, error) {
	refName := metaRefPrefix + git.EncodeBranchRef(branch)
	data, err := repo.ReadRef(refName)
	if err != nil {
		return nil, fmt.Errorf("failed to read ref metadata for %s: %w", branch, err)
	}

	var meta BranchMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse ref metadata for %s: %w", branch, err)
	}

	return &meta, nil
}

// ReadAllRefMeta reads all branch metadata from refs/gs/meta/*.
func ReadAllRefMeta(repo *git.Repo) (map[string]*BranchMetadata, error) {
	refs, err := repo.ListRefs(metaRefPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list metadata refs: %w", err)
	}

	branches := make(map[string]*BranchMetadata)
	for _, ref := range refs {
		encoded := strings.TrimPrefix(ref, metaRefPrefix)
		branchName := git.DecodeBranchRef(encoded)

		data, err := repo.ReadRef(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to read ref %s: %w", ref, err)
		}

		var meta BranchMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("failed to parse metadata for %s: %w", branchName, err)
		}

		branches[branchName] = &meta
	}

	return branches, nil
}

// DeleteRefBranchMeta deletes one branch's metadata ref.
func DeleteRefBranchMeta(repo *git.Repo, branch string) error {
	refName := metaRefPrefix + git.EncodeBranchRef(branch)
	return repo.DeleteRef(refName)
}

// WriteRefConfig writes the gs config to refs/gs/config.
func WriteRefConfig(repo *git.Repo, cfg *Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return repo.WriteRef("config", data)
}

// ReadRefConfig reads the gs config from refs/gs/config.
func ReadRefConfig(repo *git.Repo) (*Config, error) {
	data, err := repo.ReadRef("config")
	if err != nil {
		return nil, fmt.Errorf("failed to read config ref: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config ref: %w", err)
	}

	return &cfg, nil
}

// LoadMetadataWithRefs tries to load metadata from git refs first,
// falling back to the JSON file if refs are not present.
func LoadMetadataWithRefs(repo *git.Repo, jsonPath string) (*Metadata, MetadataSource, error) {
	// Try refs first
	branches, err := ReadAllRefMeta(repo)
	if err == nil && len(branches) > 0 {
		return &Metadata{Branches: branches}, SourceRefs, nil
	}

	// Fall back to JSON
	meta, err := LoadMetadata(jsonPath)
	if err != nil {
		return nil, SourceEmpty, err
	}

	if len(meta.Branches) > 0 {
		return meta, SourceJSON, nil
	}

	return meta, SourceEmpty, nil
}

// SaveWithRefs writes metadata to both the JSON file and git refs.
// It also cleans up refs for branches that are no longer tracked.
func (m *Metadata) SaveWithRefs(repo *git.Repo, jsonPath string) error {
	// 1. Save to JSON (existing behavior)
	if err := m.Save(jsonPath); err != nil {
		return err
	}

	// 2. Write each branch to refs
	for name, meta := range m.Branches {
		if err := WriteRefBranchMeta(repo, name, meta); err != nil {
			return fmt.Errorf("failed to write ref for %s: %w", name, err)
		}
	}

	// 3. Clean up refs for branches no longer in metadata
	existingRefs, err := repo.ListRefs(metaRefPrefix)
	if err != nil {
		// Non-fatal: refs may not be available
		return nil
	}

	tracked := make(map[string]bool)
	for name := range m.Branches {
		tracked[metaRefPrefix+git.EncodeBranchRef(name)] = true
	}

	for _, ref := range existingRefs {
		if !tracked[ref] {
			_ = repo.DeleteRef(ref) // Best-effort cleanup
		}
	}

	return nil
}

// gsRefspec is the fetch refspec for gs metadata refs.
const gsRefspec = "+refs/gs/*:refs/gs/*"

// ConfigureRemoteRefspec sets up the fetch refspec for gs refs on a remote.
func ConfigureRemoteRefspec(repo *git.Repo, remote string) error {
	return repo.ConfigureRefspec(remote, gsRefspec)
}

// FetchAllRefs fetches all refs/gs/* from the remote.
func FetchAllRefs(repo *git.Repo, remote string) error {
	return repo.FetchRefs(remote, gsRefspec)
}

// PushAllRefs pushes all refs/gs/* to the remote.
func PushAllRefs(repo *git.Repo, remote string) error {
	return repo.PushRefs(remote, "refs/gs/*:refs/gs/*")
}

// PushBranchMeta pushes a single branch's metadata ref to the remote.
func PushBranchMeta(repo *git.Repo, remote, branch string) error {
	ref := "refs/gs/" + metaRefPrefix + git.EncodeBranchRef(branch)
	refspec := ref + ":" + ref
	return repo.PushRefs(remote, refspec)
}

// PushConfig pushes refs/gs/config to the remote.
func PushConfig(repo *git.Repo, remote string) error {
	return repo.PushRefs(remote, "refs/gs/config:refs/gs/config")
}

// DeleteRemoteBranchMeta deletes a branch's metadata ref from the remote.
func DeleteRemoteBranchMeta(repo *git.Repo, remote, branch string) error {
	refName := metaRefPrefix + git.EncodeBranchRef(branch)
	return repo.DeleteRemoteRef(remote, refName)
}
