package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
)

// DefaultSyncThreshold is the time threshold after which a stack is considered stale
// and needs to be refreshed to check for merged PRs on GitHub
const DefaultSyncThreshold = 5 * time.Minute

// GitOperations defines the git operations needed by Stack Client
type GitOperations interface {
	GetCurrentBranch() (string, error)
	BranchExists(name string) bool
	CreateAndCheckoutBranch(name string) error
	CheckoutBranch(name string) error
	GetCommits(branch, base string) ([]git.Commit, error)
	GetCommitHash(ref string) (string, error)
	GitRoot() string
	GetRemoteName() (string, error)
	Fetch(remote string) error
	Rebase(onto string) error
	DeleteBranch(branchName string, force bool) error
	DeleteRemoteBranch(branchName string) error
	ResetHard(ref string) error
	CreateAndCheckoutBranchAt(name string, commitHash string) error
	GetUpstreamBranch(branch string) (string, error)
	CreateBranchAt(branchName string, ref string) error
	UpdateRef(branchName string, commitHash string) error
}

// Client provides stack operations
type Client struct {
	git     GitOperations
	gh      *gh.Client
	gitRoot string
}

// NewClient creates a new stack client
func NewClient(gitOps GitOperations, ghClient *gh.Client) *Client {
	return &Client{
		git:     gitOps,
		gh:      ghClient,
		gitRoot: gitOps.GitRoot(),
	}
}

// getStackDir returns the directory where stack metadata is stored
func (c *Client) getStackDir(stackName string) string {
	return filepath.Join(c.gitRoot, ".git", "stack", stackName)
}

// getStacksRootDir returns the root directory for all stacks
func (c *Client) getStacksRootDir() string {
	return filepath.Join(c.gitRoot, ".git", "stack")
}

// LoadStack loads a stack configuration from disk
func (c *Client) LoadStack(name string) (*Stack, error) {
	stackDir := c.getStackDir(name)
	configPath := filepath.Join(stackDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack config: %w", err)
	}

	var stack Stack
	if err := json.Unmarshal(data, &stack); err != nil {
		return nil, fmt.Errorf("failed to parse stack config: %w", err)
	}

	// Backfill Owner and RepoName if missing (for stacks created before this feature)
	if stack.Owner == "" || stack.RepoName == "" {
		owner, repoName, err := c.gh.GetRepoInfo()
		if err != nil {
			// Non-fatal - we'll fetch it later when needed
			// Just continue without backfilling
		} else {
			stack.Owner = owner
			stack.RepoName = repoName
			// Save the updated stack (ignore errors to keep LoadStack read-only semantics)
			_ = c.SaveStack(&stack)
		}
	}

	return &stack, nil
}

// SaveStack saves a stack configuration to disk
func (c *Client) SaveStack(stack *Stack) error {
	stackDir := c.getStackDir(stack.Name)

	// Create stack directory if it doesn't exist
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	// Write config
	configPath := filepath.Join(stackDir, "config.json")
	data, err := json.MarshalIndent(stack, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stack config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write stack config: %w", err)
	}

	return nil
}

// SyncStatus indicates if a stack needs GitHub synchronization
type SyncStatus struct {
	NeedsSync bool   // True if stack needs refresh
	Reason    string // Why sync is needed (internal use)
	Warning   string // User-facing warning message
}

// CheckSyncStatus checks if a stack needs refresh without hitting GitHub.
// Returns status indicating whether sync is needed and why.
func (c *Client) CheckSyncStatus(stackName string) (*SyncStatus, error) {
	stack, err := c.LoadStack(stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack: %w", err)
	}

	status := &SyncStatus{}

	// Never synced - definitely needs sync
	if stack.LastSynced.IsZero() {
		status.NeedsSync = true
		status.Reason = "never_synced"
		status.Warning = "Stack has never been synced with GitHub. Run 'stack refresh' to check for merged PRs."
		return status, nil
	}

	// Check if TOP branch changed since last sync
	// This indicates new commits were added
	currentHash, err := c.git.GetCommitHash(stack.Branch)
	if err != nil {
		// If we can't get the hash, be conservative and say sync needed
		status.NeedsSync = true
		status.Reason = "hash_check_failed"
		status.Warning = "Could not verify stack sync status. Run 'stack refresh' to ensure consistency."
		return status, nil
	}

	if currentHash != stack.SyncHash {
		status.NeedsSync = true
		status.Reason = "commits_changed"
		status.Warning = "Stack has new commits since last sync. Run 'stack refresh' to ensure consistency with GitHub."
		return status, nil
	}

	// Check time threshold
	// Even if nothing changed locally, GitHub might have merged PRs
	if time.Since(stack.LastSynced) > DefaultSyncThreshold {
		status.NeedsSync = true
		status.Reason = "stale"
		status.Warning = "Stack sync is stale. Run 'stack refresh' to check for merged PRs."
		return status, nil
	}

	// All checks passed - in sync
	status.NeedsSync = false
	return status, nil
}

// StackExists checks if a stack exists
func (c *Client) StackExists(name string) bool {
	stackDir := c.getStackDir(name)
	configPath := filepath.Join(stackDir, "config.json")
	_, err := os.Stat(configPath)
	return err == nil
}

// ListStacks returns all stacks in the repository
func (c *Client) ListStacks() ([]*Stack, error) {
	stacksRoot := c.getStacksRootDir()

	// Check if stacks directory exists
	if _, err := os.Stat(stacksRoot); os.IsNotExist(err) {
		return []*Stack{}, nil
	}

	// Read all subdirectories
	entries, err := os.ReadDir(stacksRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read stacks directory: %w", err)
	}

	stacks := []*Stack{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stack, err := c.LoadStack(entry.Name())
		if err != nil {
			// Skip invalid stacks
			continue
		}

		stacks = append(stacks, stack)
	}

	return stacks, nil
}

// GetStackContext returns the stack context based on the current git branch.
// This is the single source of truth for what stack you're working on.
func (c *Client) GetStackContext() (*StackContext, error) {
	// Get current branch
	branch, err := c.git.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	ctx := &StackContext{}

	// Determine stack name and editing UUID from branch
	var editingUUID string
	if isStackBranch(branch) {
		ctx.StackName = ExtractStackName(branch)
	} else if isUUIDBranch(branch) {
		ctx.StackName, editingUUID = ExtractUUIDFromBranch(branch)
		ctx.currentUUID = editingUUID
	} else {
		// Not on a stack-related branch
		return ctx, nil
	}

	// Load stack metadata
	if ctx.StackName != "" {
		stack, err := c.LoadStack(ctx.StackName)
		if err != nil {
			return ctx, fmt.Errorf("failed to load stack '%s': %w", ctx.StackName, err)
		}
		ctx.Stack = stack

		// Load all changes (merged + active)
		allChanges, activeChanges, err := c.getChangesForStack(stack)
		if err != nil {
			return nil, fmt.Errorf("failed to load changes for stack '%s': %w", ctx.StackName, err)
		}
		ctx.AllChanges = allChanges
		ctx.ActiveChanges = activeChanges
	}

	return ctx, nil
}

// GetStackContextByName loads stack context for a specific stack by name.
// This is useful for commands that operate on a stack without being on a stack branch.
func (c *Client) GetStackContextByName(name string) (*StackContext, error) {
	if name == "" {
		return nil, fmt.Errorf("stack name is required")
	}

	// Load stack metadata
	stack, err := c.LoadStack(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack '%s': %w", name, err)
	}

	// Load all changes (merged + active)
	allChanges, activeChanges, err := c.getChangesForStack(stack)
	if err != nil {
		return nil, err
	}

	return &StackContext{
		StackName:     name,
		Stack:         stack,
		AllChanges:    allChanges,
		ActiveChanges: activeChanges,
		currentUUID:   "", // Not editing (loaded by name)
	}, nil
}

// SwitchStack checks out the TOP branch of the specified stack.
// This is a convenience wrapper around git checkout.
func (c *Client) SwitchStack(name string) error {
	// Load the stack to get its branch name
	stack, err := c.LoadStack(name)
	if err != nil {
		return fmt.Errorf("failed to load stack: %w", err)
	}

	// Checkout the stack's branch
	if err := c.git.CheckoutBranch(stack.Branch); err != nil {
		return fmt.Errorf("failed to checkout stack branch: %w", err)
	}

	return nil
}

// CreateStack creates a new stack with the given name and base branch
func (c *Client) CreateStack(name string, baseBranch string) (*Stack, error) {
	// Check if stack already exists
	if c.StackExists(name) {
		return nil, fmt.Errorf("stack '%s' already exists", name)
	}

	// Get current branch as base if not specified
	if baseBranch == "" {
		currentBranch, err := c.git.GetCurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get current branch: %w", err)
		}
		baseBranch = currentBranch
	}

	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return nil, fmt.Errorf("failed to get username: %w", err)
	}

	// Format branch name
	branchName := FormatStackBranch(username, name)

	// Check if branch already exists
	if c.git.BranchExists(branchName) {
		return nil, fmt.Errorf("branch '%s' already exists", branchName)
	}

	// Create stack branch
	if err := c.git.CreateAndCheckoutBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to create stack branch: %w", err)
	}

	// Fetch and cache repo info
	owner, repoName, err := c.gh.GetRepoInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info: %w", err)
	}

	// Create stack metadata
	s := &Stack{
		Name:     name,
		Branch:   branchName,
		Base:     baseBranch,
		Owner:    owner,
		RepoName: repoName,
		Created:  time.Now(),
	}

	if err := c.SaveStack(s); err != nil {
		return nil, fmt.Errorf("failed to save stack: %w", err)
	}

	return s, nil
}

// getChangesForStack loads all changes for a stack (shared logic)
// getChangesForStack returns both AllChanges and ActiveChanges for a stack.
// AllChanges includes merged + active changes. ActiveChanges includes only unmerged changes.
func (c *Client) getChangesForStack(s *Stack) (allChanges []Change, activeChanges []Change, err error) {
	// Load PR tracking data
	prData, err := c.LoadPRs(s.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load PRs: %w", err)
	}

	// Get merged changes from stack metadata (not from a git branch)
	mergedChanges := s.MergedChanges
	if mergedChanges == nil {
		mergedChanges = []Change{}
	}

	// Load active changes from TOP branch
	activeCommits, err := c.git.GetCommits(s.Branch, s.Base)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get active commits: %w", err)
	}

	// Convert to changes with IsMerged = false
	activeChanges = c.commitsToChanges(activeCommits, prData, false)

	// Renumber positions for active changes (1-indexed)
	for i := range activeChanges {
		activeChanges[i].Position = i + 1
	}

	// Build AllChanges (merged first, then active)
	allChanges = make([]Change, 0, len(mergedChanges)+len(activeChanges))
	allChanges = append(allChanges, mergedChanges...)
	allChanges = append(allChanges, activeChanges...)

	return allChanges, activeChanges, nil
}

// commitsToChanges converts git commits to Changes with the specified merged status
func (c *Client) commitsToChanges(commits []git.Commit, prData *PRData, isMerged bool) []Change {
	changes := make([]Change, len(commits))
	for i, commit := range commits {
		uuid := commit.Message.Trailers["PR-UUID"]
		var pr *PR
		if uuid != "" {
			if p, ok := prData.PRs[uuid]; ok {
				pr = p
			}
		}

		changes[i] = Change{
			Position:    i + 1, // 1-indexed by commit order; renumbered later for active changes only
			Title:       commit.Message.Title,
			Description: commit.Message.Body,
			CommitHash:  commit.Hash,
			UUID:        uuid,
			PR:          pr,
			IsMerged:    isMerged,
		}
	}

	return changes
}

// LoadPRs loads PR tracking data for a stack
func (c *Client) LoadPRs(stackName string) (*PRData, error) {
	stackDir := c.getStackDir(stackName)
	prsPath := filepath.Join(stackDir, "prs.json")

	// If file doesn't exist, return empty PRData with current version
	if _, err := os.Stat(prsPath); os.IsNotExist(err) {
		return &PRData{Version: 1, PRs: make(map[string]*PR)}, nil
	}

	data, err := os.ReadFile(prsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRs file: %w", err)
	}

	var prData PRData
	if err := json.Unmarshal(data, &prData); err != nil {
		return nil, fmt.Errorf("failed to parse PRs file: %w", err)
	}

	// Set default version if not present (files created before versioning)
	if prData.Version == 0 {
		prData.Version = 1
	}

	// Ensure the map is initialized even if the JSON was empty
	if prData.PRs == nil {
		prData.PRs = make(map[string]*PR)
	}

	return &prData, nil
}

// SavePRs saves PR tracking data for a stack
func (c *Client) SavePRs(stackName string, prData *PRData) error {
	stackDir := c.getStackDir(stackName)

	// Ensure version is set before saving
	if prData.Version == 0 {
		prData.Version = 1
	}

	// Create stack directory if it doesn't exist
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	prsPath := filepath.Join(stackDir, "prs.json")
	data, err := json.MarshalIndent(prData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRs: %w", err)
	}

	if err := os.WriteFile(prsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write PRs file: %w", err)
	}

	return nil
}

// SetPR sets PR information for a UUID
func (c *Client) SetPR(stackName string, uuid string, pr *PR) error {
	prData, err := c.LoadPRs(stackName)
	if err != nil {
		return err
	}

	prData.PRs[uuid] = pr

	return c.SavePRs(stackName, prData)
}

// SyncPRFromGitHub syncs PR information from GitHub to local storage
// This is used by stack push to update prs.json with GitHub PR data
func (c *Client) SyncPRFromGitHub(stackName, uuid, branch, commitHash string, ghPR *gh.PR) error {
	prData, err := c.LoadPRs(stackName)
	if err != nil {
		return err
	}

	// Get existing PR or create new one
	pr, exists := prData.PRs[uuid]
	if !exists {
		pr = &PR{
			CreatedAt: ghPR.CreatedAt,
		}
	}

	// Update PR with GitHub data
	pr.PRNumber = ghPR.Number
	pr.URL = ghPR.URL
	pr.State = ghPR.State
	pr.Branch = branch
	pr.CommitHash = commitHash
	pr.LastPushed = ghPR.UpdatedAt

	// Store back in map
	prData.PRs[uuid] = pr

	return c.SavePRs(stackName, prData)
}

// ====================================================================
// Refresh Operations (moved from RefreshOperations)
// ====================================================================

// RefreshResult contains the results of a refresh operation
type RefreshResult struct {
	MergedCount    int      // Number of PRs that were merged
	RemainingCount int      // Number of PRs still active
	MergedChanges  []Change // The changes that were merged
}

// PerformRefresh performs a complete refresh operation on a stack
// Returns the refresh result or an error if the operation fails
func (c *Client) PerformRefresh(stackCtx *StackContext) (*RefreshResult, error) {
	// If no active changes, nothing to refresh
	if len(stackCtx.ActiveChanges) == 0 {
		// Still update sync metadata
		if err := c.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: 0,
			MergedChanges:  nil,
		}, nil
	}

	// Query GitHub for PR states first (fast check for merges)
	newlyMerged, err := c.FindMergedPRs(stackCtx)
	if err != nil {
		return nil, err
	}

	if len(newlyMerged) == 0 {
		// No merges found - update metadata and return early (skip fetch!)
		if err := c.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: len(stackCtx.ActiveChanges),
			MergedChanges:  nil,
		}, nil
	}

	// Merges found - fetch from remote before rebasing
	if err := c.FetchRemote(); err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
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
	if err := c.SaveMergedChanges(stackCtx.StackName, newlyMerged); err != nil {
		return nil, fmt.Errorf("failed to save merged changes: %w", err)
	}

	// Rebase TOP on latest base
	if err := c.RebaseTopBranch(stackCtx); err != nil {
		return nil, fmt.Errorf("failed to rebase TOP: %w", err)
	}

	// Clean up UUID branches for merged PRs (non-fatal errors)
	_ = c.CleanupMergedBranches(stackCtx, newlyMerged)

	// Update sync metadata
	if err := c.UpdateSyncMetadata(stackCtx.StackName); err != nil {
		return nil, err
	}

	remainingCount := len(stackCtx.ActiveChanges) - len(newlyMerged)
	return &RefreshResult{
		MergedCount:    len(newlyMerged),
		RemainingCount: remainingCount,
		MergedChanges:  newlyMerged,
	}, nil
}

// ForceRefresh ALWAYS syncs with GitHub regardless of staleness threshold.
// Use this for critical operations that could corrupt state (edit, fixup, push, navigation).
// This ensures we have the absolute latest PR states before mutating operations.
func (c *Client) ForceRefresh(stackCtx *StackContext) (*StackContext, error) {

	// Perform refresh (no threshold check)
	result, err := c.PerformRefresh(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync stack with GitHub: %w", err)
	}

	// Show brief result if anything changed
	if result.MergedCount > 0 {
		fmt.Printf("✓ Found %d merged PR(s)\n", result.MergedCount)
	}

	// Reload context with fresh data
	freshCtx, err := c.GetStackContext()
	if err != nil {
		return nil, fmt.Errorf("failed to reload stack context: %w", err)
	}

	return freshCtx, nil
}

// MaybeRefreshStack syncs with GitHub only if staleness threshold exceeded.
// Use this for non-critical operations (show, list) where slight staleness is acceptable.
// Respects the 5-minute threshold to avoid unnecessary API calls.
func (c *Client) MaybeRefreshStack(stackCtx *StackContext) (*StackContext, error) {
	// Quick check: do we need sync?
	syncStatus, err := c.CheckSyncStatus(stackCtx.StackName)
	if err != nil {
		// If we can't check, be conservative and refresh anyway
	} else if !syncStatus.NeedsSync {
		// Already fresh, no action needed
		return stackCtx, nil
	}

	// Perform refresh
	result, err := c.PerformRefresh(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync stack with GitHub: %w", err)
	}

	// Show brief result if anything changed
	if result.MergedCount > 0 {
		fmt.Printf("✓ Found %d merged PR(s)\n", result.MergedCount)
	}

	// Reload context with fresh data
	freshCtx, err := c.GetStackContext()
	if err != nil {
		return nil, fmt.Errorf("failed to reload stack context: %w", err)
	}

	return freshCtx, nil
}

// FetchRemote fetches from the remote repository
func (c *Client) FetchRemote() error {
	remote, err := c.git.GetRemoteName()
	if err != nil {
		return err
	}

	if err := c.git.Fetch(remote); err != nil {
		return err
	}

	return nil
}

// FindMergedPRs queries GitHub for PR states and returns those that are merged.
// Uses bulk GraphQL query for efficiency - single API call instead of N calls.
func (c *Client) FindMergedPRs(stackCtx *StackContext) ([]Change, error) {
	// Extract PR numbers from active changes
	var prNumbers []int
	for _, change := range stackCtx.ActiveChanges {
		// Skip local changes (not yet pushed to GitHub)
		if change.PR != nil {
			prNumbers = append(prNumbers, change.PR.PRNumber)
		}
	}

	// If no PRs to check, return empty
	if len(prNumbers) == 0 {
		return nil, nil
	}

	// Bulk query all PRs (much faster than individual queries)
	result, err := c.gh.BatchGetPRs(stackCtx.Stack.Owner, stackCtx.Stack.RepoName, prNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to batch query PRs: %w", err)
	}

	var merged []Change

	for _, change := range stackCtx.ActiveChanges {
		// Skip local changes
		if change.PR == nil {
			continue
		}

		// Lookup PR state from bulk query results
		prState, found := result.PRStates[change.PR.PRNumber]
		if !found {
			// PR was deleted or doesn't exist - skip it
			continue
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
func (c *Client) SaveMergedChanges(stackName string, newlyMerged []Change) error {
	// Load current stack
	s, err := c.LoadStack(stackName)
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
	if err := c.SaveStack(s); err != nil {
		return err
	}

	return nil
}

// RebaseTopBranch rebases the TOP branch on the latest base branch, removing merged commits
func (c *Client) RebaseTopBranch(stackCtx *StackContext) error {
	// Get the base branch (e.g., origin/main)
	baseBranch := stackCtx.Stack.Base

	// The rebase will automatically skip commits that are already in base
	// (which includes the merged commits). Note: fetch already done earlier.
	if err := c.git.Rebase(baseBranch); err != nil {
		return err
	}

	return nil
}

// CleanupMergedBranches deletes UUID branches for merged PRs
// Errors are non-fatal and just printed as warnings
func (c *Client) CleanupMergedBranches(stackCtx *StackContext, merged []Change) error {
	username, err := common.GetUsername()
	if err != nil {
		return err
	}

	for _, change := range merged {
		branchName := stackCtx.FormatUUIDBranch(username, change.UUID)

		// Delete local branch if it exists
		if c.git.BranchExists(branchName) {
			_ = c.git.DeleteBranch(branchName, true) // Ignore errors
		}

		// Delete remote branch if it exists
		_ = c.git.DeleteRemoteBranch(branchName) // Ignore errors
	}

	return nil
}

// UpdateSyncMetadata updates the stack's sync timestamp and hash
func (c *Client) UpdateSyncMetadata(stackName string) error {
	s, err := c.LoadStack(stackName)
	if err != nil {
		return err
	}

	// Get current TOP branch hash
	currentHash, err := c.git.GetCommitHash(s.Branch)
	if err != nil {
		return err
	}

	// Update sync metadata
	s.LastSynced = time.Now()
	s.SyncHash = currentHash

	// Save
	if err := c.SaveStack(s); err != nil {
		return err
	}

	return nil
}

type RestackOptions struct {
	// Onto specifies the base branch to rebase onto (required)
	Onto string

	// Fetch from remote before rebasing
	Fetch bool
}

// Restack rebases the stack on top of the specified base branch.
// Always updates stack metadata with the new base (idempotent if unchanged).
// If opts.Fetch is true, fetches from remote and updates local base ref before rebasing.
func (c *Client) Restack(stackCtx *StackContext, opts RestackOptions) error {
	targetBase := opts.Onto

	if opts.Fetch {
		fmt.Println("Fetching from remote...")
		if err := c.FetchRemote(); err != nil {
			return fmt.Errorf("failed to fetch: %w", err)
		}

		if err := c.UpdateLocalBaseRef(targetBase); err != nil {
			// Non-fatal: show warning and continue
			fmt.Fprintf(os.Stderr, "Warning: could not update local base ref: %v\n", err)
		}
	}

	if err := c.git.Rebase(targetBase); err != nil {
		return err
	}

	if stackCtx.Stack.Base != targetBase {
		stackCtx.Stack.Base = targetBase
		if err := c.SaveStack(stackCtx.Stack); err != nil {
			return fmt.Errorf("failed to update stack metadata: %w", err)
		}
		fmt.Printf("✓ Updated base branch: %s → %s\n", stackCtx.Stack.Base, targetBase)
	}
	return nil
}

// UpdateLocalBaseRef updates the local base branch ref to match its upstream
func (c *Client) UpdateLocalBaseRef(baseBranch string) error {
	upstream, err := c.git.GetUpstreamBranch(baseBranch)
	if err != nil {
		return err
	}
	if upstream == "" {
		return fmt.Errorf("no upstream tracking branch configured for %s", baseBranch)
	}

	upstreamHash, err := c.git.GetCommitHash(upstream)
	if err != nil {
		return err
	}

	// Get current hash of local base (may not exist, that's ok)
	currentHash, err := c.git.GetCommitHash(baseBranch)
	if err != nil {
		// Branch doesn't exist locally - create it at upstream's commit
		if err := c.git.CreateBranchAt(baseBranch, upstreamHash); err != nil {
			return err
		}
		fmt.Printf("Created local branch %s at %s\n", baseBranch, upstreamHash[:7])
		return nil
	}

	if upstreamHash != currentHash {
		if err := c.git.UpdateRef(baseBranch, upstreamHash); err != nil {
			return err
		}
		fmt.Printf("Updating %s: %s..%s\n", baseBranch, currentHash[:7], upstreamHash[:7])
	}
	return nil
}

// CheckoutChangeForEditing checks out a UUID branch for the given change, creating it if needed.
// If the branch already exists but points to a different commit, it syncs it to the current commit.
// Returns the branch name that was checked out.
func (c *Client) CheckoutChangeForEditing(stackCtx *StackContext, change *Change) (string, error) {
	// Get username for branch naming
	username, err := common.GetUsername()
	if err != nil {
		return "", fmt.Errorf("failed to get username: %w", err)
	}

	// Format UUID branch name
	branchName := stackCtx.FormatUUIDBranch(username, change.UUID)

	// Check if UUID branch already exists
	if c.git.BranchExists(branchName) {
		// Get the commit hash the existing branch points to
		existingHash, err := c.git.GetCommitHash(branchName)
		if err != nil {
			return "", fmt.Errorf("failed to get branch commit: %w", err)
		}

		// Checkout the branch first
		if err := c.git.CheckoutBranch(branchName); err != nil {
			return "", fmt.Errorf("failed to checkout branch: %w", err)
		}

		// If branch is at wrong commit, sync it to the current commit location
		if existingHash != change.CommitHash {
			if err := c.git.ResetHard(change.CommitHash); err != nil {
				return "", fmt.Errorf("failed to sync branch to current commit: %w", err)
			}
			// TODO:(this is probably unexpected)
			fmt.Printf("⚠️  Synced branch to current commit (was at %s, now at %s)\n",
				git.ShortHash(existingHash), git.ShortHash(change.CommitHash))
		}
	} else {
		// Create and checkout new branch at the commit
		if err := c.git.CreateAndCheckoutBranchAt(branchName, change.CommitHash); err != nil {
			return "", fmt.Errorf("failed to create branch: %w", err)
		}
	}

	return branchName, nil
}

// IsStackBranch checks if a branch name matches the stack branch pattern
func isStackBranch(branch string) bool {
	// Stack branches follow pattern: username/stack-<name>/TOP
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return false
	}

	// Check second part starts with "stack-" and third part is "TOP"
	return strings.HasPrefix(parts[1], "stack-") && parts[2] == "TOP"
}
