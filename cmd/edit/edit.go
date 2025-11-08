package edit

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

// Command edits a change in the stack
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "edit",
		Short: "Edit a change in the stack",
		Long: `Interactively select a change to edit using a fuzzy finder.

Creates a UUID branch at the selected commit, allowing you to make changes.
Use 'git commit --amend' to update the change, or create a new commit to insert after it.

Example:
  stack edit`,
		Args: cobra.NoArgs,
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

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Check for uncommitted changes before switching branches
	hasUncommitted, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if hasUncommitted {
		return fmt.Errorf("uncommitted changes detected: commit or stash your changes before editing a different change")
	}

	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Validate we're in a stack
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	// Sync metadata with GitHub (read-only, no git operations)
	stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Validate stack has active changes
	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("no active changes to edit: all changes are merged")
	}

	// Use fuzzy finder to select a change
	selectedChange, err := ui.SelectChange(stackCtx.ActiveChanges)
	if err != nil {
		return err
	}
	if selectedChange == nil {
		// User cancelled
		return nil
	}

	// Error if trying to edit a merged change
	if c.Stack.IsChangeMerged(selectedChange) {
		return fmt.Errorf(
			"cannot edit change #%d - it has been merged on GitHub\nRun 'stack refresh' to sync your stack",
			selectedChange.Position,
		)
	}

	// Validate UUID exists
	if selectedChange.UUID == "" {
		return fmt.Errorf("cannot edit change #%d: commit missing PR-UUID trailer (may have been created before git hooks were installed - try amending it on the stack branch first)", selectedChange.Position)
	}

	// Checkout UUID branch for editing
	branchName, err := c.Stack.CheckoutChangeForEditing(stackCtx, selectedChange)
	if err != nil {
		return err
	}

	// Print success message with stack tree
	ui.Print(ui.RenderNavigationSuccess(ui.NavigationSuccess{
		Message:     fmt.Sprintf("Checked out change #%d: %s\nBranch: %s", selectedChange.Position, selectedChange.Title, branchName),
		Stack:       stackCtx.Stack,
		Changes:     stackCtx.AllChanges,
		CurrentUUID: selectedChange.UUID,
		IsEditing:   true,
	}))
	return nil
}
