package edit

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

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

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.GH = gh.NewClient()
	c.Stack = stack.NewClient(c.Git, c.GH)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a change in the stack",
		Long: `Interactively select a change to edit using a fuzzy finder.

Creates a UUID branch at the selected commit, allowing you to make changes.
Use 'git commit --amend' to update the change, or create a new commit to insert after it.

Example:
  stack edit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
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

	// Print success message
	ui.Print(ui.RenderEditSuccess(selectedChange.Position, selectedChange.Title, branchName))

	// TODO: Add cleanup mechanism for stale UUID branches after changes are merged/deleted.
	// Over time, users will accumulate many UUID branches that should be cleaned up.
	// Consider implementing: stack clean [--stack <name>] [--merged] [--all]

	return nil
}
