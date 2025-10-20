package hook

import (
	"fmt"
	"os"
	"strings"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// StackContext contains the context information for a stack operation
type StackContext struct {
	StackName   string
	StackBranch string
	UUIDBranch  string
}

// GetStackContext extracts stack context from the current state
func GetStackContext(g *git.Client, s *stack.Client, currentBranch string, branchUUID string) (*StackContext, error) {
	// Extract stack name and UUID from branch
	stackName, uuid := git.ExtractUUIDFromBranch(currentBranch)
	if stackName == "" || uuid == "" {
		return nil, fmt.Errorf("failed to parse UUID branch: %s", currentBranch)
	}

	if uuid != branchUUID {
		return nil, fmt.Errorf("branch UUID mismatch: expected %s, got %s", branchUUID, uuid)
	}

	// Get stack configuration
	stackConfig, err := s.LoadStack(stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack: %w", err)
	}

	return &StackContext{
		StackName:   stackName,
		StackBranch: stackConfig.Branch,
		UUIDBranch:  currentBranch,
	}, nil
}

// PostUpdateWorkflow performs the common post-update operations after modifying a stack
// This includes:
// 1. Updating commit tracking in prs.json
// 2. Updating all UUID branches to point to their new commit locations
// 3. Checking out the original UUID branch
func PostUpdateWorkflow(g *git.Client, s *stack.Client, ctx *StackContext) error {
	// Update commit tracking in prs.json
	if err := updateCommitTracking(g, s, ctx.StackName, ctx.StackBranch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update commit tracking: %v\n", err)
	}

	// Update ALL UUID branches for this stack
	if err := updateAllUUIDBranches(g, ctx.StackName, ctx.StackBranch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update UUID branches: %v\n", err)
	}

	// Checkout the original UUID branch (which now points to the correct location)
	if err := g.CheckoutBranch(ctx.UUIDBranch); err != nil {
		return fmt.Errorf("failed to checkout UUID branch: %w", err)
	}

	return nil
}

// updateCommitTracking updates the commit hash tracking in prs.json for all PRs in the stack
func updateCommitTracking(g *git.Client, s *stack.Client, stackName string, stackBranch string) error {
	// Load PRs
	prs, err := s.LoadPRs(stackName)
	if err != nil {
		return fmt.Errorf("failed to load PRs: %w", err)
	}

	// For each UUID in prs.json, find its current commit hash on the stack
	for uuid, pr := range prs {
		commit, err := g.FindCommitByTrailer(stackBranch, "PR-UUID", uuid)
		if err != nil {
			// Commit might have been deleted or not yet created
			continue
		}

		// Update the commit hash
		pr.CommitHash = commit.Hash
		prs[uuid] = pr
	}

	// Save updated PRs
	if err := s.SavePRs(stackName, prs); err != nil {
		return fmt.Errorf("failed to save PRs: %w", err)
	}

	return nil
}

// updateAllUUIDBranches finds and updates all UUID branches for this stack to point to their new commit locations
func updateAllUUIDBranches(g *git.Client, stackName string, stackBranch string) error {
	// Get all local branches
	branches, err := g.GetLocalBranches()
	if err != nil {
		return fmt.Errorf("failed to get branches: %w", err)
	}

	// Get username for branch prefix matching
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	// Construct the prefix for UUID branches in this stack
	prefix := fmt.Sprintf("%s/stack-%s/", username, stackName)

	// Find and update all UUID branches for this stack
	for _, branch := range branches {
		if !strings.HasPrefix(branch, prefix) {
			continue
		}

		// Check if it's a UUID branch (not the TOP branch)
		if !git.IsUUIDBranch(branch) {
			continue
		}

		// Extract UUID from branch name
		_, uuid := git.ExtractUUIDFromBranch(branch)
		if uuid == "" {
			continue
		}

		// Find where this commit is now on the stack
		commit, err := g.FindCommitByTrailer(stackBranch, "PR-UUID", uuid)
		if err != nil {
			// Branch might be stale or commit was deleted
			continue
		}

		// Update the UUID branch to point to the new commit location
		if err := g.UpdateRef(branch, commit.Hash); err != nil {
			return fmt.Errorf("failed to update branch %s: %w", branch, err)
		}
	}

	return nil
}
