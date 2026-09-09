package push

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command pushes PRs to GitHub
type Command struct {
	// Flags
	DryRun bool // Show what would happen without actually doing it
	Force  bool // Force push all PRs (bypass diff check)

	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "push",
		Short: "Push PRs to GitHub",
		Long: `Push all PRs in the current stack to GitHub.

Creates new PRs or updates existing ones based on each change's local draft/ready state.
By default, new changes are created as drafts. Use 'stack ready' or 'stack draft' to
change a PR's state before pushing.

By default, stack uses a diff-based approach and skips PRs that haven't changed.
Use --force to bypass the diff check and push all PRs regardless.

Example:
  stack push              # Push all PRs (respects local draft/ready state)
  stack push --dry-run    # Show what would happen
  stack push --force      # Force push all PRs even if unchanged`,
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return c.Run(cobraCmd.Context())
		},
	}

	command.Flags().BoolVar(&c.DryRun, "dry-run", false, "Show what would happen without pushing")
	command.Flags().BoolVar(&c.Force, "force", false, "Force push all PRs even if unchanged (bypass diff check)")

	parent.AddCommand(command)
}

// pushPR pushes a single PR to GitHub and returns PR number, URL, and whether it was newly created
func (c *Command) syncPR(
	stackCtx *stack.StackContext,
	change model.Change,
	prBranch string,
	base string,
	existingPRNumber int,
) (prNumber int, url string, isNew bool, err error) {
	spec := gh.PRSpec{
		Number: existingPRNumber,
		Title:  change.Title,
		Body:   change.Description,
		Base:   base,
		Head:   prBranch,
		Draft:  change.GetDraftStatus(),
	}

	ghPR, err := c.GH.SyncPR(spec)
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to sync PR for %s: %w", change.Title, err)
	}

	// Find the change in stackCtx to update it (since change is passed by value)
	changeInCtx := stackCtx.FindChange(change.UUID)
	if changeInCtx == nil {
		return 0, "", false, fmt.Errorf("change %s not found in stack context", change.UUID)
	}

	// Update the change with the GitHub response
	changeInCtx.UpdateFromPush(ghPR, prBranch)
	changeInCtx.UpdateTitle(spec.Title, spec.Body, spec.Base)

	// Persist to disk
	if err := stackCtx.Save(); err != nil {
		return 0, "", false, fmt.Errorf("failed to save stack context: %w", err)
	}

	return ghPR.Number, ghPR.URL, existingPRNumber == 0, nil
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack.")
	}

	// Check for uncommitted changes
	hasChanges, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return err
	}
	if hasChanges {
		ui.Warning("Uncommitted changes detected in the working directory.")
	}

	if !c.DryRun {
		res, err := c.Stack.SyncPRMetadata(stackCtx)
		if err != nil {
			return fmt.Errorf("failed to sync with GitHub: %w", err)
		}
		if res.StaleMergedCount > 0 {
			ui.Println("")
			ui.Warning("One or more PRs have been merged on GitHub.")
			ui.Print("Please run 'stack restack' to sync your stack before pushing.")
			return fmt.Errorf("stack out of sync - run 'stack restack' first")
		}
	}

	if c.DryRun {
		ui.Info("Dry run mode - no changes will be made")
		ui.Println("")
	}

	type pushItem struct {
		change               *model.Change
		branch, reason, base string
		existing             int
	}
	var items []pushItem
	var created, updated, skipped int
	var existingNumbers []int
	for _, change := range stackCtx.ActiveChanges {
		if change.PR != nil && change.PR.PRNumber > 0 {
			existingNumbers = append(existingNumbers, change.PR.PRNumber)
		}
	}
	current := make(map[int]gh.PRDetails, len(existingNumbers))
	if len(existingNumbers) > 0 {
		current, _, err = c.GH.GetPRDetails(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, existingNumbers)
		if err != nil {
			return fmt.Errorf("failed to read current PR bases: %w", err)
		}
	}
	for _, number := range existingNumbers {
		if current[number].Number == 0 {
			return fmt.Errorf("PR #%d is missing on GitHub", number)
		}
	}
	if err := validatePRHeads(stackCtx, current); err != nil {
		return err
	}
	if c.DryRun {
		for _, detail := range current {
			if detail.Merged {
				return fmt.Errorf("stack out of sync - run 'stack restack' first")
			}
		}
	}

	for _, change := range stackCtx.ActiveChanges {
		prBranch := stackCtx.FormatUUIDBranch(change.UUID)

		existingPRNumber := 0
		var existingPR *model.PR

		if change.PR != nil {
			existingPRNumber = change.PR.PRNumber
			existingPR = change.PR
		}

		if c.DryRun {
			if existingPRNumber > 0 {
				ui.Printf("Would update PR #%d: %s\n", existingPRNumber, change.Title)
			} else {
				ui.Printf("Would create PR: %s\n", change.Title)
			}
			if existingPRNumber == 0 {
				continue
			}
		}

		// Skip PRs that are closed on GitHub (not merged)
		// GitHub doesn't allow updating the base branch of closed PRs
		if existingPR != nil && existingPR.State == "closed" {
			skipped++
			ui.Print(ui.RenderPushProgress(ui.PushProgress{
				Position: change.Position,
				Total:    len(stackCtx.AllChanges),
				Title:    change.Title,
				PRNumber: existingPR.PRNumber,
				URL:      existingPR.URL,
				Action:   "skipped",
				Reason:   "PR is closed on GitHub - reopen it or remove the commit from the stack",
			}))
			continue
		}

		var updateReason string
		if existingPR != nil && !c.Force {
			syncStatus := change.NeedsSyncToGitHub()

			if !syncStatus.NeedsSync {
				skipped++
				ui.Print(ui.RenderPushProgress(ui.PushProgress{
					Position: change.Position,
					Total:    len(stackCtx.AllChanges),
					Title:    change.Title,
					PRNumber: existingPR.PRNumber,
					URL:      existingPR.URL,
					Action:   "skipped",
				}))
				continue
			}

			updateReason = syncStatus.Reason
		}

		base := change.DesiredBase
		if existingPRNumber > 0 {
			base = current[existingPRNumber].BaseRef
		}
		items = append(items, pushItem{change: change, branch: prBranch, reason: updateReason, base: base, existing: existingPRNumber})
	}

	if c.DryRun {
		_, err := c.Stack.ReconcileNative(ctx, stackCtx, true)
		return err
	}

	branches := make([]string, 0, len(items))
	for _, item := range items {
		if err := c.Git.UpdateRef(item.branch, item.change.CommitHash); err != nil {
			return fmt.Errorf("failed to update branch %s: %w", item.branch, err)
		}
		branches = append(branches, item.branch)
	}
	if err := c.Git.PushAtomic(branches, true); err != nil {
		return err
	}

	for _, item := range items {
		prNumber, prURL, isNew, err := c.syncPR(stackCtx, *item.change, item.branch, item.base, item.existing)
		if err != nil {
			return err
		}
		action := "updated"
		if isNew {
			created++
			action = "created"
		} else {
			updated++
		}
		ui.Print(ui.RenderPushProgress(ui.PushProgress{Position: item.change.Position, Total: len(stackCtx.AllChanges), Title: item.change.Title,
			PRNumber: prNumber, URL: prURL, Action: action, Reason: item.reason}))
	}

	ui.Print(ui.RenderPushSummary(created, updated, skipped))
	if _, err := c.Stack.ReconcileNative(ctx, stackCtx, false); err != nil {
		return fmt.Errorf("reconcile GitHub-native stack: %w", err)
	}

	return nil
}

// GitHub cannot retarget a PR's head. Reject mismatches before pushing refs or editing PRs.
func validatePRHeads(stackCtx *stack.StackContext, current map[int]gh.PRDetails) error {
	for _, change := range stackCtx.ActiveChanges {
		if change.PR == nil || change.PR.PRNumber == 0 {
			continue
		}
		number := change.PR.PRNumber
		want := stackCtx.FormatUUIDBranch(change.UUID)
		if got := current[number].HeadRef; got != want {
			return fmt.Errorf("PR #%d uses head %q, but stack metadata targets %q; repair the local stack branch before pushing", number, got, want)
		}
	}
	return nil
}
