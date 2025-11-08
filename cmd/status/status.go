package status

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

type Command struct {
	StackName string
	Table     bool
	Git       *git.Client
	Stack     *stack.Client
	GH        *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "status [stack-name]",
		Short: "Show status of a stack",
		Long: `Show detailed status of a stack including all PRs.

If no stack name is provided, shows the current stack.

Example:
  stack status
  stack status auth-refactor
  stack status --table`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cobraCmd.Context())
		},
	}

	command.Flags().BoolVar(&c.Table, "table", false, "Display as table instead of tree")

	parent.AddCommand(command)
}

func (c *Command) Run(ctx context.Context) error {
	var stackCtx *stack.StackContext
	var err error

	if c.StackName == "" {
		stackCtx, err = c.Stack.GetStackContext()
		if err != nil || !stackCtx.IsStack() {
			return fmt.Errorf("not on a stack branch: use 'stack status <name>'")
		}
	} else {
		stackCtx, err = c.Stack.GetStackContextByName(c.StackName)
		if err != nil {
			return err
		}
	}

	if stackCtx.Stack == nil {
		return fmt.Errorf("stack '%s' does not exist", stackCtx.StackName)
	}

	stackCtx, err = c.Stack.MaybeRefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	currentUUID := stackCtx.ChangeID()

	var output string
	if c.Table {
		output = ui.RenderStackDetailsTable(stackCtx.Stack, stackCtx.AllChanges, currentUUID)
	} else {
		output = ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges, currentUUID)
	}
	ui.Print(output)

	return nil
}
