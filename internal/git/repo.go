package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetContinueStatePath returns the path to gs continue state file
func (r *Repo) GetContinueStatePath() string {
	return filepath.Join(r.commonDir, ".gs_continue_state")
}

// Repo represents a git repository
type Repo struct {
	workDir   string
	gitDir    string
	commonDir string
}

// NewRepo creates a new Repo instance and validates it's a git repository
func NewRepo() (*Repo, error) {
	// Check if we're in a git repository
	if !IsGitRepo() {
		return nil, fmt.Errorf("not a git repository (or any of the parent directories)")
	}

	repo := &Repo{}

	// Get git directory (handles worktrees)
	gitDir, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git directory: %w", err)
	}
	repo.gitDir = strings.TrimSpace(string(gitDir))

	// Get common git directory (shared across worktrees)
	commonDir, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git common directory: %w", err)
	}
	repo.commonDir = strings.TrimSpace(string(commonDir))

	// Get working directory
	workDir, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	repo.workDir = strings.TrimSpace(string(workDir))

	return repo, nil
}

// IsGitRepo checks if the current directory is inside a git repository
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// GetCommonDir returns the common git directory (shared across worktrees)
func (r *Repo) GetCommonDir() string {
	return r.commonDir
}

// GetGitDir returns the git directory
func (r *Repo) GetGitDir() string {
	return r.gitDir
}

// GetWorkDir returns the working directory
func (r *Repo) GetWorkDir() string {
	return r.workDir
}

// GetConfigPath returns the path to gs config file
func (r *Repo) GetConfigPath() string {
	return filepath.Join(r.commonDir, ".gs_config")
}

// GetMetadataPath returns the path to gs metadata file
func (r *Repo) GetMetadataPath() string {
	return filepath.Join(r.commonDir, ".gs_stack_metadata")
}

// RunGitCommand executes a git command and returns output
func (r *Repo) RunGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// RunGitCommandWithStdin executes a git command with stdin data and returns output
func (r *Repo) RunGitCommandWithStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(stdin)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
