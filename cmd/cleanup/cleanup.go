package cleanup

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
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
		Use:   "cleanup",
		Short: "Clean up stacks with all PRs merged or empty stacks",
		Long: `Clean up stacks that are eligible for deletion:
  - Stacks where all PRs have been merged
  - Stacks with no changes (empty stacks)

The command will scan all stacks, identify candidates, and prompt for confirmation
before deleting. Deleted stacks are archived to .git/stack/.archived/ for recovery.

Example:
  stack cleanup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	fmt.Println(ui.RenderInfoMessage("Scanning stacks for cleanup candidates..."))
	fmt.Println()

	candidates, err := c.Stack.GetCleanupCandidates()
	if err != nil {
		return fmt.Errorf("failed to get cleanup candidates: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Println(ui.RenderSuccessMessage("No stacks need cleanup! All stacks are active."))
		return nil
	}

	c.displayCandidates(candidates)

	prompt := "Select stacks to clean up (e.g., '1,3,5' or 'all' or 'none'): "
	selectedIndices := ui.PromptSelection(prompt, len(candidates))
	if len(selectedIndices) == 0 {
		fmt.Println("Cleanup cancelled.")
		return nil
	}

	fmt.Println()
	return c.deleteSelected(candidates, selectedIndices)
}

func (c *Command) displayCandidates(candidates []stack.CleanupCandidate) {
	fmt.Printf("Found %d stack(s) eligible for cleanup:\n\n", len(candidates))

	for i, candidate := range candidates {
		reason := c.formatReason(candidate)
		fmt.Printf("  [%d] %s\n", i+1, ui.Bold(candidate.Stack.Name))
		fmt.Printf("      Base: %s\n", candidate.Stack.Base)
		fmt.Printf("      Reason: %s\n", reason)
		fmt.Println()
	}
}

func (c *Command) formatReason(candidate stack.CleanupCandidate) string {
	switch candidate.Reason {
	case "empty":
		return "Empty stack (no changes)"
	case "all_merged":
		return fmt.Sprintf("All %d change(s) merged", candidate.ChangeCount)
	default:
		return candidate.Reason
	}
}

func (c *Command) deleteSelected(candidates []stack.CleanupCandidate, indices []int) error {
	successCount := 0
	for _, idx := range indices {
		candidate := candidates[idx]
		fmt.Println(ui.RenderInfoMessage(fmt.Sprintf("Cleaning up stack: %s", candidate.Stack.Name)))
		fmt.Println()

		if err := c.Stack.DeleteStack(candidate.Stack.Name, true); err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning up stack %s: %v\n", candidate.Stack.Name, err)
			continue
		}

		successCount++
		fmt.Println()
	}

	if successCount > 0 {
		fmt.Println(ui.RenderSuccessMessage(fmt.Sprintf("Successfully cleaned up %d stack(s)", successCount)))
	}
	return nil
}
