package fixup

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command creates a fixup commit for a selected change in the stack
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.GH = gh.NewClient()
	c.Stack = stack.NewClient(c.Git, c.GH)

	cmd := &cobra.Command{
		Use:   "fixup",
		Short: "Create a fixup commit for a change in the stack",
		Long: `Interactively select a change to fixup using a fuzzy finder.

Creates a fixup commit for the selected change and automatically squashes it using an autosquash rebase.
This is similar to 'git commit --fixup <commit> && git rebase -i --autosquash <commit>^' but with
an interactive fuzzy finder for change selection.

Requirements:
- Must be on a stack TOP branch
- Must have staged changes (use 'git add' first)

After successful rebase, you will remain on the TOP branch.

Example:
  git add .
  stack fixup`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Check if rebase is already in progress
	if c.Git.IsRebaseInProgress() {
		return fmt.Errorf("rebase already in progress: resolve conflicts and run 'git rebase --continue' or abort with 'git rebase --abort'")
	}

	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Validate we're in a stack and not editing
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	if stackCtx.OnUUIDBranch() {
		return fmt.Errorf("cannot run fixup while editing a change: checkout the stack TOP branch first")
	}

	// Sync metadata with GitHub (read-only, no git operations)
	stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Check for staged changes - this is required
	hasStaged, err := c.Git.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}
	if !hasStaged {
		return fmt.Errorf("no staged changes detected: stage your changes with 'git add' before running fixup")
	}

	// Validate stack has active changes
	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("no active changes to fixup: all changes are merged")
	}

	// Use fuzzy finder to select a change to fixup
	selectedChange, err := ui.SelectChange(stackCtx.ActiveChanges)
	if err != nil {
		return err
	}
	if selectedChange == nil {
		// User cancelled
		return nil
	}

	// Error if trying to fixup a merged change
	if c.Stack.IsChangeMerged(selectedChange) {
		return fmt.Errorf(
			"cannot fixup change #%d - it has been merged on GitHub\nRun 'stack refresh' to sync your stack",
			selectedChange.Position,
		)
	}

	// Validate the change has a commit hash
	if selectedChange.CommitHash == "" {
		return fmt.Errorf("cannot fixup change #%d: missing commit hash", selectedChange.Position)
	}

	// Create fixup commit
	ui.Infof("Creating fixup commit for: %s", selectedChange.Title)
	if err := c.Git.CommitFixup(selectedChange.CommitHash); err != nil {
		return err
	}

	// Get parent of the target commit for rebase
	parentHash, err := c.Git.GetParentCommit(selectedChange.CommitHash)
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Run interactive rebase with autosquash
	ui.Info("Running autosquash rebase...")
	if err := c.Git.RebaseInteractiveAutosquash(parentHash); err != nil {
		return err
	}

	// Success message
	ui.Successf("Successfully fixed up change #%d: %s", selectedChange.Position, selectedChange.Title)
	ui.Info("You are now on the TOP branch with the updated stack.")

	return nil
}
