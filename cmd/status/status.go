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

	// Flags
	Table bool

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
  stack status auth-refactor
  stack status --table`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.Table, "table", false, "Display as table instead of tree")

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	var stackCtx *stack.StackContext
	var err error

	// If no stack name provided, use current branch context
	// If stack name provided, load by name (won't show arrow unless we're on that stack)
	if c.StackName == "" {
		// Get context from current branch (includes position)
		stackCtx, err = c.Stack.GetStackContext()
		if err != nil || !stackCtx.IsStack() {
			return fmt.Errorf("not on a stack branch: use 'stack status <name>'")
		}
	} else {
		// Load stack by name
		stackCtx, err = c.Stack.GetStackContextByName(c.StackName)
		if err != nil {
			return err
		}
	}

	if stackCtx.Stack == nil {
		return fmt.Errorf("stack '%s' does not exist", stackCtx.StackName)
	}

	// Sync metadata if stale (respects staleness threshold)
	stackCtx, err = c.Stack.MaybeRefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Get current position for arrow indicator (empty if not on this stack)
	currentUUID := stackCtx.GetCurrentPositionUUID()

	// Render using table or tree view based on flag
	var output string
	if c.Table {
		output = ui.RenderStackDetailsTable(stackCtx.Stack, stackCtx.AllChanges, currentUUID)
	} else {
		output = ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges, currentUUID)
	}
	ui.Print(output)

	return nil
}
