package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// NewCommand creates a new stack
type NewCommand struct {
	// Arguments
	StackName string

	// Flags
	BaseBranch string
}

// Register registers the command with cobra
func (c *NewCommand) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "new <stack-name>",
		Short: "Create a new stack",
		Long: `Create a new stack for managing a set of stacked pull requests.

This will:
  1. Create a new branch (username/stack-<name>) from the current HEAD
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
func (c *NewCommand) Run(ctx context.Context) error {
	// Check if we're in a git repository
	if !git.IsGitRepo() {
		return fmt.Errorf("not in a git repository")
	}

	// Check if stack already exists
	if stack.StackExists(c.StackName) {
		return fmt.Errorf("stack '%s' already exists", c.StackName)
	}

	// Get current branch as base if not specified
	if c.BaseBranch == "" {
		currentBranch, err := git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		c.BaseBranch = currentBranch
	}

	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	// Format branch name
	branchName := git.FormatStackBranch(username, c.StackName)

	// Check if branch already exists
	if git.BranchExists(branchName) {
		return fmt.Errorf("branch '%s' already exists", branchName)
	}

	// Create stack branch
	if err := git.CreateAndCheckoutBranch(branchName); err != nil {
		return fmt.Errorf("failed to create stack branch: %w", err)
	}

	// Create stack metadata
	s := &stack.Stack{
		Name:       c.StackName,
		Branch:     branchName,
		Base:       c.BaseBranch,
		Created:    time.Now(),
		LastSynced: time.Time{},
	}

	if err := stack.SaveStack(s); err != nil {
		return fmt.Errorf("failed to save stack: %w", err)
	}

	// Set as current stack
	if err := stack.SetCurrentStack(c.StackName); err != nil {
		return fmt.Errorf("failed to set current stack: %w", err)
	}

	fmt.Printf("✓ Created stack '%s'\n", c.StackName)
	fmt.Printf("✓ Branch: %s\n", branchName)
	fmt.Printf("✓ Base: %s\n", c.BaseBranch)
	fmt.Printf("✓ Switched to stack branch\n")

	return nil
}
