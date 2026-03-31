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

// NewRepo creates a new Repo instance from the current working directory.
func NewRepo() (*Repo, error) {
	return NewRepoAt("")
}

// NewRepoAt creates a new Repo instance rooted at the given directory.
// If dir is empty, the current working directory is used.
func NewRepoAt(dir string) (*Repo, error) {
	gitCmd := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		if dir != "" {
			cmd.Dir = dir
		}
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	gitDir, err := gitCmd("rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or any of the parent directories)")
	}

	commonDir, err := gitCmd("rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("failed to get git common directory: %w", err)
	}

	workDir, err := gitCmd("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Resolve relative paths when dir is provided (rev-parse returns
	// relative paths like ".git" when run via cmd.Dir)
	if dir != "" {
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(dir, gitDir)
		}
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(dir, commonDir)
		}
	}

	return &Repo{
		workDir:   workDir,
		gitDir:    gitDir,
		commonDir: commonDir,
	}, nil
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

// RunGitCommand executes a git command in the repo's working directory and returns output
func (r *Repo) RunGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRemoteURL returns the URL configured for a named remote.
func (r *Repo) GetRemoteURL(remote string) (string, error) {
	output, err := r.RunGitCommand("remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("failed to get URL for remote %s: %w", remote, err)
	}
	return output, nil
}

// RunGitCommandWithStdin executes a git command with stdin data in the repo's working directory
func (r *Repo) RunGitCommandWithStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workDir
	cmd.Stdin = strings.NewReader(stdin)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
