package top

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

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

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Move to the top of the stack",
		Long: `Move to the top of the stack (TOP branch with all commits).

Can be used from any position in the stack to return to the main stack branch.

Example:
  stack top    # Move to TOP branch from any position`,
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

	// Check sync status and warn if stale
	ui.WarnIfStackStale(stackCtx.StackName, c.Stack)

	// Check if already on TOP branch
	if !stackCtx.IsEditing() {
		return fmt.Errorf("already at top of stack")
	}

	// Checkout the TOP branch
	topBranch := stackCtx.Stack.Branch
	if err := c.Git.CheckoutBranch(topBranch); err != nil {
		return fmt.Errorf("failed to checkout top branch: %w", err)
	}

	// Print success message
	fmt.Println(ui.RenderSuccessMessage(fmt.Sprintf("Moved to top of stack: %s", topBranch)))

	return nil
}
