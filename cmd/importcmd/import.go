package importcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
	Branch string
	Base   string

	Git   *git.Client
	Stack *stack.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "import <branch>",
		Short: "Import a pushed stack from any member branch",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, _, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c.Branch = args[0]
			return c.Run(cobraCmd.Context())
		},
	}
	command.Flags().StringVar(&c.Base, "base", "", "Override the imported stack's base branch")
	parent.AddCommand(command)
}

func (c *Command) Run(ctx context.Context) error {
	installed, err := c.Stack.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status: %w", err)
	}
	if !installed {
		return fmt.Errorf("stack is not installed in this repository\n\nRun 'stack install' first to set up hooks and configuration")
	}
	hasChanges, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("uncommitted changes detected: commit or stash your changes before importing a stack")
	}

	stackCtx, err := c.Stack.ImportStack(ctx, stack.ImportOptions{Branch: c.Branch, Base: c.Base})
	if err != nil {
		return err
	}
	ui.Successf("Imported stack '%s'", stackCtx.StackName)
	ui.Successf("Branch: %s", stackCtx.Stack.Branch)
	ui.Successf("Base: %s", stackCtx.Stack.Base)
	ui.Successf("Pull requests: %d", len(stackCtx.AllChanges))
	return nil
}
