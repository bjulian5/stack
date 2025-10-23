package push

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

// Command pushes PRs to GitHub
type Command struct {
	// Flags
	DryRun bool // Show what would happen without actually doing it
	Force  bool // Force push all PRs (bypass diff check) and update visualizations

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
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.DryRun, "dry-run", false, "Show what would happen without pushing")
	cmd.Flags().BoolVar(&c.Force, "force", false, "Force push all PRs even if unchanged (bypass diff check)")

	parent.AddCommand(cmd)
}

// pushPR pushes a single PR to GitHub and returns PR number, URL, and whether it was newly created
func (c *Command) pushPR(
	stackName string,
	change stack.Change,
	prBranch string,
	baseBranch string,
	existingPRNumber int,
) (prNumber int, url string, isNew bool, err error) {
	if err := c.Git.UpdateRef(prBranch, change.CommitHash); err != nil {
		return 0, "", false, fmt.Errorf("failed to update branch %s: %w", prBranch, err)
	}

	if err := c.Git.Push(prBranch, true); err != nil {
		return 0, "", false, fmt.Errorf("failed to push branch %s: %w", prBranch, err)
	}

	spec := gh.PRSpec{
		Number: existingPRNumber,
		Title:  change.Title,
		Body:   change.Description,
		Base:   baseBranch,
		Head:   prBranch,
		Draft:  change.LocalDraft,
	}

	ghPR, err := c.GH.SyncPR(spec)
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to sync PR for %s: %w", change.Title, err)
	}

	syncData := stack.PRSyncData{
		StackName:  stackName,
		UUID:       change.UUID,
		Branch:     prBranch,
		CommitHash: change.CommitHash,
		GitHubPR:   ghPR,
		Title:      spec.Title,
		Body:       spec.Body,
		Base:       spec.Base,
	}
	if err := c.Stack.SyncPRFromGitHub(syncData); err != nil {
		return 0, "", false, fmt.Errorf("failed to update PR tracking: %w", err)
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
		return fmt.Errorf("you have uncommitted changes. Commit or stash them before pushing.")
	}

	if len(stackCtx.ActiveChanges) == 0 {
		ui.Info("No unmerged PRs to push - all changes are merged.")
		return nil
	}

	// Check if any PRs have been merged on GitHub
	hasMerged, err := c.hasAnyMergedPRs(stackCtx.Stack.Owner, stackCtx.Stack.RepoName, stackCtx.ActiveChanges)
	if err != nil {
		return fmt.Errorf("failed to check PR states: %w", err)
	}
	if hasMerged {
		ui.Println("")
		ui.Warning("One or more PRs have been merged on GitHub.")
		ui.Print("Please run 'stack refresh' to sync your stack before pushing.")
		return fmt.Errorf("stack out of sync - run 'stack refresh' first")
	}

	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	// Load existing PRs
	prData, err := c.Stack.LoadPRs(stackCtx.StackName)
	if err != nil {
		return fmt.Errorf("failed to load PRs: %w", err)
	}

	if c.DryRun {
		ui.Info("Dry run mode - no changes will be made")
		ui.Println("")
	}

	var created, updated, skipped int
	var previousBranch string

	for i, change := range stackCtx.ActiveChanges {
		position := i + 1
		total := len(stackCtx.ActiveChanges)

		prBranch := stackCtx.FormatUUIDBranch(username, change.UUID)

		baseBranch := stackCtx.Stack.Base
		if previousBranch != "" {
			baseBranch = previousBranch
		}

		existingPRNumber := 0
		var existingPR *stack.PR
		if pr := prData.PRs[change.UUID]; pr != nil {
			existingPRNumber = pr.PRNumber
			existingPR = pr
		}

		if c.DryRun {
			if existingPRNumber > 0 {
				ui.Printf("Would update PR #%d: %s\n", existingPRNumber, change.Title)
			} else {
				ui.Printf("Would create PR: %s\n", change.Title)
			}
			previousBranch = prBranch
			continue
		}

		var updateReason string
		if existingPR != nil && !c.Force {
			desiredState := stack.PRCompareState{
				Title:      change.Title,
				Body:       change.Description,
				Base:       baseBranch,
				CommitHash: change.CommitHash,
				IsDraft:    change.LocalDraft,
			}
			if !existingPR.NeedsUpdate(desiredState) {
				skipped++
				ui.Print(ui.RenderPushProgress(ui.PushProgress{
					Position: position,
					Total:    total,
					Title:    change.Title,
					PRNumber: existingPR.PRNumber,
					URL:      existingPR.URL,
					Action:   "skipped",
				}))
				previousBranch = prBranch
				continue
			}
			updateReason = existingPR.WhyNeedsUpdate(desiredState)
		}

		prNumber, prURL, isNew, err := c.pushPR(stackCtx.StackName, change, prBranch, baseBranch, existingPRNumber)
		if err != nil {
			return err
		}

		var action string
		if isNew {
			created++
			action = "created"
		} else {
			updated++
			action = "updated"
		}

		ui.Print(ui.RenderPushProgress(ui.PushProgress{
			Position: position,
			Total:    total,
			Title:    change.Title,
			PRNumber: prNumber,
			URL:      prURL,
			Action:   action,
			Reason:   updateReason,
		}))

		previousBranch = prBranch
	}

	if c.DryRun {
		return nil
	}

	ui.Print(ui.RenderPushSummary(created, updated, skipped))

	if created > 0 || c.Force {
		ui.Println("")
		ui.Info("Updating stack visualizations...")

		freshCtx, err := c.Stack.GetStackContext()
		if err != nil {
			return fmt.Errorf("failed to reload stack context: %w", err)
		}

		if err := c.Stack.SyncVisualizationComments(freshCtx); err != nil {
			return fmt.Errorf("failed to sync visualization comments: %w", err)
		}

		ui.Success("Stack visualizations updated")
	}

	return nil
}

// hasAnyMergedPRs checks if any PRs in the stack have been merged on GitHub
func (c *Command) hasAnyMergedPRs(owner, repoName string, activeChanges []stack.Change) (bool, error) {
	var prNumbers []int
	for _, change := range activeChanges {
		if change.PR != nil {
			prNumbers = append(prNumbers, change.PR.PRNumber)
		}
	}

	if len(prNumbers) == 0 {
		return false, nil
	}

	result, err := c.GH.BatchGetPRs(owner, repoName, prNumbers)
	if err != nil {
		return false, fmt.Errorf("failed to batch query PRs: %w", err)
	}

	for _, prState := range result.PRStates {
		if prState.IsMerged {
			return true, nil
		}
	}

	return false, nil
}
