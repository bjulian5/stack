package stack

import (
	"fmt"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
)

// GitOperationsForNavigation defines the git operations needed for navigation
type GitOperationsForNavigation interface {
	BranchExists(name string) bool
	GetCommitHash(ref string) (string, error)
	CheckoutBranch(name string) error
	ResetHard(ref string) error
	CreateAndCheckoutBranchAt(name string, commitHash string) error
}

// CheckoutChangeForEditing checks out a UUID branch for the given change, creating it if needed.
// If the branch already exists but points to a different commit, it syncs it to the current commit.
// Returns the branch name that was checked out.
func CheckoutChangeForEditing(
	gitClient GitOperationsForNavigation,
	stackCtx *StackContext,
	change *Change,
) (string, error) {
	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return "", fmt.Errorf("failed to get username: %w", err)
	}

	// Format UUID branch name
	branchName := stackCtx.FormatUUIDBranch(username, change.UUID)

	// Check if UUID branch already exists
	if gitClient.BranchExists(branchName) {
		// Get the commit hash the existing branch points to
		existingHash, err := gitClient.GetCommitHash(branchName)
		if err != nil {
			return "", fmt.Errorf("failed to get branch commit: %w", err)
		}

		// Checkout the branch first
		if err := gitClient.CheckoutBranch(branchName); err != nil {
			return "", fmt.Errorf("failed to checkout branch: %w", err)
		}

		// If branch is at wrong commit, sync it to the current commit location
		if existingHash != change.CommitHash {
			if err := gitClient.ResetHard(change.CommitHash); err != nil {
				return "", fmt.Errorf("failed to sync branch to current commit: %w", err)
			}
			fmt.Printf("⚠️  Synced branch to current commit (was at %s, now at %s)\n",
				git.ShortHash(existingHash), git.ShortHash(change.CommitHash))
		}
	} else {
		// Create and checkout new branch at the commit
		if err := gitClient.CreateAndCheckoutBranchAt(branchName, change.CommitHash); err != nil {
			return "", fmt.Errorf("failed to create branch: %w", err)
		}
	}

	return branchName, nil
}
