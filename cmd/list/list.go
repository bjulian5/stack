package list

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command lists all stacks
type Command struct {
	// Clients (can be mocked in tests)
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
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all stacks",
		Long: `List all stacks in the repository.

Shows the stack name, number of PRs, and base branch for each stack.
The current stack is marked with an asterisk (*).

Example:
  stack list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get all stacks
	stacks, err := c.Stack.ListStacks()
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	// Get current stack from working context
	var currentStack string
	stackCtx, err := c.Stack.GetStackContext()
	if err == nil && stackCtx.IsStack() {
		currentStack = stackCtx.StackName
	}

	// Load changes for all stacks
	stackChanges := make(map[string][]stack.Change)
	for _, s := range stacks {
		ctx, err := c.Stack.GetStackContextByName(s.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load stack %s: %v\n", s.Name, err)
			continue
		}
		stackChanges[s.Name] = ctx.AllChanges
	}

	output := ui.RenderStackList(stacks, currentStack, stackChanges)
	fmt.Println(output)

	return nil
}
