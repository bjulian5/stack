package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/ui"
)

// DefaultSyncThreshold is the time threshold after which a stack is considered stale
// and needs to be refreshed to check for merged PRs on GitHub
const DefaultSyncThreshold = 5 * time.Minute

var validStackNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// GitClient defines the git operations needed by Stack Client
type GitClient interface {
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

// GithubClient defines the GitHub operations needed by Stack Client
type GithubClient interface {
	GetRepoInfo() (owner string, repoName string, err error)
	MarkPRDraft(prNumber int) error
	MarkPRReady(prNumber int) error
	BatchGetPRs(owner, repoName string, prNumbers []int) (*gh.BatchPRsResult, error)
	UpdatePRComment(commentID string, body string) error
	ListPRComments(prNumber int) ([]gh.Comment, error)
	CreatePRComment(prNumber int, body string) (string, error)
}

// Client provides stack operations
type Client struct {
	git      GitClient
	gh       GithubClient
	gitRoot  string
	username string
}

// NewClient creates a new stack client
func NewClient(gitOps GitClient, ghClient GithubClient) *Client {
	// TODO: update callsites to handle error
	username, err := getUsername()
	if err != nil {
		panic(fmt.Sprintf("failed to get username: %v", err))
	}
	return &Client{
		git:      gitOps,
		gh:       ghClient,
		gitRoot:  gitOps.GitRoot(),
		username: username,
	}
}

func (c *Client) getStackDir(stackName string) string {
	return filepath.Join(c.gitRoot, ".git", "stack", stackName)
}

func (c *Client) getStacksRootDir() string {
	return filepath.Join(c.gitRoot, ".git", "stack")
}

func (c *Client) LoadStack(name string) (*model.Stack, error) {
	stackDir := c.getStackDir(name)
	configPath := filepath.Join(stackDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack config: %w", err)
	}

	var stack model.Stack
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

func (c *Client) SaveStack(stack *model.Stack) error {
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

func (c *Client) ListStacks() ([]*model.Stack, error) {
	stacksRoot := c.getStacksRootDir()

	if _, err := os.Stat(stacksRoot); os.IsNotExist(err) {
		return []*model.Stack{}, nil
	}

	entries, err := os.ReadDir(stacksRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read stacks directory: %w", err)
	}

	var stacks []*model.Stack
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
	stackName := extractStackName(currentBranch)
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

	// Load all changes (merged + active + stale merged)
	changes, err := c.getChangesForStack(stack)
	if err != nil {
		return nil, err
	}

	// Build the changes map as source of truth (indexed by UUID)
	changesMap := make(map[string]*model.Change)
	for _, change := range changes.All {
		if change.UUID != "" {
			changesMap[change.UUID] = change
		}
	}

	res := &StackContext{
		StackName:          name,
		Stack:              stack,
		changes:            changesMap,
		client:             c,
		AllChanges:         changes.All,
		ActiveChanges:      changes.Active,
		StaleMergedChanges: changes.StaleMerged,
		username:           c.username,
	}

	if isUUIDBranch(currentBranch) {
		currentStackName, uuid := extractUUIDFromBranch(currentBranch)
		res.stackActive = currentStackName == name
		res.currentUUID = uuid
		res.onUUIDBranch = true
		return res, nil
	} else if IsStackBranch(currentBranch) {
		currentStackName, uuid := extractUUIDFromBranch(currentBranch)
		if uuid != "TOP" {
			return nil, fmt.Errorf("unexpected stack branch format: %s", currentBranch)
		}
		res.stackActive = currentStackName == name

		if len(changes.All) > 0 {
			lastChange := changes.All[len(changes.All)-1]
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
func (c *Client) CreateStack(name string, baseBranch string) (*model.Stack, error) {
	// Check if stack already exists
	if c.StackExists(name) {
		return nil, fmt.Errorf("stack '%s' already exists", name)
	}

	// Get current branch as base if not specified
	if baseBranch == "" {
		return nil, fmt.Errorf("base branch is required")
	}

	if err := validateStackName(name); err != nil {
		return nil, err
	}

	// Format branch name
	branchName := formatStackBranch(c.username, name)

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

	baseRef, err := c.git.GetCommitHash(baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get base branch hash: %w", err)
	}

	// Create stack metadata
	s := &model.Stack{
		Name:          name,
		Branch:        branchName,
		Base:          baseBranch,
		Owner:         owner,
		RepoName:      repoName,
		Created:       time.Now(),
		BaseRef:       baseRef,
		MergedChanges: []model.Change{},
		LastSynced:    time.Time{},
		SyncHash:      baseRef,
	}

	if err := c.SaveStack(s); err != nil {
		return nil, fmt.Errorf("failed to save stack: %w", err)
	}

	return s, nil
}

func validateStackName(name string) error {
	if !validStackNameRegex.MatchString(name) {
		return fmt.Errorf("invalid stack name '%s': only letters, numbers, dots, underscores, and hyphens are allowed", name)
	}
	return nil
}

// StackChanges contains the various categories of changes in a stack.
type stackChanges struct {
	// All includes merged + active changes (deduplicated by UUID).
	All []*model.Change
	// Active includes only unmerged changes currently on the stack branch.
	Active []*model.Change
	// StaleMerged includes active changes that are merged on GitHub but still on the TOP branch (need refresh).
	StaleMerged []*model.Change
}

// getChangesForStack loads all changes for a stack
func (c *Client) getChangesForStack(s *model.Stack) (*stackChanges, error) {
	// Load PR tracking data
	prData, err := c.LoadPRs(s.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to load PRs: %w", err)
	}

	var allChanges []*model.Change
	var activeChanges []*model.Change
	var staleMergedChanges []*model.Change

	// Get merged changes from stack metadata (not from a git branch)
	// Convert to pointers
	mergedChanges := make([]*model.Change, 0)
	if s.MergedChanges != nil {
		for i := range s.MergedChanges {
			// Create a copy and convert to pointer
			change := s.MergedChanges[i]
			mergedChanges = append(mergedChanges, &change)
		}
	}

	// Update merged changes with current PR state from prData
	// This ensures merged changes reflect the latest PR information
	for i := range mergedChanges {
		if mergedChanges[i].UUID != "" {
			if pr, ok := prData.PRs[mergedChanges[i].UUID]; ok {
				mergedChanges[i].PR = pr
			}
		}
	}

	baseRef := s.BaseRef
	if baseRef == "" {
		baseRef = s.Base
	}

	activeCommits, err := c.git.GetCommits(s.Branch, baseRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get active commits: %w", err)
	}

	// Filter commits to only include those belonging to this stack
	filteredCommits := make([]git.Commit, 0, len(activeCommits))
	for _, commit := range activeCommits {
		stackName := commit.Message.Trailers["PR-Stack"]
		if stackName == s.Name {
			filteredCommits = append(filteredCommits, commit)
		}
	}

	for _, change := range c.commitsToChanges(filteredCommits, prData) {
		if change.UUID == "" {
			continue
		}
		if change.PR != nil && change.PR.IsMerged() {
			staleMergedChanges = append(staleMergedChanges, change)
		} else {
			activeChanges = append(activeChanges, change)
		}
	}

	numMergedPRs := 0
	if prData != nil {
		for _, pr := range prData.PRs {
			if pr.IsMerged() {
				numMergedPRs++
			}
		}
	}

	// Enumerate positions for merged changes (1-indexed)
	for i := range mergedChanges {
		mergedChanges[i].Position = i + 1
	}

	// Stale merged changes get sequential positions after non-deduplicated mergedChanges
	// Ones that overlap with mergedChanges will get deduplicated anyway
	for i := range staleMergedChanges {
		staleMergedChanges[i].Position = len(mergedChanges) + i + 1
	}

	// Renumber positions for active changes (1-indexed)
	for i := range activeChanges {
		activeChanges[i].Position = numMergedPRs + i + 1
		activeChanges[i].ActivePosition = i + 1
	}

	for i := range activeChanges {
		// Determine the correct base for this change by looking at what comes before it
		var desiredBase string

		if i == 0 {
			// First active change: check if there are merged changes before it
			if len(mergedChanges) > 0 {
				// Merged changes exist, so base off the stack's base branch
				// (the merged PRs have already been merged into the base)
				desiredBase = s.Base
			} else {
				// No merged changes, base off the stack's base
				desiredBase = s.Base
			}
		} else {
			// Subsequent active changes: base off the previous active change's PR branch
			prevChange := activeChanges[i-1]
			desiredBase = fmt.Sprintf("%s/stack-%s/%s", c.username, s.Name, prevChange.UUID)
		}

		activeChanges[i].DesiredBase = desiredBase
	}

	// Build AllChanges (merged first, then active)
	// Deduplicate by UUID to avoid showing the same PR twice if it appears in both
	// merged changes and active commits on the TOP branch
	allChanges = make([]*model.Change, 0, len(mergedChanges)+len(activeChanges))
	seenUUIDs := make(map[string]bool)

	// Add merged changes first
	for _, change := range mergedChanges {
		if !seenUUIDs[change.UUID] {
			allChanges = append(allChanges, change)
			seenUUIDs[change.UUID] = true
		}
	}

	// Add stale merged changes, skipping any we've already seen
	for _, change := range staleMergedChanges {
		if !seenUUIDs[change.UUID] {
			allChanges = append(allChanges, change)
			seenUUIDs[change.UUID] = true
		}
	}

	// Add active changes, skipping any we've already seen
	for _, change := range activeChanges {
		if !seenUUIDs[change.UUID] {
			allChanges = append(allChanges, change)
			seenUUIDs[change.UUID] = true
		}
	}

	return &stackChanges{
		All:         allChanges,
		Active:      activeChanges,
		StaleMerged: staleMergedChanges,
	}, nil
}

// commitsToChanges converts git commits to Changes with the specified merged status
func (c *Client) commitsToChanges(commits []git.Commit, prData *model.PRData) []*model.Change {
	changes := make([]*model.Change, len(commits))
	for i, commit := range commits {
		uuid := commit.Message.Trailers["PR-UUID"]
		var pr *model.PR

		if uuid != "" {
			if p, ok := prData.PRs[uuid]; ok {
				pr = p
			}
		}

		changes[i] = &model.Change{
			Title:       commit.Message.Title,
			Description: commit.Message.Body,
			CommitHash:  commit.Hash,
			UUID:        uuid,
			PR:          pr,
		}
	}

	return changes
}

// LoadPRs loads PR tracking data for a stack
func (c *Client) LoadPRs(stackName string) (*model.PRData, error) {
	stackDir := c.getStackDir(stackName)
	prsPath := filepath.Join(stackDir, "prs.json")

	// If file doesn't exist, return empty PRData with current version
	if _, err := os.Stat(prsPath); os.IsNotExist(err) {
		return &model.PRData{Version: 1, PRs: make(map[string]*model.PR)}, nil
	}

	data, err := os.ReadFile(prsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRs file: %w", err)
	}

	var prData model.PRData
	if err := json.Unmarshal(data, &prData); err != nil {
		return nil, fmt.Errorf("failed to parse PRs file: %w", err)
	}

	// Set default version if not present (files created before versioning)
	if prData.Version == 0 {
		prData.Version = 1
	}

	// Ensure the map is initialized even if the JSON was empty
	if prData.PRs == nil {
		prData.PRs = make(map[string]*model.PR)
	}

	return &prData, nil
}

// savePRs saves PR tracking data for a stack
func (c *Client) savePRs(stackName string, prData *model.PRData) error {
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

type MarkChangeStatusResult struct {
	SyncedToGitHub bool
	PRNumber       int
}

func (c *Client) MarkChangeDraft(stackCtx *StackContext, change *model.Change) (*MarkChangeStatusResult, error) {
	return c.markChangeStatus(stackCtx, change, true)
}

func (c *Client) MarkChangeReady(stackCtx *StackContext, change *model.Change) (*MarkChangeStatusResult, error) {
	return c.markChangeStatus(stackCtx, change, false)
}

func (c *Client) markChangeStatus(stackCtx *StackContext, change *model.Change, isDraft bool) (*MarkChangeStatusResult, error) {
	result := &MarkChangeStatusResult{}

	if !change.IsLocal() && (change.PR.State == "open" || change.PR.State == "draft") {
		if change.PR.LocalDraftStatus == change.PR.RemoteDraftStatus {
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
		}
		change.PR.RemoteDraftStatus = isDraft
		change.PR.LocalDraftStatus = isDraft

		if isDraft {
			change.PR.State = "draft"
		} else {
			change.PR.State = "open"
		}
		result.SyncedToGitHub = true
		result.PRNumber = change.PR.PRNumber
	} else {
		if change.PR == nil {
			change.PR = &model.PR{}
		}
		change.PR.LocalDraftStatus = isDraft
		result.SyncedToGitHub = false
	}

	if err := stackCtx.Save(); err != nil {
		return nil, fmt.Errorf("failed to save stack context: %w", err)
	}

	if err := c.SyncVisualizationComments(stackCtx); err != nil {
		return nil, fmt.Errorf("failed to sync visualization comments: %w", err)
	}

	return result, nil
}

// RefreshResult contains the results of a refresh operation
type RefreshResult struct {
	StaleMergedCount   int             // Number of PRs that were merged on GitHub but still on TOP (stale)
	RemainingCount     int             // Number of PRs still active
	StaleMergedChanges []*model.Change // The changes that were merged on GitHub but still on TOP (stale)
}

// SyncPRMetadata queries GitHub and updates local metadata without modifying git state.
// This is safe to call from any branch with any working tree state.
// Returns info about what changed (merged PRs, etc).
func (c *Client) SyncPRMetadata(stackCtx *StackContext) (*RefreshResult, error) {
	if len(stackCtx.AllChanges) == 0 {
		// Update sync metadata in Stack
		commitHash, err := c.git.GetCommitHash(stackCtx.Stack.Branch)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit hash: %w", err)
		}
		stackCtx.Stack.LastSynced = time.Now()
		stackCtx.Stack.SyncHash = commitHash

		// Save everything (PRs + Stack)
		if err := stackCtx.Save(); err != nil {
			return nil, fmt.Errorf("failed to save stack context: %w", err)
		}

		return &RefreshResult{
			StaleMergedCount:   0,
			RemainingCount:     0,
			StaleMergedChanges: nil,
		}, nil
	}

	var prNumbers []int
	for _, change := range stackCtx.AllChanges {
		if !change.IsLocal() {
			prNumbers = append(prNumbers, change.PR.PRNumber)
		}
	}

	if len(prNumbers) == 0 {
		// Update sync metadata in Stack
		commitHash, err := c.git.GetCommitHash(stackCtx.Stack.Branch)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit hash: %w", err)
		}
		stackCtx.Stack.LastSynced = time.Now()
		stackCtx.Stack.SyncHash = commitHash

		// Save everything (PRs + Stack)
		if err := stackCtx.Save(); err != nil {
			return nil, fmt.Errorf("failed to save stack context: %w", err)
		}

		return &RefreshResult{
			StaleMergedCount:   0,
			RemainingCount:     len(stackCtx.ActiveChanges),
			StaleMergedChanges: nil,
		}, nil
	}

	result, err := c.gh.BatchGetPRs(stackCtx.Stack.Owner, stackCtx.Stack.RepoName, prNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to batch query PRs: %w", err)
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

		// Update the PR metadata directly in the change
		if change.PR != nil {
			// Normalize state based on IsMerged flag
			// GitHub's state field returns "CLOSED" for merged PRs, but we need to distinguish
			// merged from closed. Use the IsMerged flag to set the canonical state.
			if prState.IsMerged {
				change.PR.State = "merged"
			} else {
				change.PR.State = strings.ToLower(prState.State)
			}
			change.PR.RemoteDraftStatus = prState.IsDraft
		}
	}

	// Calculate all merged changes
	var mergedChanges []*model.Change
	for _, change := range stackCtx.AllChanges {
		if change != nil && change.PR != nil && change.PR.IsMerged() {
			mergedChanges = append(mergedChanges, change)
		}
	}

	// Validate bottom-up merges
	mergedPRNumbers := make(map[int]bool)
	for _, change := range mergedChanges {
		mergedPRNumbers[change.PR.PRNumber] = true
	}
	if err := validateBottomUpMerges(stackCtx.AllChanges, mergedPRNumbers); err != nil {
		return nil, err
	}

	// Update merged changes in Stack
	stackCtx.Stack.MergedChanges = make([]model.Change, len(mergedChanges))
	for i, change := range mergedChanges {
		// Convert pointer back to value for storage
		stackCtx.Stack.MergedChanges[i] = *change
	}

	// Update sync metadata in Stack
	commitHash, err := c.git.GetCommitHash(stackCtx.Stack.Branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}
	stackCtx.Stack.LastSynced = time.Now()
	stackCtx.Stack.SyncHash = commitHash

	// Save everything (PRs + Stack)
	if err := stackCtx.Save(); err != nil {
		return nil, fmt.Errorf("failed to save stack context: %w", err)
	}

	remainingCount := len(stackCtx.ActiveChanges) - len(stackCtx.StaleMergedChanges)
	return &RefreshResult{
		StaleMergedCount:   len(stackCtx.StaleMergedChanges),
		RemainingCount:     remainingCount,
		StaleMergedChanges: stackCtx.StaleMergedChanges,
	}, nil
}

// ApplyRefresh applies a refresh by rebasing the TOP branch onto the latest base.
// Requires: current branch is TOP, no uncommitted changes.
// This performs the git operations to actually apply merged PR removals.
func (c *Client) ApplyRefresh(stackCtx *StackContext, merged []*model.Change) error {
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

	return nil
}

// RefreshStackMetadata syncs metadata from GitHub without staleness threshold.
// IMPORTANT: This is read-only - never performs git operations.
// Use for commands that need fresh state (edit, navigation, switch).
// Returns fresh context with updated metadata.
func (c *Client) RefreshStackMetadata(stackCtx *StackContext) (*StackContext, error) {
	// Always sync metadata (no staleness check)
	// This updates stackCtx in place and persists via stackCtx.Save()
	_, err := c.SyncPRMetadata(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// stackCtx is already fresh - no need to reload
	return stackCtx, nil
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
	// This updates stackCtx in place and persists via stackCtx.Save()
	_, err = c.SyncPRMetadata(stackCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync with GitHub: %w", err)
	}

	// stackCtx is already fresh - no need to reload
	return stackCtx, nil
}

// IsChangeMerged returns true if a change has been merged on GitHub
func (c *Client) IsChangeMerged(change *model.Change) bool {
	return !change.IsLocal() && change.PR != nil && strings.ToLower(change.PR.State) == "merged"
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
		ui.Info("Fetching from remote...")
		if err := c.fetchRemote(); err != nil {
			return fmt.Errorf("failed to fetch: %w", err)
		}

		if err := c.UpdateLocalBaseRef(targetBase); err != nil {
			// Non-fatal: show warning and continue
			ui.Warningf("could not update local base ref: %v", err)
		}
	}

	if err := c.git.Rebase(targetBase); err != nil {
		return err
	}

	ref, err := c.git.GetCommitHash(targetBase)
	if err != nil {
		return fmt.Errorf("failed to get target base hash: %w", err)
	}

	stackCtx.Stack.BaseRef = ref
	stackCtx.Stack.Base = targetBase
	if err := c.SaveStack(stackCtx.Stack); err != nil {
		return fmt.Errorf("failed to update stack metadata: %w", err)
	}

	// if _, err := c.UpdateUUIDBranches(stackCtx.StackName); err != nil {
	// 	return fmt.Errorf("failed to update UUID branches: %w", err)
	// }
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
		ui.Infof("Created local branch %s at %s", baseBranch, upstreamHash[:7])
		return nil
	}

	if upstreamHash != currentHash {
		if err := c.git.UpdateRef(baseBranch, upstreamHash); err != nil {
			return err
		}
		ui.Infof("Updating %s: %s..%s", baseBranch, currentHash[:7], upstreamHash[:7])
	}
	return nil
}

// CheckoutChangeForEditing checks out a UUID branch for the given change, creating it if needed.
// If the branch already exists but points to a different commit, it syncs it to the current commit.
// Returns the branch name that was checked out.
func (c *Client) CheckoutChangeForEditing(stackCtx *StackContext, change *model.Change) (string, error) {

	// Format UUID branch name
	branchName := stackCtx.FormatUUIDBranch(change.UUID)

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
			ui.Warningf("Synced branch to current commit (was at %s, now at %s)",
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
	if change.Position == len(stackCtx.AllChanges) {
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

	updatedCount := 0
	for _, change := range ctx.ActiveChanges {
		if change.UUID == "" {
			continue
		}

		branchName := ctx.FormatUUIDBranch(change.UUID)

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
		ui.Warningf("failed to save rebase state: %v", err)
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
		ui.Warningf("failed to clear rebase state: %v", err)
	}

	return rebasedCount, nil
}

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

func (c *Client) GetStackBranches(stackName string) ([]string, error) {
	pattern := fmt.Sprintf("refs/heads/%s/stack-%s/*", c.username, stackName)
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

	branches, err := c.GetStackBranches(stackName)
	if err != nil {
		return fmt.Errorf("failed to get stack branches: %w", err)
	}

	if err := c.checkoutBaseBranchIfNeeded(stack, branches); err != nil {
		return err
	}

	if err := c.ArchiveStack(stackName); err != nil {
		return fmt.Errorf("failed to archive stack metadata: %w", err)
	}

	ui.Successf("Archived stack metadata to .git/stack/.archived/%s-*", stackName)

	c.deleteBranches(branches)
	return nil
}

func (c *Client) checkoutBaseBranchIfNeeded(stack *model.Stack, branches []string) error {
	currentBranch, err := c.git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	for _, branch := range branches {
		if branch == currentBranch {
			ui.Infof("Currently on stack branch '%s', checking out base branch '%s'...", currentBranch, stack.Base)
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
				ui.Warningf("failed to delete local branch %s: %v", branch, err)
			} else {
				deletedLocal++
			}
		}

		if err := c.git.DeleteRemoteBranch(branch); err != nil {
			if !strings.Contains(err.Error(), "remote ref does not exist") {
				ui.Warningf("failed to delete remote branch %s: %v", branch, err)
			}
		} else {
			deletedRemote++
		}
	}

	if deletedLocal > 0 {
		ui.Successf("Deleted %d local branch(es)", deletedLocal)
	}
	if deletedRemote > 0 {
		ui.Successf("Deleted %d remote branch(es)", deletedRemote)
	}
}

type CleanupCandidate struct {
	Stack       *model.Stack
	Reason      string
	ChangeCount int
}

func (c *Client) IsStackEligibleForCleanup(stackCtx *StackContext) (bool, string) {
	if len(stackCtx.ActiveChanges) == 0 {
		return true, "empty"
	}

	for _, change := range stackCtx.ActiveChanges {
		if !c.IsChangeMerged(change) {
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
			ui.Warningf("failed to load stack %s: %v", s.Name, err)
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
		ui.Warningf("failed to sync stack %s with GitHub: %v", name, err)
	}

	// stackCtx is already fresh after SyncPRMetadata - no need to reload
	return stackCtx, nil
}

// getUsername returns the username for branch naming
func getUsername() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return currentUser.Username, nil
}
