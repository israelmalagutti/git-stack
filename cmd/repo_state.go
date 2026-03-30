package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

// repoState holds all the state needed by most command handlers.
type repoState struct {
	Repo     *git.Repo
	Config   *config.Config
	Metadata *config.Metadata
	Stack    *stack.Stack
}

// loadRepoState initializes repo, config, metadata, and builds the stack tree.
// Use this for commands that need the full stack.
func loadRepoState() (*repoState, error) {
	repo, err := git.NewRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return nil, err
	}

	metadata, err := loadMetadata(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to build stack: %w", err)
	}

	return &repoState{
		Repo:     repo,
		Config:   cfg,
		Metadata: metadata,
		Stack:    s,
	}, nil
}

// loadRepoConfig initializes repo, config, and metadata without building the stack.
// Use this for commands that don't need the full stack tree (e.g., create, rename, track).
func loadRepoConfig() (*repoState, error) {
	repo, err := git.NewRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return nil, err
	}

	metadata, err := loadMetadata(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	return &repoState{
		Repo:     repo,
		Config:   cfg,
		Metadata: metadata,
	}, nil
}

// RebuildStack rebuilds the stack tree from the current metadata.
// Use after mutations that change branch relationships.
func (rs *repoState) RebuildStack() error {
	s, err := stack.BuildStack(rs.Repo, rs.Config, rs.Metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}
	rs.Stack = s
	return nil
}
