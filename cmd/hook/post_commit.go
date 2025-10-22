package hook

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PostCommitCommand implements the post-commit hook
type PostCommitCommand struct {
	Git   *git.Client
	Stack *stack.Client
}

// Register registers the post-commit command
func (c *PostCommitCommand) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "post-commit",
		Short: "post-commit git hook",
		Long:  `Called by git after a commit is created.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	parent.AddCommand(cmd)
}

// Run executes the post-commit hook
func (c *PostCommitCommand) Run() error {
	// If a rebase is in progress, skip hook execution
	// This prevents the hook from running during interactive rebases
	if c.Git.IsRebaseInProgress() {
		return nil
	}

	// Get stack context
	ctx, err := c.Stack.GetStackContext()
	if err != nil {
		return nil
	}

	// If not a stack, nothing to do
	if !ctx.IsStack() {
		return nil
	}

	// If not editing (i.e., on TOP branch), check if this is an amend
	if !ctx.IsEditing() {
		// Get the HEAD commit to check if it's an amend
		headCommit, err := c.Git.GetCommit("HEAD")
		if err != nil {
			return nil
		}

		// Check if the commit's UUID already exists in the stack
		commitUUID := headCommit.Message.Trailers["PR-UUID"]
		if commitUUID != "" && ctx.FindChange(commitUUID) != nil {
			// This is an amend on the TOP branch - update UUID branches
			currentBranch, err := c.Git.GetCurrentBranch()
			if err != nil {
				return nil
			}

			if err := c.handleTopBranchAmend(ctx, currentBranch); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating stack after TOP branch amend: %v\n", err)
			}
		}
		return nil
	}

	// Get current branch name for passing to handle functions
	currentBranch, err := c.Git.GetCurrentBranch()
	if err != nil {
		return nil // Exit silently
	}

	// Get the HEAD commit that was just created
	headCommit, err := c.Git.GetCommit("HEAD")
	if err != nil {
		return nil // Exit silently
	}

	// Check the commit's PR-UUID to determine if this is an amend or new commit
	commitUUID := headCommit.Message.Trailers["PR-UUID"]
	isAmend := commitUUID == ctx.CurrentChange().UUID

	// Perform the update
	if isAmend {
		if err := c.handleAmend(ctx, currentBranch, headCommit); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating stack: %v\n", err)
			return nil // Don't fail the commit
		}
	} else {
		if err := c.handleInsert(ctx, currentBranch, headCommit); err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting commit into stack: %v\n", err)
			return nil // Don't fail the commit
		}
	}

	return nil
}

// handleAmend handles the case where the user amended an existing commit
func (c *PostCommitCommand) handleAmend(ctx *stack.StackContext, currentBranch string, newCommit git.Commit) error {
	stackBranch := ctx.Stack.Branch

	// Defensive check: ensure we're actually editing a change
	currentChange := ctx.CurrentChange()
	if currentChange == nil {
		return fmt.Errorf("not currently editing a change (not on UUID branch)")
	}
	uuid := currentChange.UUID

	// Switch to stack branch
	if err := c.Git.CheckoutBranch(stackBranch); err != nil {
		return err
	}

	// Find the old commit with matching UUID in the stack
	oldChange := ctx.FindChange(uuid)
	if oldChange == nil {
		return fmt.Errorf("commit with UUID %s not found in stack", uuid)
	}

	oldCommit, err := c.Git.GetCommit(oldChange.CommitHash)
	if err != nil {
		return fmt.Errorf("failed to get old commit: %w", err)
	}

	// Save the current HEAD of the stack branch (before we modify it)
	originalStackHead, err := c.Git.GetCommitHash(stackBranch)
	if err != nil {
		return fmt.Errorf("failed to get original stack HEAD: %w", err)
	}

	// Get commits that come AFTER the old commit (before we modify anything)
	commitsAfter, err := c.Git.GetCommits(stackBranch, oldCommit.Hash)
	if err != nil {
		return fmt.Errorf("failed to get commits after old commit: %w", err)
	}
	subsequentCount := len(commitsAfter)

	// Get the parent of the old commit
	parentHash, err := c.Git.GetParentCommit(oldCommit.Hash)
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Get the tree from the new amended commit
	newTree, err := c.Git.GetCommitTree(newCommit.Hash)
	if err != nil {
		return fmt.Errorf("failed to get tree from amended commit: %w", err)
	}

	// Create a new commit on the stack branch with the amended tree and message
	// This preserves all changes: tree, message, and trailers
	newCommitHash, err := c.Git.CommitTree(newTree, parentHash, newCommit.Message.String())
	if err != nil {
		return fmt.Errorf("failed to create commit with amended changes: %w", err)
	}

	// Reset stack branch to the new commit
	if err := c.Git.ResetHard(newCommitHash); err != nil {
		return err
	}

	// If there are subsequent commits, rebase them onto the new commit
	if subsequentCount > 0 {
		rebasedCount, err := c.Git.RebaseSubsequentCommits(stackBranch, oldCommit.Hash, newCommitHash, originalStackHead)
		if err != nil {
			return err
		}
		subsequentCount = rebasedCount
	}

	// Reload context to get fresh commit hashes after rebase
	// The rebase updated the TOP branch with new commit hashes, so we need to reload
	// the context to ensure UUID branches get updated with correct hashes
	ctx, err = c.Stack.GetStackContextByName(ctx.StackName)
	if err != nil {
		return fmt.Errorf("failed to reload stack context: %w", err)
	}

	// Perform post-update operations
	if err := PostUpdateWorkflow(c.Git, c.Stack, ctx, currentBranch); err != nil {
		return err
	}

	// Print success message
	if subsequentCount > 0 {
		fmt.Printf("✓ Updated commit and rebased %d subsequent commit(s)\n", subsequentCount)
	} else {
		fmt.Printf("✓ Updated commit\n")
	}

	return nil
}

// handleTopBranchAmend handles the case where the user amended a commit directly on the TOP branch
// This updates all UUID branches to point to their new commit locations after the amend
func (c *PostCommitCommand) handleTopBranchAmend(ctx *stack.StackContext, currentBranch string) error {
	// Reload context to get fresh commit hashes after the amend
	ctx, err := c.Stack.GetStackContextByName(ctx.StackName)
	if err != nil {
		return fmt.Errorf("failed to reload stack context: %w", err)
	}

	// Update all UUID branches to point to their new commit locations
	// This is the key fix - without this, UUID branches become stale after TOP branch amends
	if err := updateAllUUIDBranches(c.Git, ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update UUID branches: %v\n", err)
	}

	fmt.Printf("✓ Updated commit on TOP branch\n")
	return nil
}

// handleInsert handles the case where the user created a new commit
func (c *PostCommitCommand) handleInsert(ctx *stack.StackContext, currentBranch string, newCommit git.Commit) error {
	stackBranch := ctx.Stack.Branch
	branchUUID := ctx.CurrentChange().UUID

	// The new commit doesn't have a UUID yet (or has a different one)
	// We need to add the UUID trailer if it's missing
	newCommitUUID := newCommit.Message.Trailers["PR-UUID"]
	if newCommitUUID == "" {
		// Generate a new UUID for this commit
		newCommitUUID = common.GenerateUUID()

		// Switch to UUID branch and amend the commit to add the UUID
		newCommit.Message.AddTrailer("PR-UUID", newCommitUUID)
		newCommit.Message.AddTrailer("PR-Stack", newCommit.Message.Trailers["PR-Stack"])

		if err := c.Git.AmendCommitMessage(newCommit.Message.String()); err != nil {
			return fmt.Errorf("failed to add UUID to new commit: %w", err)
		}

		// Refresh the commit object
		newCommit, err := c.Git.GetCommit("HEAD")
		if err != nil {
			return err
		}
		_ = newCommit
	}

	// Switch to stack branch
	if err := c.Git.CheckoutBranch(stackBranch); err != nil {
		return err
	}

	// Save the current HEAD of the stack branch (before we modify it)
	originalStackHead, err := c.Git.GetCommitHash(stackBranch)
	if err != nil {
		return fmt.Errorf("failed to get original stack HEAD: %w", err)
	}

	// Find the commit with the branch UUID (the insertion point)
	insertAfterChange := ctx.FindChange(branchUUID)
	if insertAfterChange == nil {
		return fmt.Errorf("insertion point commit with UUID %s not found", branchUUID)
	}

	insertAfter, err := c.Git.GetCommit(insertAfterChange.CommitHash)
	if err != nil {
		return fmt.Errorf("failed to get insertion point commit: %w", err)
	}

	// Get all commits after the insertion point
	commitsAfter, err := c.Git.GetCommits(stackBranch, insertAfter.Hash)
	if err != nil {
		return err
	}
	subsequentCount := len(commitsAfter)

	// Reset to the insertion point
	if err := c.Git.ResetHard(insertAfter.Hash); err != nil {
		return err
	}

	// Cherry-pick the new commit onto the insertion point
	if err := c.Git.CherryPick(newCommit.Hash); err != nil {
		return fmt.Errorf("failed to insert commit: %w", err)
	}

	// Get the new commit hash (this is where the inserted commit ended up)
	newCommitHead, err := c.Git.GetCommitHash("HEAD")
	if err != nil {
		return err
	}

	// If there are commits after the insertion point, rebase them onto the new commit
	if subsequentCount > 0 {
		rebasedCount, err := c.Git.RebaseSubsequentCommits(stackBranch, insertAfter.Hash, newCommitHead, originalStackHead)
		if err != nil {
			return err
		}
		subsequentCount = rebasedCount
	}

	// Reload context to get fresh commit hashes after rebase
	// The rebase updated the TOP branch with new commit hashes, so we need to reload
	// the context to ensure UUID branches get updated with correct hashes
	ctx, err = c.Stack.GetStackContextByName(ctx.StackName)
	if err != nil {
		return fmt.Errorf("failed to reload stack context: %w", err)
	}

	// Perform post-update operations
	if err := PostUpdateWorkflow(c.Git, c.Stack, ctx, currentBranch); err != nil {
		return err
	}

	// Print success message
	if subsequentCount > 0 {
		fmt.Printf("✓ Inserted new commit and rebased %d subsequent commit(s)\n", subsequentCount)
	} else {
		fmt.Printf("✓ Inserted new commit\n")
	}

	return nil
}
