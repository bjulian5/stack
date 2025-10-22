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
	Ready  bool // Mark PRs as ready (not draft)
	DryRun bool // Show what would happen without actually doing it
	Force  bool // Force update stack visualizations even if no PRs created

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

Creates new PRs or updates existing ones. By default, PRs are created as drafts.
Use --ready to mark them as ready for review.

Example:
  stack push              # Push all PRs as drafts
  stack push --ready      # Push all PRs as ready for review
  stack push --dry-run    # Show what would happen
  stack push --force      # Force update stack visualizations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.Ready, "ready", false, "Mark PRs as ready for review (not draft)")
	cmd.Flags().BoolVar(&c.DryRun, "dry-run", false, "Show what would happen without pushing")
	cmd.Flags().BoolVar(&c.Force, "force", false, "Force update stack visualizations even if no PRs created")

	parent.AddCommand(cmd)
}

// pushPR pushes a single PR to GitHub
// Returns the PR number, URL, and whether it was newly created
func (c *Command) pushPR(
	stackName string,
	change stack.Change,
	prBranch string,
	baseBranch string,
	existingPRNumber int,
) (prNumber int, url string, isNew bool, err error) {
	// Create/update PR branch ref to point to this commit (update-ref is idempotent)
	if err := c.Git.UpdateRef(prBranch, change.CommitHash); err != nil {
		return 0, "", false, fmt.Errorf("failed to update branch %s: %w", prBranch, err)
	}

	// Push branch to remote
	if err := c.Git.Push(prBranch, true); err != nil {
		return 0, "", false, fmt.Errorf("failed to push branch %s: %w", prBranch, err)
	}

	// Build PR spec
	spec := gh.PRSpec{
		Number: existingPRNumber,
		Title:  change.Title,
		Body:   change.Description,
		Base:   baseBranch,
		Head:   prBranch,
		Draft:  !c.Ready,
	}

	// Sync PR on GitHub
	ghPR, err := c.GH.SyncPR(spec)
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to sync PR for %s: %w", change.Title, err)
	}

	// Update local PR tracking
	if err := c.Stack.SyncPRFromGitHub(stackName, change.UUID, prBranch, change.CommitHash, ghPR); err != nil {
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
		fmt.Println(ui.RenderInfoMessage("No unmerged PRs to push - all changes are merged."))
		return nil
	}

	// Check if any PRs have been merged on GitHub
	hasMerged, err := c.hasAnyMergedPRs(stackCtx.Stack.Owner, stackCtx.Stack.RepoName, stackCtx.ActiveChanges)
	if err != nil {
		return fmt.Errorf("failed to check PR states: %w", err)
	}
	if hasMerged {
		fmt.Println()
		fmt.Println(ui.RenderWarningMessage("One or more PRs have been merged on GitHub."))
		fmt.Println("Please run 'stack refresh' to sync your stack before pushing.")
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
		fmt.Println(ui.RenderInfoMessage("Dry run mode - no changes will be made"))
		fmt.Println()
	}

	// Push each PR (only active/unmerged changes)
	var created, updated int
	var previousBranch string // Track base branch for next PR

	for i, change := range stackCtx.ActiveChanges {
		position := i + 1
		total := len(stackCtx.ActiveChanges)

		// Get PR branch name
		prBranch := stackCtx.FormatUUIDBranch(username, change.UUID)

		// Determine base branch (previous PR's branch or stack base)
		baseBranch := stackCtx.Stack.Base
		if previousBranch != "" {
			baseBranch = previousBranch
		}

		// Get existing PR number
		existingPRNumber := 0
		if existingPR := prData.PRs[change.UUID]; existingPR != nil {
			existingPRNumber = existingPR.PRNumber
		}

		// Handle dry-run display
		if c.DryRun {
			if existingPRNumber > 0 {
				fmt.Printf("Would update PR #%d: %s\n", existingPRNumber, change.Title)
			} else {
				fmt.Printf("Would create PR: %s\n", change.Title)
			}
			previousBranch = prBranch
			continue
		}

		// Push PR to GitHub
		prNumber, prURL, isNew, err := c.pushPR(stackCtx.StackName, change, prBranch, baseBranch, existingPRNumber)
		if err != nil {
			return err
		}

		// Track stats
		if isNew {
			created++
		} else {
			updated++
		}

		// Display progress
		fmt.Println(ui.RenderPushProgress(position, total, change.Title, prNumber, prURL, isNew))

		// Set previous branch for next iteration
		previousBranch = prBranch
	}

	if c.DryRun {
		return nil
	}

	// Display summary
	fmt.Println(ui.RenderPushSummary(created, updated))

	// Update stack visualizations if any PRs were created or if force flag is set
	if created > 0 || c.Force {
		fmt.Println()
		fmt.Println(ui.RenderInfoMessage("Updating stack visualizations..."))

		// Reload stack context to get fresh PR data
		freshCtx, err := c.Stack.GetStackContext()
		if err != nil {
			return fmt.Errorf("failed to reload stack context: %w", err)
		}

		if err := c.Stack.SyncVisualizationComments(freshCtx); err != nil {
			return fmt.Errorf("failed to sync visualization comments: %w", err)
		}

		fmt.Println(ui.RenderSuccessMessage("Stack visualizations updated"))
	}

	return nil
}

// hasAnyMergedPRs checks if any PRs in the stack have been merged on GitHub
// Uses batch API to efficiently query all PRs in a single request
func (c *Command) hasAnyMergedPRs(owner, repoName string, activeChanges []stack.Change) (bool, error) {
	// Collect all PR numbers from active changes
	var prNumbers []int
	for _, change := range activeChanges {
		if change.PR != nil {
			prNumbers = append(prNumbers, change.PR.PRNumber)
		}
	}

	// If no PRs exist yet, nothing is merged
	if len(prNumbers) == 0 {
		return false, nil
	}

	// Batch query all PRs in one API call
	result, err := c.GH.BatchGetPRs(owner, repoName, prNumbers)
	if err != nil {
		return false, fmt.Errorf("failed to batch query PRs: %w", err)
	}

	// Check if any are merged
	for _, prState := range result.PRStates {
		if prState.IsMerged {
			return true, nil
		}
	}

	return false, nil
}
