package up

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command moves up to the next change in the stack
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Move up to the next change in the stack",
		Long: `Move up to the next change (higher position) in the stack.

Must be on a UUID branch (editing a specific change). Use 'stack down' to start navigating from the TOP branch.

Example:
  stack up    # Move from position 2 to position 3`,
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

	// Validate stack has changes
	if len(stackCtx.Changes) == 0 {
		return fmt.Errorf("no changes in stack: add commits to create PRs")
	}

	// Check if we're on TOP branch
	if !stackCtx.IsEditing() {
		return fmt.Errorf("already at top of stack: use 'stack down' to start navigating")
	}

	// Get current change and position
	currentChange := stackCtx.CurrentChange()
	if currentChange == nil {
		return fmt.Errorf("failed to find current change in stack")
	}

	// Calculate target position
	targetPosition := currentChange.Position + 1

	// Validate target exists
	if targetPosition > len(stackCtx.Changes) {
		return fmt.Errorf("already at top position %d of %d", currentChange.Position, len(stackCtx.Changes))
	}

	// Get target change by index (Position is 1-indexed)
	targetChange := &stackCtx.Changes[targetPosition-1]

	// Validate UUID exists
	if targetChange.UUID == "" {
		return fmt.Errorf("cannot navigate to change #%d: commit missing PR-UUID trailer", targetPosition)
	}

	// Checkout UUID branch for editing
	branchName, err := stack.CheckoutChangeForEditing(c.Git, stackCtx, targetChange)
	if err != nil {
		return err
	}

	// Print success message
	fmt.Println(ui.RenderEditSuccess(targetChange.Position, targetChange.Title, branchName))

	return nil
}
