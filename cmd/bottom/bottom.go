package bottom

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command moves to the bottom of the stack (first change)
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
		Use:   "bottom",
		Short: "Move to the bottom of the stack",
		Long: `Move to the bottom of the stack (first change at position 1).

Can be used from any position in the stack to navigate to the first change.

Example:
  stack bottom    # Move to position 1 from any position`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Check for uncommitted changes before switching branches
	hasUncommitted, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if hasUncommitted {
		return fmt.Errorf("uncommitted changes detected: commit or stash your changes before navigating")
	}

	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Validate we're in a stack
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	// Sync metadata with GitHub (read-only, no git operations)
	stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Validate stack has active changes
	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("no active changes in stack: all changes are merged")
	}

	bottomActiveChange := stackCtx.ActiveChanges[0]

	// Validate UUID exists
	if bottomActiveChange.UUID == "" {
		return fmt.Errorf("cannot navigate to change #1: commit missing PR-UUID trailer")
	}

	// Checkout UUID branch for editing
	_, err = c.Stack.CheckoutChangeForEditing(stackCtx, bottomActiveChange)
	if err != nil {
		return err
	}

	// Warn if navigating to a merged change
	if c.Stack.IsChangeMerged(bottomActiveChange) {
		ui.Warningf(
			"Change #%d has been merged on GitHub - run 'stack refresh' to sync",
			bottomActiveChange.Position,
		)
	}

	// Print success message with stack tree
	ui.Print(ui.RenderNavigationSuccess(ui.NavigationSuccess{
		Message:     fmt.Sprintf("Moved to change #%d: %s", bottomActiveChange.Position, bottomActiveChange.Title),
		Stack:       stackCtx.Stack,
		Changes:     stackCtx.AllChanges,
		CurrentUUID: stackCtx.GetCurrentPositionUUID(),
		IsEditing:   true,
	}))

	return nil
}
