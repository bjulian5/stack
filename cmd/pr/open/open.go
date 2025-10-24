package open

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
	UseSelect bool

	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open a PR in the browser",
		Long: `Opens the PR for the current change in your browser.

Use --select to interactively choose which PR to open.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&c.UseSelect, "select", "s", false, "Interactively select which PR to open")

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	var selectedChange *stack.Change

	if c.UseSelect {
		var prsOnly []stack.Change
		for _, change := range stackCtx.AllChanges {
			if !change.IsLocal() {
				prsOnly = append(prsOnly, change)
			}
		}

		if len(prsOnly) == 0 {
			return fmt.Errorf("no PRs in this stack: use 'stack push' to create PRs")
		}

		selectedChange, err = ui.SelectChange(prsOnly)
		if err != nil {
			return err
		}
		if selectedChange == nil {
			return nil
		}
	} else {
		currentUUID := stackCtx.GetCurrentPositionUUID()
		if currentUUID == "" {
			return fmt.Errorf("no current position in stack")
		}

		selectedChange = stackCtx.FindChange(currentUUID)
		if selectedChange == nil {
			return fmt.Errorf("current change not found in stack")
		}

		if selectedChange.PR == nil {
			return fmt.Errorf("current change does not have a PR yet: use 'stack push' to create it")
		}
	}

	if err := c.GH.OpenPR(selectedChange.PR.PRNumber); err != nil {
		return fmt.Errorf("failed to open PR in browser: %w (ensure 'gh' CLI is installed)", err)
	}

	ui.Successf("Opening PR #%d: %s", selectedChange.PR.PRNumber, selectedChange.Title)

	return nil
}
