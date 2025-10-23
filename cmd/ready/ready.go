package ready

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command marks changes as ready for review
type Command struct {
	// Flags
	All bool // Mark all changes in the stack as ready

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
	ghClient := gh.NewClient()
	c.Stack = stack.NewClient(c.Git, ghClient)

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Mark change(s) as ready for review",
		Long: `Mark one or more changes as ready for review (not draft).

When on a UUID branch: marks the current change as ready
When on TOP branch: marks the top change as ready
Use --all to mark all changes in the stack as ready

The ready/draft state is stored locally and applied during 'stack push'.

Example:
  stack ready         # Mark current change as ready
  stack ready --all   # Mark all changes in stack as ready`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.All, "all", false, "Mark all changes in the stack as ready")

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack.")
	}

	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("no changes in stack")
	}

	// Determine which changes to mark as ready
	var changesToMark []stack.Change
	if c.All {
		// Mark all active changes
		changesToMark = stackCtx.ActiveChanges
	} else {
		// Mark current change only
		currentChange := stackCtx.CurrentChange()
		if currentChange == nil {
			// If on TOP branch, mark the top change
			currentChange = &stackCtx.ActiveChanges[len(stackCtx.ActiveChanges)-1]
		}
		changesToMark = []stack.Change{*currentChange}
	}

	// Update LocalDraft for each change
	for _, change := range changesToMark {
		if change.UUID == "" {
			ui.Warningf("Skipping change without UUID: %s", change.Title)
			continue
		}

		if err := c.Stack.SetLocalDraft(stackCtx.StackName, change.UUID, false); err != nil {
			return fmt.Errorf("failed to update change %s: %w", change.Title, err)
		}

		ui.Successf("Marked as ready: %s", change.Title)
	}

	ui.Println("")
	ui.Info("Run 'stack push' to sync changes to GitHub")

	return nil
}
