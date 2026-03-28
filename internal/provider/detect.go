package provider

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRemoteURL extracts the host, owner, and repo from a git remote URL.
// Supports HTTPS, SSH (git@host:owner/repo), and ssh:// schemes.
func ParseRemoteURL(rawURL string) (host, owner, repo string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", "", fmt.Errorf("empty remote URL")
	}

	// Handle SSH shorthand: git@github.com:owner/repo.git
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") && !strings.Contains(rawURL, "://") {
		// git@github.com:owner/repo.git
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("invalid SSH remote URL: %s", rawURL)
		}
		hostPart := parts[0]
		pathPart := parts[1]

		// Extract host from user@host
		if idx := strings.Index(hostPart, "@"); idx >= 0 {
			host = hostPart[idx+1:]
		} else {
			host = hostPart
		}

		owner, repo = parseOwnerRepo(pathPart)
		if owner == "" || repo == "" {
			return "", "", "", fmt.Errorf("could not parse owner/repo from: %s", rawURL)
		}
		return host, owner, repo, nil
	}

	// Handle HTTPS and ssh:// URLs
	u, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("invalid remote URL: %w", parseErr)
	}

	host = u.Hostname()
	if host == "" {
		return "", "", "", fmt.Errorf("no host in remote URL: %s", rawURL)
	}

	owner, repo = parseOwnerRepo(u.Path)
	if owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("could not parse owner/repo from: %s", rawURL)
	}

	return host, owner, repo, nil
}

// parseOwnerRepo extracts owner and repo from a path like "/owner/repo.git" or "owner/repo.git".
func parseOwnerRepo(path string) (owner, repo string) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// DetectFromRemoteURL returns the appropriate Provider for a given remote URL.
func DetectFromRemoteURL(remoteURL string) (Provider, error) {
	host, owner, repo, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote URL: %w", err)
	}

	hostLower := strings.ToLower(host)

	if strings.Contains(hostLower, "github") {
		return NewGitHubProvider(host, owner, repo), nil
	}

	// GitLab, Bitbucket, etc. — fall back to generic for now
	return NewGenericProvider(host), nil
}
