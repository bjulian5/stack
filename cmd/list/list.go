package list

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
	Table bool
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.GH = gh.NewClient()
	c.Stack = stack.NewClient(c.Git, c.GH)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all stacks",
		Long: `List all stacks in the repository.

Shows the stack name, number of PRs, and base branch for each stack.
The current stack is marked with an asterisk (*).

Example:
  stack list
  stack list --table`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.Table, "table", false, "Display as table instead of tree")

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	stacks, err := c.Stack.ListStacks()
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	var currentStack string
	stackCtx, err := c.Stack.GetStackContext()
	if err == nil && stackCtx.IsStack() {
		currentStack = stackCtx.StackName
	}

	stackChanges := make(map[string][]stack.Change)
	for _, s := range stacks {
		ctx, err := c.Stack.GetStackContextByName(s.Name)
		if err != nil {
			ui.Warningf("failed to load stack %s: %v", s.Name, err)
			continue
		}

		if s.Name == currentStack {
			ctx, err = c.Stack.MaybeRefreshStackMetadata(ctx)
			if err != nil {
				ui.Warningf("failed to refresh stack %s: %v", s.Name, err)
			}
		}

		stackChanges[s.Name] = ctx.AllChanges
	}

	var output string
	if c.Table {
		output = ui.RenderStackListTable(stacks, stackChanges, currentStack)
	} else {
		output = ui.RenderStackList(stacks, currentStack, stackChanges)
	}
	ui.Print(output)

	return nil
}
