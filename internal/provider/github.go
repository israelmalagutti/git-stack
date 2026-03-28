package provider

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GitHubProvider implements Provider using the gh CLI.
type GitHubProvider struct {
	host  string // e.g., "github.com" or GHE hostname
	owner string
	repo  string
}

// NewGitHubProvider creates a GitHubProvider for the given host/owner/repo.
func NewGitHubProvider(host, owner, repo string) *GitHubProvider {
	return &GitHubProvider{host: host, owner: owner, repo: repo}
}

func (g *GitHubProvider) Name() string { return "github" }

func (g *GitHubProvider) CLIAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (g *GitHubProvider) CLIAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status", "--hostname", g.host)
	return cmd.Run() == nil
}

func (g *GitHubProvider) repoFlag() string {
	return g.owner + "/" + g.repo
}

// runGH executes a gh command with --repo prepended and returns stdout.
func (g *GitHubProvider) runGH(args ...string) (string, error) {
	fullArgs := append([]string{}, args...)

	cmd := exec.Command("gh", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (g *GitHubProvider) CreatePR(opts PRCreateOpts) (*PRResult, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("%w: install gh from https://cli.github.com/", ErrCLINotFound)
	}
	if !g.CLIAuthenticated() {
		return nil, fmt.Errorf("%w: run 'gh auth login'", ErrNotAuthenticated)
	}

	args := []string{"pr", "create",
		"--repo", g.repoFlag(),
		"--base", opts.Base,
		"--head", opts.Head,
		"--title", opts.Title,
	}

	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	} else {
		args = append(args, "--body", "")
	}

	if opts.Draft {
		args = append(args, "--draft")
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// gh pr create outputs the PR URL
	prURL := strings.TrimSpace(output)
	number := extractPRNumber(prURL)

	return &PRResult{
		Number: number,
		URL:    prURL,
		Action: "created",
	}, nil
}

func (g *GitHubProvider) UpdatePR(number int, opts PRUpdateOpts) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("%w: install gh from https://cli.github.com/", ErrCLINotFound)
	}

	args := []string{"pr", "edit", strconv.Itoa(number), "--repo", g.repoFlag()}

	if opts.Title != nil {
		args = append(args, "--title", *opts.Title)
	}
	if opts.Body != nil {
		args = append(args, "--body", *opts.Body)
	}

	_, err := g.runGH(args...)
	return err
}

func (g *GitHubProvider) GetPRStatus(number int) (*PRStatus, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("%w: install gh from https://cli.github.com/", ErrCLINotFound)
	}

	output, err := g.runGH("pr", "view", strconv.Itoa(number),
		"--repo", g.repoFlag(),
		"--json", "number,state,title,url,isDraft,reviewDecision,statusCheckRollup,mergeable",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR status: %w", err)
	}

	var raw struct {
		Number            int              `json:"number"`
		State             string           `json:"state"`
		Title             string           `json:"title"`
		URL               string           `json:"url"`
		IsDraft           bool             `json:"isDraft"`
		ReviewDecision    string           `json:"reviewDecision"`
		Mergeable         string           `json:"mergeable"`
		StatusCheckRollup []ghCheckStatus  `json:"statusCheckRollup"`
	}

	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse PR status: %w", err)
	}

	status := &PRStatus{
		Number:       raw.Number,
		State:        strings.ToLower(raw.State),
		Draft:        raw.IsDraft,
		Title:        raw.Title,
		URL:          raw.URL,
		ReviewStatus: mapReviewDecision(raw.ReviewDecision),
		CIStatus:     mapCIStatus(raw.StatusCheckRollup),
		Mergeable:    raw.Mergeable == "MERGEABLE",
	}

	return status, nil
}

func (g *GitHubProvider) MergePR(number int, opts PRMergeOpts) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("%w: install gh from https://cli.github.com/", ErrCLINotFound)
	}

	args := []string{"pr", "merge", strconv.Itoa(number), "--repo", g.repoFlag()}

	switch opts.Method {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}

	if opts.Auto {
		args = append(args, "--auto")
	}

	_, err := g.runGH(args...)
	return err
}

func (g *GitHubProvider) UpdatePRBase(number int, newBase string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("%w: install gh from https://cli.github.com/", ErrCLINotFound)
	}

	_, err := g.runGH("pr", "edit", strconv.Itoa(number),
		"--repo", g.repoFlag(),
		"--base", newBase,
	)
	return err
}

// extractPRNumber parses a PR number from a GitHub PR URL.
func extractPRNumber(prURL string) int {
	parts := strings.Split(prURL, "/")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func mapReviewDecision(decision string) string {
	switch strings.ToUpper(decision) {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	case "REVIEW_REQUIRED":
		return "pending"
	default:
		return ""
	}
}

type ghCheckStatus struct {
	State string `json:"state"`
}

func mapCIStatus(checks []ghCheckStatus) string {
	if len(checks) == 0 {
		return ""
	}
	allPass := true
	for _, c := range checks {
		switch strings.ToUpper(c.State) {
		case "FAILURE", "ERROR":
			return "fail"
		case "PENDING", "QUEUED", "IN_PROGRESS":
			allPass = false
		}
	}
	if allPass {
		return "pass"
	}
	return "pending"
}
