package down

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command moves down to the previous change in the stack
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
		Use:   "down",
		Short: "Move down to the previous change in the stack",
		Long: `Move down to the previous change (lower position) in the stack.

Can be used from the TOP branch to start navigating, or from a UUID branch to move down one position.

Example:
  stack down    # From TOP: move to position N-1
  stack down    # From UUID branch: move to previous position`,
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

	// Determine target position based on current state
	var targetPosition int

	if stackCtx.OnUUIDBranch() {
		// On UUID branch - move down from current position
		currentChange := stackCtx.CurrentChange()
		if currentChange == nil {
			return fmt.Errorf("failed to determine current change")
		}
		targetPosition = currentChange.Position - 1

		// Check if already at bottom
		if targetPosition < 1 {
			return fmt.Errorf("already at bottom position")
		}
	} else {
		// On TOP branch - move to N-1
		targetPosition = len(stackCtx.ActiveChanges) - 1

		// Check if stack has at least 2 changes
		if targetPosition < 1 {
			return fmt.Errorf("stack only has one change: already at bottom")
		}
	}

	// Get target change by index (Position is 1-indexed)
	targetChange := &stackCtx.ActiveChanges[targetPosition-1]

	// Validate UUID exists
	if targetChange.UUID == "" {
		return fmt.Errorf("cannot navigate to change #%d: commit missing PR-UUID trailer", targetPosition)
	}

	// Checkout UUID branch for editing
	_, err = c.Stack.CheckoutChangeForEditing(stackCtx, targetChange)
	if err != nil {
		return err
	}

	// Warn if navigating to a merged change
	if c.Stack.IsChangeMerged(targetChange) {
		ui.Warningf(
			"Change #%d has been merged on GitHub - run 'stack refresh' to sync",
			targetChange.Position,
		)
	}

	// Get updated context (now on UUID branch)
	stackCtx, err = c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get updated stack context: %w", err)
	}

	// Print success message with stack tree
	ui.Print(ui.RenderNavigationSuccess(ui.NavigationSuccess{
		Message:     fmt.Sprintf("Moved to change #%d: %s", targetChange.Position, targetChange.Title),
		Stack:       stackCtx.Stack,
		Changes:     stackCtx.AllChanges,
		CurrentUUID: stackCtx.GetCurrentPositionUUID(),
		IsEditing:   true,
	}))

	return nil
}
