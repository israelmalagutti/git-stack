package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
)

// errTrunkOperation returns an error when a command is attempted on trunk.
func errTrunkOperation(op string) error {
	return fmt.Errorf("cannot %s trunk branch", op)
}

// errNotTracked returns an error when a branch is not tracked by gs.
func errNotTracked(branch string) error {
	return fmt.Errorf("branch '%s' is not tracked by gs", branch)
}

// validateNotTrunk checks that the branch is not trunk.
func validateNotTrunk(branch, trunk, op string) error {
	if branch == trunk {
		return errTrunkOperation(op)
	}
	return nil
}

// validateTracked checks that the branch is tracked by gs.
func validateTracked(metadata *config.Metadata, branch string) error {
	if !metadata.IsTracked(branch) {
		return errNotTracked(branch)
	}
	return nil
}

// validateNotTrunkAndTracked performs both trunk and tracked checks.
func validateNotTrunkAndTracked(metadata *config.Metadata, branch, trunk, op string) error {
	if err := validateNotTrunk(branch, trunk, op); err != nil {
		return err
	}
	return validateTracked(metadata, branch)
}
