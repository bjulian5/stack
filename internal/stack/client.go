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
}

// Client provides stack operations
type Client struct {
	git     GitOperations
	gitRoot string
}

// NewClient creates a new stack client
func NewClient(gitOps GitOperations) *Client {
	return &Client{
		git:     gitOps,
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

	// Create stack metadata
	s := &Stack{
		Name:    name,
		Branch:  branchName,
		Base:    baseBranch,
		Created: time.Now(),
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
