package provider

import "fmt"

// GenericProvider is the fallback for unsupported hosting platforms.
// All PR operations return ErrNotSupported with helpful messages.
type GenericProvider struct {
	host string
}

// NewGenericProvider creates a GenericProvider for the given host.
func NewGenericProvider(host string) *GenericProvider {
	return &GenericProvider{host: host}
}

func (g *GenericProvider) Name() string          { return "generic" }
func (g *GenericProvider) CLIAvailable() bool     { return false }
func (g *GenericProvider) CLIAuthenticated() bool { return false }

func (g *GenericProvider) CreatePR(opts PRCreateOpts) (*PRResult, error) {
	return nil, fmt.Errorf("PR creation not supported for %s: %w — create the PR manually with base branch: %s",
		g.host, ErrNotSupported, opts.Base)
}

func (g *GenericProvider) UpdatePR(number int, opts PRUpdateOpts) error {
	return fmt.Errorf("PR update not supported for %s: %w", g.host, ErrNotSupported)
}

func (g *GenericProvider) GetPRStatus(number int) (*PRStatus, error) {
	return nil, fmt.Errorf("PR status not supported for %s: %w", g.host, ErrNotSupported)
}

func (g *GenericProvider) MergePR(number int, opts PRMergeOpts) error {
	return fmt.Errorf("PR merge not supported for %s: %w", g.host, ErrNotSupported)
}

func (g *GenericProvider) UpdatePRBase(number int, newBase string) error {
	return fmt.Errorf("PR base update not supported for %s: %w", g.host, ErrNotSupported)
}
