package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// EncodeBranchRef encodes a branch name for use in a ref path.
// Slashes in branch names are replaced with "--" to avoid git ref
// directory conflicts (e.g., "feat/auth" cannot coexist with "feat/auth/ui"
// as refs because one would be both a file and a directory).
func EncodeBranchRef(branch string) string {
	return strings.ReplaceAll(branch, "/", "--")
}

// DecodeBranchRef decodes a ref-encoded branch name back to the original.
func DecodeBranchRef(encoded string) string {
	return strings.ReplaceAll(encoded, "--", "/")
}

// WriteRef stores data as a blob in git's object database and points a ref at it.
// The refName should be relative to refs/gs/ (e.g., "config" becomes "refs/gs/config").
func (r *Repo) WriteRef(refName string, data []byte) error {
	fullRef := "refs/gs/" + refName

	// Store the data as a blob
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(string(data))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to hash-object: %w\n%s", err, string(output))
	}
	sha := strings.TrimSpace(string(output))

	// Point the ref at the blob
	_, err = r.RunGitCommand("update-ref", fullRef, sha)
	if err != nil {
		return fmt.Errorf("failed to update-ref %s: %w", fullRef, err)
	}

	return nil
}

// ReadRef reads the blob content pointed to by a ref.
// The refName should be relative to refs/gs/ (e.g., "config").
func (r *Repo) ReadRef(refName string) ([]byte, error) {
	fullRef := "refs/gs/" + refName

	output, err := r.RunGitCommand("cat-file", "-p", fullRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read ref %s: %w", fullRef, err)
	}

	return []byte(output), nil
}

// RefExists checks if a ref exists under refs/gs/.
func (r *Repo) RefExists(refName string) bool {
	fullRef := "refs/gs/" + refName
	_, err := r.RunGitCommand("rev-parse", "--verify", fullRef)
	return err == nil
}

// DeleteRef deletes a ref under refs/gs/.
func (r *Repo) DeleteRef(refName string) error {
	fullRef := "refs/gs/" + refName

	if !r.RefExists(refName) {
		return fmt.Errorf("ref %s does not exist", fullRef)
	}

	_, err := r.RunGitCommand("update-ref", "-d", fullRef)
	if err != nil {
		return fmt.Errorf("failed to delete ref %s: %w", fullRef, err)
	}

	return nil
}

// ListRefs lists all refs under refs/gs/<prefix> and returns their names
// relative to refs/gs/.
func (r *Repo) ListRefs(prefix string) ([]string, error) {
	fullPrefix := "refs/gs/" + prefix

	output, err := r.RunGitCommand("for-each-ref", "--format=%(refname)", fullPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list refs under %s: %w", fullPrefix, err)
	}

	if output == "" {
		return []string{}, nil
	}

	var refs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip the "refs/gs/" prefix to return relative names
		ref := strings.TrimPrefix(line, "refs/gs/")
		refs = append(refs, ref)
	}

	return refs, nil
}

// HasRemote checks if a named remote exists in the repository.
func (r *Repo) HasRemote(remote string) bool {
	output, err := r.RunGitCommand("remote")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == remote {
			return true
		}
	}
	return false
}

// HasRefspec checks if a fetch refspec is already configured for a remote.
func (r *Repo) HasRefspec(remote, refspec string) (bool, error) {
	output, err := r.RunGitCommand("config", "--get-all", fmt.Sprintf("remote.%s.fetch", remote))
	if err != nil {
		// No fetch config at all
		return false, nil
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == refspec {
			return true, nil
		}
	}
	return false, nil
}

// ConfigureRefspec adds a fetch refspec to .git/config for a remote if not already present.
func (r *Repo) ConfigureRefspec(remote, refspec string) error {
	has, err := r.HasRefspec(remote, refspec)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = r.RunGitCommand("config", "--add", fmt.Sprintf("remote.%s.fetch", remote), refspec)
	if err != nil {
		return fmt.Errorf("failed to add refspec %s for %s: %w", refspec, remote, err)
	}
	return nil
}

// PushRefs pushes one or more refspecs to a remote.
// Each refspec should be a full refspec like "refs/gs/meta/feat--auth:refs/gs/meta/feat--auth".
func (r *Repo) PushRefs(remote string, refspecs ...string) error {
	args := []string{"push", remote}
	args = append(args, refspecs...)
	_, err := r.RunGitCommand(args...)
	if err != nil {
		return fmt.Errorf("failed to push refs to %s: %w", remote, err)
	}
	return nil
}

// FetchRefs fetches one or more refspecs from a remote.
func (r *Repo) FetchRefs(remote string, refspecs ...string) error {
	args := []string{"fetch", remote}
	args = append(args, refspecs...)
	_, err := r.RunGitCommand(args...)
	if err != nil {
		return fmt.Errorf("failed to fetch refs from %s: %w", remote, err)
	}
	return nil
}

// DeleteRemoteRef deletes a ref on the remote by pushing an empty source.
// The refName should be relative to refs/gs/ (e.g., "meta/feat--auth").
func (r *Repo) DeleteRemoteRef(remote, refName string) error {
	fullRef := "refs/gs/" + refName
	_, err := r.RunGitCommand("push", remote, "--delete", fullRef)
	if err != nil {
		return fmt.Errorf("failed to delete remote ref %s: %w", fullRef, err)
	}
	return nil
}
