package hook

import (
	"fmt"
	"os"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PostUpdateWorkflow performs the common post-update operations after modifying a stack
// This includes:
// 1. Updating commit tracking in prs.json
// 2. Updating all UUID branches to point to their new commit locations
// 3. Checking out the original UUID branch
func PostUpdateWorkflow(g *git.Client, s *stack.Client, ctx *stack.StackContext, returnBranch string) error {
	// Update commit tracking in prs.json
	if err := updateCommitTracking(g, s, ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update commit tracking: %v\n", err)
	}

	// Update ALL UUID branches for this stack
	if err := updateAllUUIDBranches(g, ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update UUID branches: %v\n", err)
	}

	// Checkout the original UUID branch (which now points to the correct location)
	if err := g.CheckoutBranch(returnBranch); err != nil {
		return fmt.Errorf("failed to checkout UUID branch: %w", err)
	}

	return nil
}

// updateCommitTracking is a no-op now - we don't update commit hashes in prs.json here.
// Commit hashes should only be updated when we push to GitHub via SyncPRFromGitHub().
// This allows us to detect when local commits differ from what's on GitHub (needs push).
func updateCommitTracking(g *git.Client, s *stack.Client, ctx *stack.StackContext) error {
	// Note: We intentionally do NOT update commit hashes here.
	// The commit hash in PR metadata represents what's on GitHub, not what's local.
	// It gets updated only during `stack push` via SyncPRFromGitHub().
	return nil
}

// updateAllUUIDBranches finds and updates all UUID branches for this stack to point to their new commit locations
func updateAllUUIDBranches(g *git.Client, ctx *stack.StackContext) error {
	// Get username for branch name construction
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	// Iterate through active changes and update any corresponding UUID branches
	// Only active changes can have UUID branches (merged changes don't)
	for i := range ctx.ActiveChanges {
		change := &ctx.ActiveChanges[i]

		// Skip changes without a UUID (shouldn't happen in normal operation)
		if change.UUID == "" {
			continue
		}

		// Construct the expected branch name for this change
		branchName := ctx.FormatUUIDBranch(username, change.UUID)

		// Check if this UUID branch exists
		if !g.BranchExists(branchName) {
			// Branch doesn't exist yet (not checked out), skip it
			continue
		}

		// Update the UUID branch to point to the new commit location
		if err := g.UpdateRef(branchName, change.CommitHash); err != nil {
			return fmt.Errorf("failed to update branch %s: %w", branchName, err)
		}
	}

	return nil
}
