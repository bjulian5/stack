package newcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
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

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
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
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, _, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c.StackName = args[0]
			return c.Run(cobraCmd.Context())
		},
	}

	command.Flags().StringVar(&c.BaseBranch, "base", "", "Base branch for the stack (default: current branch)")
	parent.AddCommand(command)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Check if stack is installed
	installed, err := c.Stack.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status: %w", err)
	}

	if !installed {
		return fmt.Errorf("stack is not installed in this repository\n\nRun 'stack install' first to set up hooks and configuration")
	}

	baseBranch := c.BaseBranch
	if baseBranch == "" {
		baseBranch, err = c.Git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Create the stack
	s, err := c.Stack.CreateStack(c.StackName, baseBranch)
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	// Switch to the new stack
	if err := c.Stack.SwitchStack(c.StackName); err != nil {
		return fmt.Errorf("failed to switch to stack: %w", err)
	}

	// Display results
	ui.Successf("Created stack '%s'", s.Name)
	ui.Successf("Branch: %s", s.Branch)
	ui.Successf("Base: %s", s.Base)
	ui.Success("Switched to stack branch")

	return nil
}
