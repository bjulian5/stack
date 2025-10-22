package status

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command shows details of a stack
type Command struct {
	// Arguments
	StackName string

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
		Use:   "status [stack-name]",
		Short: "Show status of a stack",
		Long: `Show detailed status of a stack including all PRs.

If no stack name is provided, shows the current stack.

Example:
  stack status
  stack status auth-refactor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Resolve stack name - use context if not specified
	stackName := c.StackName
	if stackName == "" {
		stackCtx, err := c.Stack.GetStackContext()
		if err != nil || !stackCtx.IsStack() {
			return fmt.Errorf("not on a stack branch: use 'stack status <name>'")
		}
		stackName = stackCtx.StackName
	}

	// Get stack context
	stackCtx, err := c.Stack.GetStackContextByName(stackName)
	if err != nil {
		return err
	}

	if stackCtx.Stack == nil {
		return fmt.Errorf("stack '%s' does not exist", stackName)
	}

	// Auto-refresh if stale (respects threshold for display operations)
	stackCtx, err = c.Stack.MaybeRefreshStack(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Render using the new UI
	output := ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges)
	fmt.Println(output)

	return nil
}
