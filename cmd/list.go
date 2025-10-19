package cmd

import (
	"fmt"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stacks",
	Long: `List all stacks in the repository.

Shows the stack name, number of PRs, and base branch for each stack.
The current stack is marked with an asterisk (*).

Example:
  stack list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in a git repository
		if !git.IsGitRepo() {
			return fmt.Errorf("not in a git repository")
		}

		// Get all stacks
		stacks, err := stack.ListStacks()
		if err != nil {
			return fmt.Errorf("failed to list stacks: %w", err)
		}

		if len(stacks) == 0 {
			fmt.Println("No stacks found.")
			fmt.Println("Create a new stack with: stack new <name>")
			return nil
		}

		// Get current stack
		currentStack, _ := stack.GetCurrentStack()

		fmt.Println("Available stacks:")
		fmt.Println()

		for _, s := range stacks {
			// Get PR count
			count, err := stack.GetStackPRCount(s)
			if err != nil {
				count = 0
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
	},
}
