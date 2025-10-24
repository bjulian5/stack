package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	HasUncommittedChanges() (bool, error)
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

func (c *Client) getStackDir(stackName string) string {
	return filepath.Join(c.gitRoot, ".git", "stack", stackName)
}

func (c *Client) getStacksRootDir() string {
	return filepath.Join(c.gitRoot, ".git", "stack")
}

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

	if stack.Owner == "" || stack.RepoName == "" {
		if owner, repoName, err := c.gh.GetRepoInfo(); err == nil {
			stack.Owner = owner
			stack.RepoName = repoName
			_ = c.SaveStack(&stack)
		}
	}

	return &stack, nil
}

func (c *Client) SaveStack(stack *Stack) error {
	stackDir := c.getStackDir(stack.Name)

	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

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

type SyncStatus struct {
	NeedsSync bool
	Reason    string
	Warning   string
}

func (c *Client) CheckSyncStatus(stackName string) (*SyncStatus, error) {
	stack, err := c.LoadStack(stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack: %w", err)
	}

	if stack.LastSynced.IsZero() {
		return &SyncStatus{
			NeedsSync: true,
			Reason:    "never_synced",
			Warning:   "Stack has never been synced with GitHub. Run 'stack refresh' to check for merged PRs.",
		}, nil
	}

	currentHash, err := c.git.GetCommitHash(stack.Branch)
	if err != nil {
		return &SyncStatus{
			NeedsSync: true,
			Reason:    "hash_check_failed",
			Warning:   "Could not verify stack sync status. Run 'stack refresh' to ensure consistency.",
		}, nil
	}

	if currentHash != stack.SyncHash {
		return &SyncStatus{
			NeedsSync: true,
			Reason:    "commits_changed",
			Warning:   "Stack has new commits since last sync. Run 'stack refresh' to ensure consistency with GitHub.",
		}, nil
	}

	if time.Since(stack.LastSynced) > DefaultSyncThreshold {
		return &SyncStatus{
			NeedsSync: true,
			Reason:    "stale",
			Warning:   "Stack sync is stale. Run 'stack refresh' to check for merged PRs.",
		}, nil
	}

	return &SyncStatus{NeedsSync: false}, nil
}

func (c *Client) StackExists(name string) bool {
	configPath := filepath.Join(c.getStackDir(name), "config.json")
	_, err := os.Stat(configPath)
	return err == nil
}

func (c *Client) ListStacks() ([]*Stack, error) {
	stacksRoot := c.getStacksRootDir()

	if _, err := os.Stat(stacksRoot); os.IsNotExist(err) {
		return []*Stack{}, nil
	}

	entries, err := os.ReadDir(stacksRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read stacks directory: %w", err)
	}

	var stacks []*Stack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if stack, err := c.LoadStack(entry.Name()); err == nil {
			stacks = append(stacks, stack)
		}
	}

	return stacks, nil
}

// GetStackContext returns the stack context based on the current git branch.
// This is the single source of truth for what stack you're working on.
func (c *Client) GetStackContext() (*StackContext, error) {
	currentBranch, err := c.git.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	stackName := ExtractStackName(currentBranch)
	if stackName != "" {
		return c.GetStackContextByName(stackName)
	}

	return &StackContext{}, nil
}

// GetStackContextByName loads stack context for a specific stack by name.
// This is useful for commands that operate on a stack without being on a stack branch.
func (c *Client) GetStackContextByName(name string) (*StackContext, error) {
	currentBranch, err := c.git.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	return c.getStackContextByName(name, currentBranch)
}

func (c *Client) getStackContextByName(name string, currentBranch string) (*StackContext, error) {
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

	res := &StackContext{
		StackName:     name,
		Stack:         stack,
		AllChanges:    allChanges,
		ActiveChanges: activeChanges,
	}

	if IsUUIDBranch(currentBranch) {
		currentStackName, uuid := ExtractUUIDFromBranch(currentBranch)
		res.stackActive = currentStackName == name
		res.currentUUID = uuid
		res.onUUIDBranch = true
		return res, nil
	} else if IsStackBranch(currentBranch) {
		currentStackName, uuid := ExtractUUIDFromBranch(currentBranch)
		if uuid != "TOP" {
			return nil, fmt.Errorf("unexpected stack branch format: %s", currentBranch)
		}
		res.stackActive = currentStackName == name

		if len(allChanges) > 0 {
			lastChange := allChanges[len(allChanges)-1]
			res.currentUUID = lastChange.UUID
		}
	}
	return res, nil
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

	// Calculate and set DesiredBase for each change based on stacking order
	username, err := common.GetUsername()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get username: %w", err)
	}

	for i := range activeChanges {
		if i == 0 {
			// First change targets the stack's base branch
			activeChanges[i].DesiredBase = s.Base
		} else {
			// Subsequent changes target the previous change's PR branch
			prevChange := &activeChanges[i-1]
			prevBranch := fmt.Sprintf("%s/stack-%s/%s", username, s.Name, prevChange.UUID)
			activeChanges[i].DesiredBase = prevBranch
		}
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

// SetLocalDraft sets the LocalDraftStatus preference for a change by UUID
func (c *Client) SetLocalDraft(stackName string, uuid string, localDraft bool) error {
	prData, err := c.LoadPRs(stackName)
	if err != nil {
		return err
	}

	pr, exists := prData.PRs[uuid]
	if !exists {
		// Create new PR entry with just LocalDraftStatus set
		pr = &PR{
			LocalDraftStatus: localDraft,
		}
	} else {
		pr.LocalDraftStatus = localDraft
	}
	prData.PRs[uuid] = pr

	return c.SavePRs(stackName, prData)
}

type MarkChangeStatusResult struct {
	SyncedToGitHub bool
	PRNumber       int
}

func (c *Client) MarkChangeDraft(stackName string, change *Change) (*MarkChangeStatusResult, error) {
	return c.markChangeStatus(stackName, change, true)
}

func (c *Client) MarkChangeReady(stackName string, change *Change) (*MarkChangeStatusResult, error) {
	return c.markChangeStatus(stackName, change, false)
}

func (c *Client) markChangeStatus(stackName string, change *Change, isDraft bool) (*MarkChangeStatusResult, error) {
	result := &MarkChangeStatusResult{}

	if !change.IsLocal() && (change.PR.State == "open" || change.PR.State == "draft") {
		var err error
		if isDraft {
			err = c.gh.MarkPRDraft(change.PR.PRNumber)
		} else {
			err = c.gh.MarkPRReady(change.PR.PRNumber)
		}
		if err != nil {
			status := "ready"
			if isDraft {
				status = "draft"
			}
			return nil, fmt.Errorf("failed to mark PR #%d as %s on GitHub: %w", change.PR.PRNumber, status, err)
		}

		prData, err := c.LoadPRs(stackName)
		if err != nil {
			return nil, err
		}

		pr := prData.PRs[change.UUID]
		pr.LocalDraftStatus = isDraft
		pr.RemoteDraftStatus = isDraft
		if isDraft {
			pr.State = "draft"
		} else {
			pr.State = "open"
		}
		prData.PRs[change.UUID] = pr

		if err := c.SavePRs(stackName, prData); err != nil {
			return nil, fmt.Errorf("failed to update local state: %w", err)
		}

		result.SyncedToGitHub = true
		result.PRNumber = change.PR.PRNumber
	} else {
		if err := c.SetLocalDraft(stackName, change.UUID, isDraft); err != nil {
			return nil, err
		}
		result.SyncedToGitHub = false
	}

	return result, nil
}

// SyncPRFromGitHub syncs PR information from GitHub to local storage
func (c *Client) SyncPRFromGitHub(data PRSyncData) error {
	prData, err := c.LoadPRs(data.StackName)
	if err != nil {
		return err
	}

	pr, exists := prData.PRs[data.UUID]
	if !exists {
		pr = &PR{
			CreatedAt:        data.GitHubPR.CreatedAt,
			LocalDraftStatus: true, // Default to draft for new PRs
		}
	}
	// If PR already exists, preserve existing LocalDraftStatus value

	pr.PRNumber = data.GitHubPR.Number
	pr.URL = data.GitHubPR.URL
	pr.State = data.GitHubPR.State
	pr.Branch = data.Branch
	pr.CommitHash = data.CommitHash
	pr.LastPushed = data.GitHubPR.UpdatedAt
	pr.Title = data.Title
	pr.Body = data.Body
	pr.Base = data.Base
	pr.RemoteDraftStatus = data.RemoteDraftStatus

	prData.PRs[data.UUID] = pr

	return c.SavePRs(data.StackName, prData)
}

// RefreshResult contains the results of a refresh operation
type RefreshResult struct {
	MergedCount    int      // Number of PRs that were merged
	RemainingCount int      // Number of PRs still active
	MergedChanges  []Change // The changes that were merged
}

// SyncPRMetadata queries GitHub and updates local metadata without modifying git state.
// This is safe to call from any branch with any working tree state.
// Returns info about what changed (merged PRs, etc).
func (c *Client) SyncPRMetadata(stackCtx *StackContext) (*RefreshResult, error) {
	if len(stackCtx.AllChanges) == 0 {
		if err := c.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: 0,
			MergedChanges:  nil,
		}, nil
	}

	var prNumbers []int
	for _, change := range stackCtx.AllChanges {
		if !change.IsLocal() {
			prNumbers = append(prNumbers, change.PR.PRNumber)
		}
	}

	if len(prNumbers) == 0 {
		if err := c.UpdateSyncMetadata(stackCtx.StackName); err != nil {
			return nil, err
		}
		return &RefreshResult{
			MergedCount:    0,
			RemainingCount: len(stackCtx.ActiveChanges),
			MergedChanges:  nil,
		}, nil
	}

	result, err := c.gh.BatchGetPRs(stackCtx.Stack.Owner, stackCtx.Stack.RepoName, prNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to batch query PRs: %w", err)
	}

	// Load PR data to update
	prData, err := c.LoadPRs(stackCtx.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to load PRs: %w", err)
	}

	// Update ALL PR metadata from GitHub (not just merged ones)
	for _, change := range stackCtx.AllChanges {
		if change.IsLocal() {
			continue
		}

		prState, found := result.PRStates[change.PR.PRNumber]
		if !found {
			// PR was deleted or not found - skip
			continue
		}

		// Update the PR metadata in prData
		pr := prData.PRs[change.UUID]
		if pr != nil {
			pr.State = strings.ToLower(prState.State)
			pr.RemoteDraftStatus = prState.IsDraft
			// Preserve pr.LocalDraftStatus - don't overwrite user's preference!
		}
	}

	if err := c.SavePRs(stackCtx.StackName, prData); err != nil {
		return nil, fmt.Errorf("failed to save PRs: %w", err)
	}

	var newlyMerged []Change
	for _, change := range stackCtx.AllChanges {
		if change.IsLocal() {
			continue
		}

		prState, found := result.PRStates[change.PR.PRNumber]
		if !found {
			continue
		}

		if prState.IsMerged && !c.IsChangeMerged(&change) {
			mergedChange := change
			mergedChange.IsMerged = true
			mergedChange.MergedAt = prState.MergedAt
			newlyMerged = append(newlyMerged, mergedChange)
		}
	}

	mergedPRNumbers := make(map[int]bool)
	for _, change := range newlyMerged {
		mergedPRNumbers[change.PR.PRNumber] = true
	}
	if err := ValidateBottomUpMerges(stackCtx.ActiveChanges, mergedPRNumbers); err != nil {
		return nil, err
	}

	if err := c.SaveMergedChanges(stackCtx.StackName, newlyMerged); err != nil {
		return nil, fmt.Errorf("failed to save merged changes: %w", err)
	}

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

// ApplyRefresh applies a refresh by rebasing the TOP branch onto the latest base.
// Requires: current branch is TOP, no uncommitted changes.
// This performs the git operations to actually apply merged PR removals.
func (c *Client) ApplyRefresh(stackCtx *StackContext, merged []Change) error {
	// Validate on TOP branch (not editing a specific change)
	if !stackCtx.IsStack() || stackCtx.OnUUIDBranch() {
		currentBranch, _ := c.git.GetCurrentBranch()
		return fmt.Errorf("must be on TOP branch (%s) to apply refresh, currently on %s",
			stackCtx.Stack.Branch, currentBranch)
	}

	hasChanges, err := c.git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working tree: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("cannot apply refresh with uncommitted changes - commit or stash first")
	}

	// Rebase TOP branch using Restack (handles fetch + update-ref + rebase)
	if err := c.Restack(stackCtx, RestackOptions{
		Onto:  stackCtx.Stack.Base,
		Fetch: true,
	}); err != nil {
		return fmt.Errorf("failed to rebase TOP: %w", err)
	}

	// Clean up UUID branches for merged PRs (non-fatal errors)
	_ = c.CleanupMergedBranches(stackCtx, merged)

	return nil
}

// RefreshStackMetadata syncs metadata from GitHub without staleness threshold.
// IMPORTANT: This is read-only - never performs git operations.
// Use for commands that need fresh state (edit, navigation, switch).
// Returns fresh context with updated metadata.
func (c *Client) RefreshStackMetadata(stackCtx *StackContext) (*StackContext, error) {
	// Always sync metadata (no staleness check)
	_, err := c.SyncPRMetadata(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Reload context with fresh metadata
	freshCtx, err := c.GetStackContextByName(stackCtx.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to reload stack context: %w", err)
	}

	return freshCtx, nil
}

// MaybeRefreshStackMetadata syncs metadata from GitHub only if staleness threshold exceeded.
// IMPORTANT: This is read-only - never performs git operations.
// Use for read-only display commands (list, status) where slight staleness is acceptable.
// Returns fresh context with updated metadata (or existing context if still fresh).
func (c *Client) MaybeRefreshStackMetadata(stackCtx *StackContext) (*StackContext, error) {
	// Quick check: do we need sync?
	syncStatus, err := c.CheckSyncStatus(stackCtx.StackName)
	if err == nil && !syncStatus.NeedsSync {
		// Already fresh, no action needed
		return stackCtx, nil
	}

	// Sync metadata (no git operations)
	_, err = c.SyncPRMetadata(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// Reload context with fresh metadata
	freshCtx, err := c.GetStackContextByName(stackCtx.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to reload stack context: %w", err)
	}

	return freshCtx, nil
}

// IsChangeMerged returns true if a change has been merged on GitHub
func (c *Client) IsChangeMerged(change *Change) bool {
	return !change.IsLocal() && strings.ToLower(change.PR.State) == "merged"
}

// fetchRemote fetches from the remote repository
func (c *Client) fetchRemote() error {
	remote, err := c.git.GetRemoteName()
	if err != nil {
		return err
	}

	if err := c.git.Fetch(remote); err != nil {
		return err
	}

	return nil
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
		if err := c.fetchRemote(); err != nil {
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

	// If this is the topmost change, checkout the TOP branch instead of staying on UUID branch
	// This allows users to work on the entire stack when at the top position
	if change.Position == len(stackCtx.ActiveChanges) {
		if err := c.git.CheckoutBranch(stackCtx.Stack.Branch); err != nil {
			return "", fmt.Errorf("failed to checkout TOP branch: %w", err)
		}
		// Return the TOP branch name to indicate where we ended up
		return stackCtx.Stack.Branch, nil
	}

	return branchName, nil
}

// IsStackBranch checks if a branch name matches the stack branch pattern
func IsStackBranch(branch string) bool {
	// Stack branches follow pattern: username/stack-<name>/TOP
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return false
	}

	// Check second part starts with "stack-" and third part is "TOP"
	return strings.HasPrefix(parts[1], "stack-") && (parts[2] == "TOP" || validUUID(parts[2]))
}

// UpdateUUIDBranches reloads stack context and updates all UUID branches to point to their new commit locations
// Returns the number of branches that were actually updated
func (c *Client) UpdateUUIDBranches(stackName string) (int, error) {
	ctx, err := c.GetStackContextByName(stackName)
	if err != nil {
		return 0, fmt.Errorf("failed to reload stack context: %w", err)
	}

	username, err := common.GetUsername()
	if err != nil {
		return 0, fmt.Errorf("failed to get username: %w", err)
	}

	updatedCount := 0
	for i := range ctx.ActiveChanges {
		change := &ctx.ActiveChanges[i]

		if change.UUID == "" {
			continue
		}

		branchName := ctx.FormatUUIDBranch(username, change.UUID)

		if !c.git.BranchExists(branchName) {
			continue
		}

		currentHash, err := c.git.GetCommitHash(branchName)
		if err == nil && currentHash == change.CommitHash {
			continue
		}

		if err := c.git.UpdateRef(branchName, change.CommitHash); err != nil {
			return updatedCount, fmt.Errorf("failed to update branch %s: %w", branchName, err)
		}
		updatedCount++
	}

	return updatedCount, nil
}

// RebaseParams contains parameters for rebasing subsequent commits with recovery
type RebaseParams struct {
	StackName         string
	StackBranch       string
	OldCommitHash     string
	NewCommitHash     string
	OriginalStackHead string
}

// RebaseSubsequentCommitsWithRecovery rebases commits with automatic state save/clear for recovery
func (c *Client) RebaseSubsequentCommitsWithRecovery(params RebaseParams) (int, error) {
	rebaseState := RebaseState{
		OriginalStackHead: params.OriginalStackHead,
		NewCommitHash:     params.NewCommitHash,
		OldCommitHash:     params.OldCommitHash,
		StackBranch:       params.StackBranch,
	}
	if err := c.SaveRebaseState(params.StackName, rebaseState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save rebase state: %v\n", err)
	}

	gitClient, ok := c.git.(*git.Client)
	if !ok {
		return 0, fmt.Errorf("git client type assertion failed")
	}

	rebasedCount, err := gitClient.RebaseSubsequentCommits(
		params.StackBranch,
		params.OldCommitHash,
		params.NewCommitHash,
		params.OriginalStackHead,
	)
	if err != nil {
		return 0, err
	}

	if err := c.ClearRebaseState(params.StackName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear rebase state: %v\n", err)
	}

	return rebasedCount, nil
}

// ====================================================================
// Stack Deletion and Cleanup Operations
// ====================================================================

func (c *Client) ArchiveStack(stackName string) error {
	stackDir := c.getStackDir(stackName)

	if _, err := os.Stat(stackDir); os.IsNotExist(err) {
		return fmt.Errorf("stack '%s' does not exist", stackName)
	}

	archiveRoot := filepath.Join(c.getStacksRootDir(), ".archived")
	if err := os.MkdirAll(archiveRoot, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("%s-%s", stackName, timestamp)
	archivePath := filepath.Join(archiveRoot, archiveName)

	if err := os.Rename(stackDir, archivePath); err != nil {
		return fmt.Errorf("failed to archive stack: %w", err)
	}

	return nil
}

func (c *Client) GetStackBranches(username, stackName string) ([]string, error) {
	pattern := fmt.Sprintf("refs/heads/%s/stack-%s/*", username, stackName)
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", pattern)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list stack branches: %w", err)
	}

	branchesStr := strings.TrimSpace(string(output))
	if branchesStr == "" {
		return []string{}, nil
	}

	return strings.Split(branchesStr, "\n"), nil
}

func (c *Client) DeleteStack(stackName string, skipConfirm bool) error {
	stack, err := c.LoadStack(stackName)
	if err != nil {
		return fmt.Errorf("failed to load stack: %w", err)
	}

	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	branches, err := c.GetStackBranches(username, stackName)
	if err != nil {
		return fmt.Errorf("failed to get stack branches: %w", err)
	}

	if err := c.checkoutBaseBranchIfNeeded(stack, branches); err != nil {
		return err
	}

	if err := c.ArchiveStack(stackName); err != nil {
		return fmt.Errorf("failed to archive stack metadata: %w", err)
	}

	fmt.Printf("✓ Archived stack metadata to .git/stack/.archived/%s-*\n", stackName)

	c.deleteBranches(branches)
	return nil
}

func (c *Client) checkoutBaseBranchIfNeeded(stack *Stack, branches []string) error {
	currentBranch, err := c.git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	for _, branch := range branches {
		if branch == currentBranch {
			fmt.Printf("Currently on stack branch '%s', checking out base branch '%s'...\n", currentBranch, stack.Base)
			if err := c.git.CheckoutBranch(stack.Base); err != nil {
				return fmt.Errorf("failed to checkout base branch: %w", err)
			}
			break
		}
	}
	return nil
}

func (c *Client) deleteBranches(branches []string) {
	deletedLocal, deletedRemote := 0, 0

	for _, branch := range branches {
		if c.git.BranchExists(branch) {
			if err := c.git.DeleteBranch(branch, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete local branch %s: %v\n", branch, err)
			} else {
				deletedLocal++
			}
		}

		if err := c.git.DeleteRemoteBranch(branch); err != nil {
			if !strings.Contains(err.Error(), "remote ref does not exist") {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete remote branch %s: %v\n", branch, err)
			}
		} else {
			deletedRemote++
		}
	}

	if deletedLocal > 0 {
		fmt.Printf("✓ Deleted %d local branch(es)\n", deletedLocal)
	}
	if deletedRemote > 0 {
		fmt.Printf("✓ Deleted %d remote branch(es)\n", deletedRemote)
	}
}

type CleanupCandidate struct {
	Stack       *Stack
	Reason      string
	ChangeCount int
}

func (c *Client) IsStackEligibleForCleanup(stackCtx *StackContext) (bool, string) {
	if len(stackCtx.ActiveChanges) == 0 {
		return true, "empty"
	}

	for i := range stackCtx.ActiveChanges {
		if !c.IsChangeMerged(&stackCtx.ActiveChanges[i]) {
			return false, ""
		}
	}

	return true, "all_merged"
}

func (c *Client) GetCleanupCandidates() ([]CleanupCandidate, error) {
	stacks, err := c.ListStacks()
	if err != nil {
		return nil, fmt.Errorf("failed to list stacks: %w", err)
	}

	var candidates []CleanupCandidate

	for _, s := range stacks {
		stackCtx, err := c.loadStackWithSync(s.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load stack %s: %v\n", s.Name, err)
			continue
		}

		if eligible, reason := c.IsStackEligibleForCleanup(stackCtx); eligible {
			candidates = append(candidates, CleanupCandidate{
				Stack:       s,
				Reason:      reason,
				ChangeCount: len(stackCtx.AllChanges),
			})
		}
	}

	return candidates, nil
}

func (c *Client) loadStackWithSync(name string) (*StackContext, error) {
	stackCtx, err := c.GetStackContextByName(name)
	if err != nil {
		return nil, err
	}

	if _, err := c.SyncPRMetadata(stackCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to sync stack %s with GitHub: %v\n", name, err)
	}

	return c.GetStackContextByName(name)
}
