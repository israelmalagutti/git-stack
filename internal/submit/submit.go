package submit

import (
	"fmt"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/provider"
)

// Opts configures a single branch submission.
type Opts struct {
	Branch string
	Parent string
	Draft  bool
	Title  string
	NoPush bool
}

// Result describes what happened after submitting a branch.
type Result struct {
	Branch   string
	Parent   string
	PRNumber int
	PRURL    string
	Action   string // "created" or "updated"
	Provider string
}

// Branch submits a single branch as a PR. It pushes the branch, creates or
// updates the PR, and stores the PR number in metadata. The caller is
// responsible for saving metadata and pushing refs afterward.
func Branch(repo *git.Repo, metadata *config.Metadata, prov provider.Provider, remote string, opts Opts) (*Result, error) {
	branch := opts.Branch
	parent := opts.Parent

	// 1. Push branch to remote
	if !opts.NoPush {
		if _, err := repo.RunGitCommand("push", "--force-with-lease", "-u", remote, branch); err != nil {
			return nil, fmt.Errorf("failed to push '%s': %w", branch, err)
		}
	}

	// 2. Check if PR already exists
	var result *provider.PRResult

	// First check metadata for stored PR number
	if pr := metadata.GetPR(branch); pr != nil {
		status, err := prov.GetPRStatus(pr.Number)
		if err == nil && status.State == "open" {
			result = &provider.PRResult{
				Number: pr.Number,
				URL:    status.URL,
				Action: "updated",
			}
		}
		// If PR is closed/merged or error, fall through to create
	}

	// If not found in metadata, check provider directly
	if result == nil {
		existing, err := prov.FindExistingPR(branch)
		if err == nil && existing != nil {
			result = existing
		}
	}

	// 3. Create PR if none exists
	if result == nil {
		title := opts.Title
		if title == "" {
			title = DerivePRTitle(repo, branch, parent)
		}

		created, err := prov.CreatePR(provider.PRCreateOpts{
			Base:  parent,
			Head:  branch,
			Title: title,
			Draft: opts.Draft,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create PR: %w", err)
		}
		result = created
	}

	// 4. Store PR number in metadata
	if err := metadata.SetPR(branch, &config.PRInfo{
		Number:   result.Number,
		Provider: prov.Name(),
	}); err != nil {
		return nil, fmt.Errorf("failed to store PR metadata: %w", err)
	}

	return &Result{
		Branch:   branch,
		Parent:   parent,
		PRNumber: result.Number,
		PRURL:    result.URL,
		Action:   result.Action,
		Provider: prov.Name(),
	}, nil
}

// DerivePRTitle generates a PR title from the branch's first commit message
// or the branch name as fallback.
func DerivePRTitle(repo *git.Repo, branch, parent string) string {
	output, err := repo.RunGitCommand("log", parent+".."+branch, "--format=%s", "-1")
	if err == nil {
		title := strings.TrimSpace(output)
		if title != "" {
			return title
		}
	}
	return HumanizeBranchName(branch)
}

// HumanizeBranchName converts a branch name to a human-readable title.
// "feat/auth-ui" -> "Auth ui"
func HumanizeBranchName(branch string) string {
	name := branch
	// Strip prefix (feat/, fix/, etc.)
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	// Replace dashes with spaces
	name = strings.ReplaceAll(name, "-", " ")
	// Title case first letter
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}
