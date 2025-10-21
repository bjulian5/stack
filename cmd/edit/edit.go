package edit

import (
	"context"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
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
			change := stackCtx.Changes[i]

			status := "local"
			if change.PR != nil {
				status = change.PR.State
			}
			icon := ui.GetStatusIcon(status)

			prLabel := "local"
			if change.PR != nil {
				prLabel = fmt.Sprintf("#%-4d", change.PR.PRNumber)
			}

			// Short hash
			shortHash := change.CommitHash
			if len(shortHash) > git.ShortHashLength {
				shortHash = shortHash[:git.ShortHashLength]
			}

			// Truncate title to fit nicely
			title := ui.Truncate(change.Title, 40)

			return fmt.Sprintf("%2d %s %-6s │ %-40s │ %s", change.Position, icon, prLabel, title, shortHash)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			change := stackCtx.Changes[i]

			preview := fmt.Sprintf("Position: %d\n", change.Position)
			preview += fmt.Sprintf("Title: %s\n", change.Title)
			preview += fmt.Sprintf("Commit: %s\n", change.CommitHash)
			if change.UUID != "" {
				preview += fmt.Sprintf("UUID: %s\n", change.UUID)
			}
			if change.PR != nil {
				preview += fmt.Sprintf("PR: #%d (%s)\n", change.PR.PRNumber, change.PR.State)
				preview += fmt.Sprintf("URL: %s\n", change.PR.URL)
			}
			if change.Description != "" {
				preview += fmt.Sprintf("\nDescription:\n%s\n", change.Description)
			}
			return preview
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

	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	// Format UUID branch name
	branchName := stackCtx.FormatUUIDBranch(username, selectedChange.UUID)

	// Check if UUID branch already exists
	if c.Git.BranchExists(branchName) {
		// Get the commit hash the existing branch points to
		existingHash, err := c.Git.GetCommitHash(branchName)
		if err != nil {
			return fmt.Errorf("failed to get branch commit: %w", err)
		}

		// Checkout the branch first
		if err := c.Git.CheckoutBranch(branchName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}

		// If branch is at wrong commit, sync it to the current commit location
		if existingHash != selectedChange.CommitHash {
			if err := c.Git.ResetHard(selectedChange.CommitHash); err != nil {
				return fmt.Errorf("failed to sync branch to current commit: %w", err)
			}
			// Truncate hashes for display
			oldShort := existingHash
			if len(oldShort) > git.ShortHashLength {
				oldShort = oldShort[:git.ShortHashLength]
			}
			newShort := selectedChange.CommitHash
			if len(newShort) > git.ShortHashLength {
				newShort = newShort[:git.ShortHashLength]
			}
			fmt.Println(ui.RenderWarningMessage(fmt.Sprintf("Synced branch to current commit (was at %s, now at %s)", oldShort, newShort)))
		}
	} else {
		// Create and checkout new branch at the commit
		if err := c.Git.CreateAndCheckoutBranchAt(branchName, selectedChange.CommitHash); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// Print success message using new UI
	fmt.Println(ui.RenderEditSuccess(selectedChange.Position, selectedChange.Title, branchName))

	// TODO: Add cleanup mechanism for stale UUID branches after changes are merged/deleted.
	// Over time, users will accumulate many UUID branches that should be cleaned up.
	// Consider implementing: stack clean [--stack <name>] [--merged] [--all]

	return nil
}
