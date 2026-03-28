package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PRInfo stores a pull request reference for a tracked branch.
type PRInfo struct {
	Number   int    `json:"number"`
	Provider string `json:"provider"` // "github", "gitlab"
}

// BranchMetadata represents metadata for a tracked branch
type BranchMetadata struct {
	Parent         string    `json:"parent"`
	Tracked        bool      `json:"tracked"`
	Created        time.Time `json:"created"`
	ParentRevision string    `json:"parentRevision,omitempty"`
	PR             *PRInfo   `json:"pr,omitempty"`
}

// Metadata represents the stack metadata
type Metadata struct {
	Branches map[string]*BranchMetadata `json:"branches"`
}

// LoadMetadata reads the metadata from the specified path
func LoadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty metadata if file doesn't exist yet
			return &Metadata{
				Branches: make(map[string]*BranchMetadata),
			}, nil
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Ensure map is initialized
	if metadata.Branches == nil {
		metadata.Branches = make(map[string]*BranchMetadata)
	}

	return &metadata, nil
}

// Save writes the metadata to the specified path
func (m *Metadata) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// TrackBranch adds or updates a branch in the metadata
func (m *Metadata) TrackBranch(branch, parent, parentRevision string) {
	m.Branches[branch] = &BranchMetadata{
		Parent:         parent,
		Tracked:        true,
		Created:        time.Now(),
		ParentRevision: parentRevision,
	}
}

// SetParentRevision updates the parent revision SHA for a tracked branch
func (m *Metadata) SetParentRevision(branch, sha string) error {
	meta, exists := m.Branches[branch]
	if !exists {
		return fmt.Errorf("branch %s is not tracked", branch)
	}
	meta.ParentRevision = sha
	return nil
}

// GetParentRevision returns the parent revision SHA for a tracked branch
func (m *Metadata) GetParentRevision(branch string) string {
	meta, exists := m.Branches[branch]
	if !exists {
		return ""
	}
	return meta.ParentRevision
}

// UntrackBranch removes a branch from the metadata
func (m *Metadata) UntrackBranch(branch string) {
	delete(m.Branches, branch)
}

// IsTracked checks if a branch is tracked
func (m *Metadata) IsTracked(branch string) bool {
	_, exists := m.Branches[branch]
	return exists
}

// GetParent returns the parent branch of a branch
func (m *Metadata) GetParent(branch string) (string, bool) {
	meta, exists := m.Branches[branch]
	if !exists {
		return "", false
	}
	return meta.Parent, true
}

// GetChildren returns all children of a branch
func (m *Metadata) GetChildren(branch string) []string {
	children := []string{}
	for name, meta := range m.Branches {
		if meta.Parent == branch {
			children = append(children, name)
		}
	}
	return children
}

// UpdateParent updates the parent of a branch
func (m *Metadata) UpdateParent(branch, newParent string) error {
	meta, exists := m.Branches[branch]
	if !exists {
		return fmt.Errorf("branch %s is not tracked", branch)
	}
	meta.Parent = newParent
	return nil
}

// SetPR stores a PR reference for a tracked branch.
func (m *Metadata) SetPR(branch string, pr *PRInfo) error {
	meta, exists := m.Branches[branch]
	if !exists {
		return fmt.Errorf("branch %s is not tracked", branch)
	}
	meta.PR = pr
	return nil
}

// GetPR returns the PR reference for a branch, or nil if none.
func (m *Metadata) GetPR(branch string) *PRInfo {
	meta, exists := m.Branches[branch]
	if !exists || meta.PR == nil {
		return nil
	}
	return meta.PR
}
