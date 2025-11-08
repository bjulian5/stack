package cleanup

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
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
	command := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up stacks with all PRs merged or empty stacks",
		Long: `Clean up stacks that are eligible for deletion:
  - Stacks where all PRs have been merged
  - Stacks with no changes (empty stacks)

The command will scan all stacks, identify candidates, and prompt for confirmation
before deleting. Deleted stacks are archived to .git/stack/.archived/ for recovery.

Example:
  stack cleanup`,
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context())
		},
	}

	parent.AddCommand(command)
}

func (c *Command) Run(ctx context.Context) error {
	ui.Info("Scanning stacks for cleanup candidates...")
	ui.Println("")

	candidates, err := c.Stack.GetCleanupCandidates()
	if err != nil {
		return fmt.Errorf("failed to get cleanup candidates: %w", err)
	}

	if len(candidates) == 0 {
		ui.Success("No stacks need cleanup! All stacks are active.")
		return nil
	}

	c.displayCandidates(candidates)

	prompt := "Select stacks to clean up (e.g., '1,3,5' or 'all' or 'none'): "
	selectedIndices := ui.PromptSelection(prompt, len(candidates))
	if len(selectedIndices) == 0 {
		ui.Info("Cleanup cancelled.")
		return nil
	}

	ui.Println("")
	return c.deleteSelected(candidates, selectedIndices)
}

func (c *Command) displayCandidates(candidates []stack.CleanupCandidate) {
	ui.Printf("Found %d stack(s) eligible for cleanup:\n\n", len(candidates))

	for i, candidate := range candidates {
		reason := c.formatReason(candidate)
		ui.Printf("  [%d] %s\n", i+1, ui.Bold(candidate.StackCtx.StackName))
		ui.Printf("      Base: %s\n", candidate.StackCtx.Stack.Base)
		ui.Printf("      Reason: %s\n", reason)
		ui.Println("")
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
		stack := candidate.StackCtx.Stack
		ui.Infof("Cleaning up stack: %s", stack.Name)
		ui.Println("")

		if candidate.Reason == "all_merged" {
			if err := c.Stack.SyncVisualizationComments(candidate.StackCtx); err != nil {
				ui.Errorf("updating visualization comments for stack %s: %v", stack.Name, err)
				continue
			}
		}

		if err := c.Stack.DeleteStack(candidate.StackCtx.Stack.Name, true); err != nil {
			ui.Errorf("cleaning up stack %s: %v", stack.Name, err)
			continue
		}

		successCount++
		ui.Println("")
	}

	if successCount > 0 {
		ui.Successf("Successfully cleaned up %d stack(s)", successCount)
	}
	return nil
}
