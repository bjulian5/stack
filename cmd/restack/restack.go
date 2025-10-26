package restack

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command rebases the stack on top of the base branch
type Command struct {
	Git   *git.Client
	Stack *stack.Client

	// Flags
	Fetch   bool
	Onto    string
	Recover bool
	Retry   bool
}

func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	ghClient := gh.NewClient()
	c.Stack = stack.NewClient(c.Git, ghClient)

	cmd := &cobra.Command{
		Use:   "restack",
		Short: "Rebase the stack on top of the latest base branch changes",
		Long: `Rebase the stack on top of the latest base branch changes.

By default, this command fetches from the remote and rebases your stack on top
of the updated base branch. This is the typical workflow for pulling in upstream
changes before pushing your stack.

Use --onto to move your stack to a different base branch (e.g., from main to develop).
When using --onto, fetching is NOT automatic - add --fetch if needed.

Use --recover to complete a rebase after resolving conflicts or to recover from an
aborted rebase. Use --recover --retry to automatically retry a failed rebase.

Examples:
  # Fetch and rebase on latest origin/main (most common)
  stack restack

  # Move stack to a different base branch
  stack restack --onto develop

  # Fetch first, then move to different base
  stack restack --onto develop --fetch

  # After resolving rebase conflicts
  git add resolved-file.txt
  git rebase --continue
  stack restack --recover

  # After aborting a rebase, retry it
  git rebase --abort
  stack restack --recover --retry`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.Fetch, "fetch", false, "Fetch from remote before rebasing")
	cmd.Flags().StringVar(&c.Onto, "onto", "", "Rebase stack onto a different base branch")
	cmd.Flags().BoolVar(&c.Recover, "recover", false, "Recover from a failed or aborted rebase")
	cmd.Flags().BoolVar(&c.Retry, "retry", false, "Retry the rebase (only valid with --recover)")

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	// Handle recovery mode
	if c.Recover {
		return c.runRecover()
	}

	// Validate --retry is only used with --recover
	if c.Retry {
		return fmt.Errorf("--retry can only be used with --recover")
	}

	// Normal restack logic
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack.")
	}

	if stackCtx.OnUUIDBranch() {
		return fmt.Errorf("cannot restack while editing a change. Commit or abort your changes first.")
	}

	hasChanges, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return err
	}
	if hasChanges {
		return fmt.Errorf("you have uncommitted changes. Commit or stash them before restacking.")
	}

	// Determine target base: use --onto if specified, otherwise current base
	targetBase := c.Onto
	fetch := c.Fetch

	if targetBase == "" {
		targetBase = stackCtx.Stack.Base
		fetch = true
	}

	ui.Info("Checking PR merge status on GitHub...")
	if _, err := c.Stack.SyncPRMetadata(stackCtx); err != nil {
		return fmt.Errorf("failed to sync PR metadata: %w", err)
	}

	opts := stack.RestackOptions{
		Onto:  targetBase,
		Fetch: fetch,
	}
	if err := c.Stack.Restack(stackCtx, opts); err != nil {
		return err
	}

	ui.Successf("Restacked on %s", targetBase)
	return nil
}

func (c *Command) runRecover() error {
	// Check if rebase is still in progress
	if c.Git.IsRebaseInProgress() {
		return fmt.Errorf("rebase is still in progress\n\n" +
			"Please resolve conflicts first:\n" +
			"  1. Resolve conflicts in your files\n" +
			"  2. git add <resolved-files>\n" +
			"  3. git rebase --continue\n" +
			"  4. stack restack --recover")
	}

	// Get stack context (works even in detached HEAD if we have state file)
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch")
	}

	stackName := stackCtx.StackName

	// Check if we have rebase state
	if !c.Stack.HasRebaseState(stackName) {
		return fmt.Errorf("no rebase state found\n\n" +
			"This command is only for recovering from failed rebase operations.\n" +
			"If you need to rebase your stack, use: stack restack")
	}

	// Load rebase state
	rebaseState, err := c.Stack.LoadRebaseState(stackName)
	if err != nil {
		return fmt.Errorf("failed to load rebase state: %w", err)
	}

	// Determine what scenario we're in based on current state
	currentBranch, err := c.Git.GetCurrentBranch()
	if err != nil {
		// Might be in detached HEAD after successful rebase
		return c.handleDetachedHeadRecovery(stackName, rebaseState)
	}

	// We're on a branch - check if it's the stack branch
	if !stack.IsStackBranch(currentBranch) {
		return fmt.Errorf("not on stack branch (on %s)", currentBranch)
	}

	// We're on the stack branch after abort - offer options
	return c.handleAbortRecovery(stackName, rebaseState, stackCtx)
}

func (c *Command) handleDetachedHeadRecovery(stackName string, rebaseState *stack.RebaseState) error {
	ui.Info("Detecting completed rebase...")

	// Get current HEAD (tip of rebased commits)
	newStackHead, err := c.Git.GetCommitHash("HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Update stack branch to point to HEAD
	if err := c.Git.UpdateRef(rebaseState.StackBranch, newStackHead); err != nil {
		return fmt.Errorf("failed to update stack branch: %w", err)
	}
	ui.Successf("Updated %s to %s", rebaseState.StackBranch, git.ShortHash(newStackHead))

	// Checkout the stack branch
	if err := c.Git.CheckoutBranch(rebaseState.StackBranch); err != nil {
		return fmt.Errorf("failed to checkout stack branch: %w", err)
	}
	ui.Successf("Checked out %s", rebaseState.StackBranch)

	// Reload context and update UUID branches
	if err := c.updateUUIDBranches(stackName); err != nil {
		return err
	}

	// Clear rebase state
	if err := c.Stack.ClearRebaseState(stackName); err != nil {
		ui.Warningf("failed to clear rebase state: %v", err)
	}

	ui.Success("Rebase recovery complete!")
	return nil
}

func (c *Command) handleAbortRecovery(stackName string, rebaseState *stack.RebaseState, stackCtx *stack.StackContext) error {
	ui.Info("Detected aborted rebase operation.")
	ui.Info("The amend was preserved, but subsequent commits need to be rebased.")

	// If --retry flag is set, skip prompts and retry immediately
	if c.Retry {
		ui.Info("Retrying rebase of subsequent commits...")
		return c.retryRebase(stackName, rebaseState, stackCtx)
	}

	// Prompt user for action
	ui.Println("Options:")
	ui.Println("  1. Retry rebase (recommended - keeps amend and reapplies commits)")
	ui.Println("  2. Restore to previous state (undo amend, recover all commits)")
	ui.Println("  3. Keep current state (lose subsequent commits - available in reflog)")
	ui.Println("")
	ui.Printf("Choose [1/2/3]: ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1", "":
		// Default to retry
		return c.retryRebase(stackName, rebaseState, stackCtx)
	case "2":
		return c.restorePreviousState(stackName, rebaseState, stackCtx)
	case "3":
		return c.keepCurrentState(stackName, stackCtx)
	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}
}

func (c *Command) retryRebase(stackName string, rebaseState *stack.RebaseState, stackCtx *stack.StackContext) error {
	ui.Info("Retrying rebase...")

	// Call RebaseSubsequentCommits with the saved state
	rebasedCount, err := c.Git.RebaseSubsequentCommits(
		rebaseState.StackBranch,
		rebaseState.OldCommitHash,
		rebaseState.NewCommitHash,
		rebaseState.OriginalStackHead,
	)
	if err != nil {
		// Rebase failed again - state is already saved, user can retry again
		return fmt.Errorf("rebase failed again: %w\n\n" +
			"After resolving conflicts:\n" +
			"  git add <resolved-files>\n" +
			"  git rebase --continue\n" +
			"  stack restack --recover")
	}

	ui.Successf("Successfully rebased %d commit(s)", rebasedCount)

	// Update UUID branches
	if err := c.updateUUIDBranches(stackName); err != nil {
		return err
	}

	// Clear rebase state
	if err := c.Stack.ClearRebaseState(stackName); err != nil {
		ui.Warningf("failed to clear rebase state: %v", err)
	}

	ui.Success("Rebase completed!")
	return nil
}

func (c *Command) restorePreviousState(stackName string, rebaseState *stack.RebaseState, stackCtx *stack.StackContext) error {
	ui.Infof("Restoring to previous state (%s)...", git.ShortHash(rebaseState.OriginalStackHead))

	// Reset stack branch to original head
	if err := c.Git.ResetHard(rebaseState.OriginalStackHead); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	ui.Successf("Reset %s to %s (pre-amend state)", rebaseState.StackBranch, git.ShortHash(rebaseState.OriginalStackHead))

	// Update UUID branches
	if err := c.updateUUIDBranches(stackName); err != nil {
		return err
	}

	// Clear rebase state
	if err := c.Stack.ClearRebaseState(stackName); err != nil {
		ui.Warningf("failed to clear rebase state: %v", err)
	}

	ui.Success("Stack restored. Your amend has been undone.")
	return nil
}

func (c *Command) keepCurrentState(stackName string, stackCtx *stack.StackContext) error {
	ui.Info("Keeping current state...")

	// Update UUID branches for current state
	if err := c.updateUUIDBranches(stackName); err != nil {
		return err
	}

	// Clear rebase state
	if err := c.Stack.ClearRebaseState(stackName); err != nil {
		ui.Warningf("failed to clear rebase state: %w", err)
	}

	ui.Success("Cleared rebase state")
	ui.Success("Updated UUID branches for current state")
	ui.Println("")
	ui.Warning("Subsequent commits are orphaned. Use git reflog to recover if needed.")
	return nil
}

func (c *Command) updateUUIDBranches(stackName string) error {
	updatedCount, err := c.Stack.UpdateUUIDBranches(stackName)
	if err != nil {
		return fmt.Errorf("failed to update UUID branches: %w", err)
	}

	if updatedCount > 0 {
		ui.Successf("Updated %d UUID branch(es)", updatedCount)
	}

	return nil
}
