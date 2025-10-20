package newcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/hooks"
	"github.com/bjulian5/stack/internal/stack"
)

// Command creates a new stack
type Command struct {
	// Arguments
	StackName string

	// Flags
	BaseBranch string

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
		Use:   "new <stack-name>",
		Short: "Create a new stack",
		Long: `Create a new stack for managing a set of stacked pull requests.

This will:
  1. Create a new branch (username/stack-<name>/TOP) from the current HEAD
  2. Store stack metadata in .git/stack/<name>/
  3. Set this as the current stack
  4. Checkout the stack branch

Example:
  stack new auth-refactor
  stack new feature-x --base develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.StackName = args[0]
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&c.BaseBranch, "base", "", "Base branch for the stack (default: current branch)")
	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Create the stack
	s, err := c.Stack.CreateStack(c.StackName, c.BaseBranch)
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	// Switch to the new stack
	if err := c.Stack.SwitchStack(c.StackName); err != nil {
		return fmt.Errorf("failed to switch to stack: %w", err)
	}

	// Install git hooks
	if err := hooks.InstallHooks(c.Git.GitRoot()); err != nil {
		return fmt.Errorf("failed to install git hooks: %w", err)
	}

	// Display results
	fmt.Printf("✓ Created stack '%s'\n", s.Name)
	fmt.Printf("✓ Branch: %s\n", s.Branch)
	fmt.Printf("✓ Base: %s\n", s.Base)
	fmt.Printf("✓ Installed git hooks\n")
	fmt.Printf("✓ Switched to stack branch\n")

	return nil
}
