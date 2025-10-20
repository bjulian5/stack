package list

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
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

	if len(stacks) == 0 {
		fmt.Println("No stacks found.")
		fmt.Println("Create a new stack with: stack new <name>")
		return nil
	}

	// Get current stack from working context
	var currentStack string
	stackCtx, err := c.Stack.GetStackContext()
	if err == nil && stackCtx.IsStack() {
		currentStack = stackCtx.StackName
	}

	fmt.Println("Available stacks:")
	fmt.Println()

	for _, s := range stacks {
		// Get change count
		count := 0
		ctx, err := c.Stack.GetStackContextByName(s.Name)
		if err == nil {
			count = len(ctx.Changes)
		}

		// Mark current stack
		marker := " "
		if s.Name == currentStack {
			marker = "*"
		}

		prText := "PR"
		if count != 1 {
			prText = "PRs"
		}

		fmt.Printf("%s %-20s (%d %s, base: %s)\n", marker, s.Name, count, prText, s.Base)
	}

	if currentStack != "" {
		fmt.Println()
		fmt.Println("* = current stack")
	}

	return nil
}
