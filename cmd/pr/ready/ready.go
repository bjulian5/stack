package ready

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
	All bool

	Git   *git.Client
	Stack *stack.Client
}

func (c *Command) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Mark change(s) as ready for review",
		Long: `Mark one or more changes as ready for review (not draft).

When on a UUID branch: marks the current change as ready
When on TOP branch: marks the top change as ready
Use --all to mark all changes in the stack as ready

If the PR already exists on GitHub, it will be marked as ready immediately.
Otherwise, the ready/draft state is stored locally and applied during 'stack push'.

Example:
  stack pr ready         # Mark current change as ready
  stack pr ready --all   # Mark all changes in stack as ready`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.All, "all", false, "Mark all changes in the stack as ready")

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
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

	var changesToMark []model.Change
	if c.All {
		changesToMark = stackCtx.ActiveChanges
	} else {
		currentChange := stackCtx.CurrentChange()
		if currentChange == nil {
			return fmt.Errorf("unable to determine current change")
		}
		changesToMark = []model.Change{*currentChange}
	}

	hasUnpushedChanges := false
	for i := range changesToMark {
		change := &changesToMark[i]
		if change.UUID == "" {
			ui.Warningf("Skipping change without UUID: %s", change.Title)
			continue
		}

		result, err := c.Stack.MarkChangeReady(stackCtx.StackName, change)
		if err != nil {
			return fmt.Errorf("failed to mark change %s as ready: %w", change.Title, err)
		}

		if result.SyncedToGitHub {
			ui.Successf("Marked as ready on GitHub: %s (PR #%d)", change.Title, result.PRNumber)
		} else {
			ui.Successf("Marked as ready locally: %s", change.Title)
			hasUnpushedChanges = true
		}
	}

	if hasUnpushedChanges {
		ui.Println("")
		ui.Info("Run 'stack push' to create PRs for changes that aren't yet on GitHub")
	}

	return nil
}
