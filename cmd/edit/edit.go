package edit

import (
	"context"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command edits a change in the stack
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

	// Validate stack has changes
	if len(stackCtx.Changes) == 0 {
		return fmt.Errorf("no changes in stack: add commits to create PRs")
	}

	// Use fuzzy finder to select a change
	idx, err := fuzzyfinder.Find(
		stackCtx.Changes,
		func(i int) string {
			return ui.FormatChangeFinderLine(stackCtx.Changes[i])
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return ui.FormatChangePreview(stackCtx.Changes[i])
		}),
	)

	if err != nil {
		// User cancelled
		return nil
	}

	selectedChange := stackCtx.Changes[idx]

	// Validate UUID exists
	if selectedChange.UUID == "" {
		return fmt.Errorf("cannot edit change #%d: commit missing PR-UUID trailer (may have been created before git hooks were installed - try amending it on the stack branch first)", selectedChange.Position)
	}

	// Checkout UUID branch for editing
	branchName, err := stack.CheckoutChangeForEditing(c.Git, stackCtx, &selectedChange)
	if err != nil {
		return err
	}

	// Print success message using new UI
	fmt.Println(ui.RenderEditSuccess(selectedChange.Position, selectedChange.Title, branchName))

	// TODO: Add cleanup mechanism for stale UUID branches after changes are merged/deleted.
	// Over time, users will accumulate many UUID branches that should be cleaned up.
	// Consider implementing: stack clean [--stack <name>] [--merged] [--all]

	return nil
}
