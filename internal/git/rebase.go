package git

import "fmt"

// RebaseSubsequentCommits rebases commits that come after oldCommitHash onto newCommitHash
// This is a common operation when updating a commit in the middle of a stack.
//
// It performs the following steps:
// 1. Rebases commits from oldCommitHash (exclusive) to originalStackHead (inclusive) onto newCommitHash
// 2. Updates the stack branch reference to point to the new HEAD
// 3. Checks out the stack branch
//
// Returns the number of rebased commits and any error encountered.
func (c *Client) RebaseSubsequentCommits(stackBranch string, oldCommitHash string, newCommitHash string, originalStackHead string) (int, error) {
	// Use git rebase --onto to rebase subsequent commits
	// This rebases commits from oldCommitHash (exclusive) to originalStackHead (inclusive) onto newCommitHash
	if err := c.RebaseOnto(newCommitHash, oldCommitHash, originalStackHead); err != nil {
		return 0, fmt.Errorf("rebase conflicts detected.\n\n"+
			"To resolve:\n"+
			"  1. Resolve conflicts in your files\n"+
			"  2. git add <resolved-files>\n"+
			"  3. git rebase --continue\n"+
			"  4. stack restack --recover\n\n"+
			"To abort and retry:\n"+
			"  1. git rebase --abort\n"+
			"  2. stack restack --recover --retry\n\n"+
			"Error: %w", err)
	}

	// After rebase, git leaves us in detached HEAD state
	// Capture the new HEAD (this is the tip of the rebased stack)
	newStackHead, err := c.GetCommitHash("HEAD")
	if err != nil {
		return 0, fmt.Errorf("failed to get HEAD after rebase: %w", err)
	}

	// Update the stack branch reference to point to the new HEAD
	if err := c.UpdateRef(stackBranch, newStackHead); err != nil {
		return 0, fmt.Errorf("failed to update stack branch: %w", err)
	}

	// Checkout the stack branch (now it's at the right place)
	if err := c.CheckoutBranch(stackBranch); err != nil {
		return 0, err
	}

	// Count how many commits were rebased
	// Get commits between newCommitHash and newStackHead
	commits, err := c.GetCommits(newStackHead, newCommitHash)
	if err != nil {
		return 0, fmt.Errorf("failed to count rebased commits: %w", err)
	}

	return len(commits), nil
}
