package show

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
	c.Stack = stack.NewClient(c.Git)
	c.GH = gh.NewClient()

	cmd := &cobra.Command{
		Use:   "show [stack-name]",
		Short: "Show details of a stack",
		Long: `Show detailed information about a stack including all PRs.

If no stack name is provided, shows the current stack.

Example:
  stack show
  stack show auth-refactor`,
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
			return fmt.Errorf("not on a stack branch: use 'stack show <name>'")
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

	// Auto-refresh if sync conditions are met
	if err := c.autoRefreshIfNeeded(stackCtx); err != nil {
		// If refresh fails, show warning but continue with stale data
		fmt.Println(ui.RenderWarningMessage(fmt.Sprintf("Failed to refresh stack: %v", err)))
		fmt.Println(ui.RenderInfoMessage("Showing potentially stale data. Run 'stack refresh' manually."))
		fmt.Println()
	} else {
		// Reload context to get fresh data after refresh
		stackCtx, err = c.Stack.GetStackContextByName(stackName)
		if err != nil {
			return err
		}
	}

	// Render using the new UI
	output := ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges)
	fmt.Println(output)

	return nil
}

// autoRefreshIfNeeded checks if stack needs sync and performs refresh if needed
func (c *Command) autoRefreshIfNeeded(stackCtx *stack.StackContext) error {
	// Check sync status
	syncStatus, err := c.Stack.CheckSyncStatus(stackCtx.StackName)
	if err != nil {
		// If we can't check status, skip refresh
		return nil
	}

	// If doesn't need sync, return immediately
	if !syncStatus.NeedsSync {
		return nil
	}

	// If no active changes, skip refresh (nothing to sync)
	if len(stackCtx.ActiveChanges) == 0 {
		return nil
	}

	// Perform refresh silently
	fmt.Println(ui.RenderInfoMessage("Refreshing stack to get latest PR status..."))

	refreshOps := stack.NewRefreshOperations(c.Git, c.Stack, c.GH)
	result, err := refreshOps.PerformRefresh(stackCtx)
	if err != nil {
		return err
	}

	// Show brief result message
	if result.MergedCount > 0 {
		fmt.Println(ui.RenderSuccessMessage(
			fmt.Sprintf("✓ Refreshed: %d PR(s) merged, %d remaining", result.MergedCount, result.RemainingCount),
		))
	} else {
		fmt.Println(ui.RenderSuccessMessage("✓ Stack is up to date"))
	}
	fmt.Println()

	return nil
}
