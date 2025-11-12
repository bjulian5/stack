package down

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command moves down to the previous change in the stack
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "down",
		Short: "Move down to the previous change in the stack",
		Long: `Move down to the previous change (lower position) in the stack.

Can be used from the TOP branch to start navigating, or from a UUID branch to move down one position.

Example:
  stack down    # From TOP: move to position N-1
  stack down    # From UUID branch: move to previous position`,
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

	currentChange := stackCtx.FindChange(stackCtx.ChangeID())
	if currentChange == nil {
		return fmt.Errorf("current change is not a valid change in the stack")
	}

	if currentChange.ActivePosition == 1 {
		if len(stackCtx.ActiveChanges) == 1 {
			ui.Warning("Only 1 active change in stack")
		} else {
			ui.Warning("Already at the bottommost active change.")
		}
		return nil
	}

	currentActivePosition := currentChange.ActivePosition

	// Calculate target position (move down by 1)
	targetPosition := currentActivePosition - 1
	targetChange := stackCtx.ActiveChanges[targetPosition-1]

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

	// Print success message with stack tree
	ui.Print(ui.RenderNavigationSuccess(ui.NavigationSuccess{
		Message:     fmt.Sprintf("Moved to change #%d: %s", targetChange.Position, targetChange.Title),
		Stack:       stackCtx.Stack,
		Changes:     stackCtx.AllChanges,
		CurrentUUID: targetChange.UUID,
		IsEditing:   true,
	}))

	return nil
}
