package stack

import (
	"fmt"
	"time"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
)

// RefreshOperations provides shared refresh operations for syncing stacks with GitHub
type RefreshOperations struct {
	Git   *git.Client
	Stack *Client
	GH    *gh.Client
}

// NewRefreshOperations creates a new RefreshOperations instance
func NewRefreshOperations(gitClient *git.Client, stackClient *Client, ghClient *gh.Client) *RefreshOperations {
	return &RefreshOperations{
		Git:   gitClient,
		Stack: stackClient,
		GH:    ghClient,
	}
}

// RefreshResult contains the results of a refresh operation
type RefreshResult struct {
	MergedCount    int      // Number of PRs that were merged
	RemainingCount int      // Number of PRs still active
	MergedChanges  []Change // The changes that were merged
}

// PerformRefresh performs a complete refresh operation on a stack
// Returns the refresh result or an error if the operation fails
func (r *RefreshOperations) PerformRefresh(stackCtx *StackContext) (*RefreshResult, error) {
	// If no active changes, nothing to refresh
	if len(stackCtx.ActiveChanges) == 0 {
		// Still update sync metadata
		if err := r.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: 0,
			MergedChanges:  nil,
		}, nil
	}

	// Fetch from remote
	if err := r.FetchRemote(); err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}

	// Query GitHub for PR states
	newlyMerged, err := r.FindMergedPRs(stackCtx.ActiveChanges)
	if err != nil {
		return nil, err
	}

	if len(newlyMerged) == 0 {
		// Update sync metadata even if no merges
		if err := r.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: len(stackCtx.ActiveChanges),
			MergedChanges:  nil,
		}, nil
	}

	// Validate bottom-up order
	mergedPRNumbers := make(map[int]bool)
	for _, change := range newlyMerged {
		mergedPRNumbers[change.PR.PRNumber] = true
	}
	if err := ValidateBottomUpMerges(stackCtx.ActiveChanges, mergedPRNumbers); err != nil {
		return nil, err
	}

	// Save merged changes to stack metadata
	if err := r.SaveMergedChanges(stackCtx.StackName, newlyMerged); err != nil {
		return nil, fmt.Errorf("failed to save merged changes: %w", err)
	}

	// Rebase TOP on latest base
	if err := r.RebaseTopBranch(stackCtx); err != nil {
		return nil, fmt.Errorf("failed to rebase TOP: %w", err)
	}

	// Clean up UUID branches for merged PRs (non-fatal errors)
	_ = r.CleanupMergedBranches(stackCtx, newlyMerged)

	// Update sync metadata
	if err := r.UpdateSyncMetadata(stackCtx.StackName); err != nil {
		return nil, err
	}

	remainingCount := len(stackCtx.ActiveChanges) - len(newlyMerged)
	return &RefreshResult{
		MergedCount:    len(newlyMerged),
		RemainingCount: remainingCount,
		MergedChanges:  newlyMerged,
	}, nil
}

// FetchRemote fetches from the remote repository
func (r *RefreshOperations) FetchRemote() error {
	remote, err := r.Git.GetRemoteName()
	if err != nil {
		return err
	}

	if err := r.Git.Fetch(remote); err != nil {
		return err
	}

	return nil
}

// FindMergedPRs queries GitHub for each PR and returns those that are merged
func (r *RefreshOperations) FindMergedPRs(activeChanges []Change) ([]Change, error) {
	var merged []Change

	for _, change := range activeChanges {
		// Skip local changes (not yet pushed to GitHub)
		if change.PR == nil {
			continue
		}

		// Query GitHub for this PR's state
		prState, err := r.GH.GetPRState(change.PR.PRNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get state for PR #%d: %w", change.PR.PRNumber, err)
		}

		if prState.IsMerged {
			// Mark as merged and set timestamp
			change.IsMerged = true
			change.MergedAt = prState.MergedAt
			merged = append(merged, change)
		}
	}

	return merged, nil
}

// SaveMergedChanges appends newly merged changes to the stack metadata
func (r *RefreshOperations) SaveMergedChanges(stackName string, newlyMerged []Change) error {
	// Load current stack
	s, err := r.Stack.LoadStack(stackName)
	if err != nil {
		return err
	}

	// Initialize if nil
	if s.MergedChanges == nil {
		s.MergedChanges = []Change{}
	}

	// Append newly merged changes
	s.MergedChanges = append(s.MergedChanges, newlyMerged...)

	// Save stack with updated merged changes
	if err := r.Stack.SaveStack(s); err != nil {
		return err
	}

	return nil
}

// RebaseTopBranch rebases the TOP branch on the latest base branch, removing merged commits
func (r *RefreshOperations) RebaseTopBranch(stackCtx *StackContext) error {
	// Get the base branch (e.g., origin/main)
	baseBranch := stackCtx.Stack.Base

	// The rebase will automatically skip commits that are already in base
	// (which includes the merged commits). Note: fetch already done earlier.
	if err := r.Git.Rebase(baseBranch); err != nil {
		return err
	}

	return nil
}

// CleanupMergedBranches deletes UUID branches for merged PRs
// Errors are non-fatal and just printed as warnings
func (r *RefreshOperations) CleanupMergedBranches(stackCtx *StackContext, merged []Change) error {
	username, err := common.GetUsername()
	if err != nil {
		return err
	}

	for _, change := range merged {
		branchName := stackCtx.FormatUUIDBranch(username, change.UUID)

		// Delete local branch if it exists
		if r.Git.BranchExists(branchName) {
			_ = r.Git.DeleteBranch(branchName, true) // Ignore errors
		}

		// Delete remote branch if it exists
		_ = r.Git.DeleteRemoteBranch(branchName) // Ignore errors
	}

	return nil
}

// UpdateSyncMetadata updates the stack's sync timestamp and hash
func (r *RefreshOperations) UpdateSyncMetadata(stackName string) error {
	s, err := r.Stack.LoadStack(stackName)
	if err != nil {
		return err
	}

	// Get current TOP branch hash
	currentHash, err := r.Git.GetCommitHash(s.Branch)
	if err != nil {
		return err
	}

	// Update sync metadata
	s.LastSynced = time.Now()
	s.SyncHash = currentHash

	// Save
	if err := r.Stack.SaveStack(s); err != nil {
		return err
	}

	return nil
}
