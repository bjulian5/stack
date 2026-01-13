package amend

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command amends staged changes into a selected change in the stack
type Command struct {
	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client

	// Flags
	interactive bool
	force       bool
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "amend [position]",
		Short: "Amend staged changes into a change in the stack",
		Long: `Amend staged changes into a specific change in the stack.

When called without arguments, opens an interactive fuzzy finder to select
which change to amend. When given a position number, amends directly into
that change.

This is the recommended way to modify commits in the middle of a stack.
It stages your changes into the target commit and automatically rebases
subsequent commits.

Requirements:
- Must be on a stack TOP branch
- Must have staged changes (use 'git add' first)

After successful rebase, you will remain on the TOP branch.

If amending a commit with many subsequent commits, the command will warn about
potential conflicts. Use --force to skip this warning.

Examples:
  # Interactive mode (default)
  git add .
  stack amend

  # Amend into change at position 2
  git add .
  stack amend 2

  # Force amend even with potential conflicts
  git add .
  stack amend 2 --force`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context(), args)
		},
	}

	command.Flags().BoolVarP(&c.interactive, "interactive", "i", false, "Use interactive fuzzy finder (default when no position given)")
	command.Flags().BoolVarP(&c.force, "force", "f", false, "Skip conflict warnings and proceed with amend")

	parent.AddCommand(command)

	// Hidden alias for backwards compatibility
	fixupAlias := &cobra.Command{
		Use:    "fixup [position]",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context(), args)
		},
	}
	fixupAlias.Flags().BoolVarP(&c.interactive, "interactive", "i", false, "Use interactive fuzzy finder")
	fixupAlias.Flags().BoolVarP(&c.force, "force", "f", false, "Skip conflict warnings")
	parent.AddCommand(fixupAlias)
}

// Run executes the command
func (c *Command) Run(ctx context.Context, args []string) error {
	// Check if rebase is already in progress
	if c.Git.IsRebaseInProgress() {
		return fmt.Errorf("rebase already in progress: resolve conflicts and run 'git rebase --continue' or abort with 'git rebase --abort'")
	}

	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Validate we're in a stack and not editing
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	if stackCtx.OnUUIDBranch() {
		return fmt.Errorf("cannot run amend while editing a change: checkout the stack TOP branch first or use 'git commit --amend'")
	}

	// Sync metadata with GitHub (read-only, no git operations)
	stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
	if err != nil {
		return fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Check for staged changes - this is required
	hasStaged, err := c.Git.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}
	if !hasStaged {
		return fmt.Errorf("no staged changes detected: stage your changes with 'git add' before running amend")
	}

	// Validate stack has active changes
	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("no active changes to amend: all changes are merged")
	}

	// Determine which change to amend
	var selectedChange *model.Change

	if len(args) > 0 && !c.interactive {
		// Direct position mode
		selectedChange, err = c.findChangeByPosition(stackCtx.ActiveChanges, args[0])
		if err != nil {
			return err
		}
	} else {
		// Interactive mode (default or explicit -i)
		selectedChange, err = ui.SelectChange(stackCtx.ActiveChanges)
		if err != nil {
			return err
		}
		if selectedChange == nil {
			// User cancelled
			return nil
		}
	}

	// Error if trying to amend a merged change
	if c.Stack.IsChangeMerged(selectedChange) {
		return fmt.Errorf(
			"cannot amend change #%d - it has been merged on GitHub\nRun 'stack refresh' to sync your stack",
			selectedChange.ActivePosition,
		)
	}

	// Validate the change has a commit hash
	if selectedChange.CommitHash == "" {
		return fmt.Errorf("cannot amend change #%d: missing commit hash", selectedChange.ActivePosition)
	}

	// Calculate how many commits will be rebased
	commitsToRebase := len(stackCtx.ActiveChanges) - selectedChange.ActivePosition
	if commitsToRebase > 0 && !c.force {
		ui.Warningf("%d commit(s) will be rebased after this amend", commitsToRebase)
		if commitsToRebase >= 3 {
			ui.Info("This may cause conflicts. Use --force to skip this warning.")
			ui.Printf("Continue? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				ui.Info("Aborted.")
				return nil
			}
		}
	}

	// Create fixup commit
	ui.Infof("Amending change #%d: %s", selectedChange.ActivePosition, selectedChange.Title)
	if err := c.Git.CommitFixup(selectedChange.CommitHash); err != nil {
		return err
	}

	// Get parent of the target commit for rebase
	parentHash, err := c.Git.GetParentCommit(selectedChange.CommitHash)
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Run interactive rebase with autosquash
	ui.Info("Rebasing stack...")
	if err := c.Git.RebaseInteractiveAutosquash(parentHash); err != nil {
		return err
	}

	// Success message
	ui.Successf("Successfully amended change #%d: %s", selectedChange.ActivePosition, selectedChange.Title)
	ui.Info("You are now on the TOP branch with the updated stack.")

	return nil
}

// findChangeByPosition finds a change by its active position number
func (c *Command) findChangeByPosition(changes []*model.Change, posArg string) (*model.Change, error) {
	pos, err := strconv.Atoi(posArg)
	if err != nil {
		return nil, fmt.Errorf("invalid position '%s': must be a number", posArg)
	}

	if pos < 1 || pos > len(changes) {
		return nil, fmt.Errorf("invalid position %d: must be between 1 and %d", pos, len(changes))
	}

	// Find change with matching ActivePosition
	for _, change := range changes {
		if change.ActivePosition == pos {
			return change, nil
		}
	}

	return nil, fmt.Errorf("no change found at position %d", pos)
}
