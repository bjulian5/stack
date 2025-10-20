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
	if err != nil || !ctx.IsEditing() {
		// Not editing a UUID branch or error - exit silently
		return nil
	}

	stackName := ctx.StackName
	branchUUID := ctx.Change.UUID

	// Get current branch name for passing to handle functions
	currentBranch, err := c.Git.GetCurrentBranch()
	if err != nil {
		return nil // Exit silently
	}

	// Get the HEAD commit that was just created
	headCommit, err := c.Git.GetHEADCommit()
	if err != nil {
		return nil // Exit silently
	}

	// Check the commit's PR-UUID to determine if this is an amend or new commit
	commitUUID := headCommit.Trailers["PR-UUID"]
	isAmend := commitUUID == branchUUID

	// Get stack configuration
	stackConfig, err := c.Stack.LoadStack(stackName)
	if err != nil {
		// Stack not found - exit silently
		return nil
	}

	stackBranch := stackConfig.Branch

	// Perform the update
	if isAmend {
		if err := c.handleAmend(stackBranch, branchUUID, headCommit, currentBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating stack: %v\n", err)
			return nil // Don't fail the commit
		}
	} else {
		if err := c.handleInsert(stackBranch, branchUUID, headCommit, currentBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting commit into stack: %v\n", err)
			return nil // Don't fail the commit
		}
	}

	return nil
}

// handleAmend handles the case where the user amended an existing commit
func (c *PostCommitCommand) handleAmend(stackBranch string, uuid string, newCommit git.Commit, uuidBranch string) error {
	// Extract stack name from branch
	stackName := git.ExtractStackName(stackBranch)
	if stackName == "" {
		return fmt.Errorf("failed to extract stack name from branch: %s", stackBranch)
	}

	// Get stack context
	ctx := &StackContext{
		StackName:   stackName,
		StackBranch: stackBranch,
		UUIDBranch:  uuidBranch,
	}

	// Switch to stack branch
	if err := c.Git.CheckoutBranch(stackBranch); err != nil {
		return err
	}

	// Find the old commit with matching UUID in the stack
	oldCommit, err := c.Git.FindCommitByTrailer(stackBranch, "PR-UUID", uuid)
	if err != nil {
		return fmt.Errorf("commit with UUID %s not found in stack: %w", uuid, err)
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
	newCommitHash, err := c.Git.CommitTree(newTree, parentHash, newCommit.Message)
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

	// Perform post-update operations
	if err := PostUpdateWorkflow(c.Git, c.Stack, ctx); err != nil {
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

// handleInsert handles the case where the user created a new commit
func (c *PostCommitCommand) handleInsert(stackBranch string, branchUUID string, newCommit git.Commit, uuidBranch string) error {
	// Extract stack name from branch
	stackName := git.ExtractStackName(stackBranch)
	if stackName == "" {
		return fmt.Errorf("failed to extract stack name from branch: %s", stackBranch)
	}

	// Get stack context
	ctx := &StackContext{
		StackName:   stackName,
		StackBranch: stackBranch,
		UUIDBranch:  uuidBranch,
	}

	// The new commit doesn't have a UUID yet (or has a different one)
	// We need to add the UUID trailer if it's missing
	newCommitUUID := newCommit.Trailers["PR-UUID"]
	if newCommitUUID == "" {
		// Generate a new UUID for this commit
		newCommitUUID = common.GenerateUUID()

		// Switch to UUID branch and amend the commit to add the UUID
		newMessage := git.AddTrailer(newCommit.Message, "PR-UUID", newCommitUUID)
		newMessage = git.AddTrailer(newMessage, "PR-Stack", newCommit.Trailers["PR-Stack"])

		if err := c.Git.AmendCommitMessage(newMessage); err != nil {
			return fmt.Errorf("failed to add UUID to new commit: %w", err)
		}

		// Refresh the commit object
		newCommit, err := c.Git.GetHEADCommit()
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
	insertAfter, err := c.Git.FindCommitByTrailer(stackBranch, "PR-UUID", branchUUID)
	if err != nil {
		return fmt.Errorf("insertion point commit with UUID %s not found: %w", branchUUID, err)
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

	// Perform post-update operations
	if err := PostUpdateWorkflow(c.Git, c.Stack, ctx); err != nil {
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
