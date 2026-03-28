package provider

import "errors"

// Sentinel errors for provider operations.
var (
	ErrCLINotFound      = errors.New("required CLI tool is not installed")
	ErrNotAuthenticated = errors.New("CLI tool is not authenticated")
	ErrNotSupported     = errors.New("operation not supported by this provider")
)

// Provider defines the interface for interacting with a git hosting platform.
type Provider interface {
	// Name returns the provider identifier ("github", "gitlab", "generic").
	Name() string

	// CLIAvailable reports whether the required CLI tool is on PATH.
	CLIAvailable() bool

	// CLIAuthenticated reports whether the CLI tool is logged in.
	CLIAuthenticated() bool

	// CreatePR creates a pull/merge request.
	CreatePR(opts PRCreateOpts) (*PRResult, error)

	// UpdatePR updates an existing PR's title, body, or draft state.
	UpdatePR(number int, opts PRUpdateOpts) error

	// GetPRStatus fetches the current status of a PR.
	GetPRStatus(number int) (*PRStatus, error)

	// MergePR merges a PR.
	MergePR(number int, opts PRMergeOpts) error

	// UpdatePRBase changes the base branch of an existing PR.
	UpdatePRBase(number int, newBase string) error
}

// PRCreateOpts configures PR creation.
type PRCreateOpts struct {
	Base  string // target branch (e.g., "main")
	Head  string // source branch (e.g., "feat/auth")
	Title string
	Body  string
	Draft bool
}

// PRUpdateOpts configures PR updates. Nil fields are not changed.
type PRUpdateOpts struct {
	Title *string
	Body  *string
	Draft *bool
}

// PRMergeOpts configures PR merging.
type PRMergeOpts struct {
	Method string // "merge", "squash", "rebase"
	Auto   bool   // enable auto-merge (for merge queues)
}

// PRResult is returned after creating or updating a PR.
type PRResult struct {
	Number int
	URL    string
	Action string // "created" or "updated"
}

// PRStatus describes the current state of a PR.
type PRStatus struct {
	Number       int
	State        string // "open", "closed", "merged"
	Draft        bool
	Title        string
	URL          string
	ReviewStatus string // "approved", "changes_requested", "pending", ""
	CIStatus     string // "pass", "fail", "pending", ""
	Mergeable    bool
}
