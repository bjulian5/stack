package cmd

import (
	"fmt"
	"os/user"
	"time"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	baseBranch string
)

var newCmd = &cobra.Command{
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
		stackName := args[0]

		// Check if we're in a git repository
		if !git.IsGitRepo() {
			return fmt.Errorf("not in a git repository")
		}

		// Check if stack already exists
		if stack.StackExists(stackName) {
			return fmt.Errorf("stack '%s' already exists", stackName)
		}

		// Get current branch as base if not specified
		if baseBranch == "" {
			currentBranch, err := git.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}
			baseBranch = currentBranch
		}

		// Get username for branch naming
		username, err := getUsername()
		if err != nil {
			return fmt.Errorf("failed to get username: %w", err)
		}

		// Format branch name
		branchName := git.FormatStackBranch(username, stackName)

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
			Name:       stackName,
			Branch:     branchName,
			Base:       baseBranch,
			Created:    time.Now(),
			LastSynced: time.Time{},
		}

		if err := stack.SaveStack(s); err != nil {
			return fmt.Errorf("failed to save stack: %w", err)
		}

		// Set as current stack
		if err := stack.SetCurrentStack(stackName); err != nil {
			return fmt.Errorf("failed to set current stack: %w", err)
		}

		fmt.Printf("✓ Created stack '%s'\n", stackName)
		fmt.Printf("✓ Branch: %s\n", branchName)
		fmt.Printf("✓ Base: %s\n", baseBranch)
		fmt.Printf("✓ Switched to stack branch\n")

		return nil
	},
}

func init() {
	newCmd.Flags().StringVar(&baseBranch, "base", "", "Base branch for the stack (default: current branch)")
}

// getUsername returns the username for branch naming
func getUsername() (string, error) {
	// TODO: Add config support for username override
	// For now, use system username
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	return currentUser.Username, nil
}
