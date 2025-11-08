package refresh

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
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

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
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
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
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
		ui.Info("No active changes to refresh - all changes are already merged.")
		return nil
	}

	// Sync metadata with GitHub
	ui.Info("Checking PR merge status on GitHub...")
	result, err := c.Stack.SyncPRMetadata(stackCtx)
	if err != nil {
		return err
	}

	// Display results if no merges
	if result.StaleMergedCount == 0 {
		ui.Success("No merged PRs found. Stack is up to date.")
		return nil
	}

	if err := c.Stack.ApplyRefresh(stackCtx, result.StaleMergedChanges); err != nil {
		return err
	}

	// Display what was merged in table format
	ui.Println("")
	ui.Print(ui.RenderTitlef("Merged PRs (%d detected)", result.StaleMergedCount))
	ui.Print(ui.RenderMergedPRsTable(result.StaleMergedChanges))

	// Display summary
	ui.Println("")
	ui.Successf("Stack refreshed: %d merged, %d remaining", result.StaleMergedCount, result.RemainingCount)

	if result.RemainingCount > 0 {
		ui.Println("")
		ui.Info("Run 'stack push' to update PR base branches on GitHub")
	}

	return nil
}
