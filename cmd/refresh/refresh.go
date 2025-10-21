package refresh

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command refreshes the stack by syncing with GitHub to detect merged PRs
type Command struct {
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
		Use:   "refresh",
		Short: "Sync stack with GitHub to detect merged PRs",
		Long: `Sync the current stack with GitHub to detect merged PRs and update the stack state.

This command:
  1. Fetches from remote
  2. Queries GitHub for each PR's merge status
  3. Validates bottom-up merging (errors if out-of-order)
  4. Saves merged changes to stack metadata
  5. Rebases remaining commits on the latest base branch
  6. Cleans up merged PR branches

Example:
  stack refresh`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack.")
	}

	// Check for uncommitted changes
	hasChanges, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return err
	}
	if hasChanges {
		return fmt.Errorf("you have uncommitted changes. Commit or stash them before refreshing.")
	}

	// If no active changes, nothing to refresh
	if len(stackCtx.ActiveChanges) == 0 {
		fmt.Println(ui.RenderInfoMessage("No active changes to refresh - all changes are already merged."))
		return nil
	}

	// Create refresh operations
	refreshOps := stack.NewRefreshOperations(c.Git, c.Stack, c.GH)

	// Perform refresh (fetches from remote and syncs with GitHub)
	fmt.Println("Fetching from remote...")
	fmt.Println("Checking PR merge status on GitHub...")
	result, err := refreshOps.PerformRefresh(stackCtx)
	if err != nil {
		return err
	}

	// Display results
	if result.MergedCount == 0 {
		fmt.Println(ui.RenderSuccessMessage("✓ No merged PRs found. Stack is up to date."))
		return nil
	}

	// Display what was merged
	fmt.Printf("\nFound %d merged PR(s):\n", result.MergedCount)
	for _, change := range result.MergedChanges {
		fmt.Printf("  ✓ #%d: %s\n", change.PR.PRNumber, change.Title)
	}

	// Display summary
	fmt.Println()
	fmt.Println(ui.RenderSuccessMessage(
		fmt.Sprintf("✓ Stack refreshed: %d merged, %d remaining", result.MergedCount, result.RemainingCount),
	))

	return nil
}
