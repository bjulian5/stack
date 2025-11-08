package draft

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
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
	command := &cobra.Command{
		Use:   "draft",
		Short: "Mark change(s) as draft",
		Long: `Mark one or more changes as draft (not ready for review).

When on a UUID branch: marks the current change as draft
When on TOP branch: marks the top change as draft
Use --all to mark all changes in the stack as draft

If the PR already exists on GitHub, it will be marked as draft immediately.
Otherwise, the ready/draft state is stored locally and applied during 'stack push'.

Example:
  stack pr draft         # Mark current change as draft
  stack pr draft --all   # Mark all changes in stack as draft`,
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, _, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context())
		},
	}

	command.Flags().BoolVar(&c.All, "all", false, "Mark all changes in the stack as draft")

	parent.AddCommand(command)
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

	var changesToMark []*model.Change
	if c.All {
		changesToMark = stackCtx.ActiveChanges
	} else {
		currentChange := stackCtx.CurrentChange()
		if currentChange == nil {
			return fmt.Errorf("unable to determine current change")
		}
		changesToMark = []*model.Change{currentChange}
	}

	hasUnpushedChanges := false
	for _, change := range changesToMark {
		if change.UUID == "" {
			ui.Warningf("Skipping change without UUID: %s", change.Title)
			continue
		}

		result, err := c.Stack.MarkChangeDraft(stackCtx, change)
		if err != nil {
			return fmt.Errorf("failed to mark change %s as draft: %w", change.Title, err)
		}

		if result.SyncedToGitHub {
			ui.Successf("✓ Marked as draft on GitHub: %s (PR #%d)", change.Title, result.PRNumber)
		} else {
			ui.Successf("✓ Marked as draft locally: %s", change.Title)
			hasUnpushedChanges = true
		}
	}

	if hasUnpushedChanges {
		ui.Println("")
		ui.Info("Run 'stack push' to create PRs for changes that aren't yet on GitHub")
	}

	return nil
}
