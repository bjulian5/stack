package top

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command moves to the top of the stack (TOP branch)
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "top",
		Short: "Move to the top of the stack",
		Long: `Move to the top of the stack (TOP branch with all commits).

Can be used from any position in the stack to return to the main stack branch.

Example:
  stack top    # Move to TOP branch from any position`,
		Args: cobra.NoArgs,
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, _, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context())
		},
	}

	parent.AddCommand(command)
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

	topActiveChange := stackCtx.ActiveChanges[len(stackCtx.ActiveChanges)-1]

	// Validate UUID exists
	if topActiveChange.UUID == "" {
		return fmt.Errorf("cannot navigate to change: commit missing PR-UUID trailer")
	}

	// Checkout UUID branch for editing
	_, err = c.Stack.CheckoutChangeForEditing(stackCtx, topActiveChange)
	if err != nil {
		return err
	}

	// Warn if navigating to a merged change
	if c.Stack.IsChangeMerged(topActiveChange) {
		ui.Warningf(
			"Change #%d has been merged on GitHub - run 'stack refresh' to sync",
			topActiveChange.Position,
		)
	}

	// Print success message with stack tree
	ui.Print(ui.RenderNavigationSuccess(ui.NavigationSuccess{
		Message:     fmt.Sprintf("Moved to top of stack: %s", topActiveChange.Title),
		Stack:       stackCtx.Stack,
		Changes:     stackCtx.AllChanges,
		CurrentUUID: topActiveChange.UUID,
		IsEditing:   false,
	}))

	return nil
}
