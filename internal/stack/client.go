package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
)

// GitOperations defines the git operations needed by Stack Client
type GitOperations interface {
	GetCurrentBranch() (string, error)
	BranchExists(name string) bool
	CreateAndCheckoutBranch(name string) error
	CheckoutBranch(name string) error
	GetCommits(branch, base string) ([]git.Commit, error)
	GitRoot() string
}

// StackDetails contains comprehensive information about a stack
type StackDetails struct {
	Stack   *Stack
	Changes []Change
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

// GetStackDir returns the directory where stack metadata is stored
func (c *Client) GetStackDir(stackName string) string {
	return filepath.Join(c.gitRoot, ".git", "stack", stackName)
}

// GetStacksRootDir returns the root directory for all stacks
func (c *Client) GetStacksRootDir() string {
	return filepath.Join(c.gitRoot, ".git", "stack")
}

// LoadStack loads a stack configuration from disk
func (c *Client) LoadStack(name string) (*Stack, error) {
	stackDir := c.GetStackDir(name)
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
	stackDir := c.GetStackDir(stack.Name)

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

// StackExists checks if a stack exists
func (c *Client) StackExists(name string) bool {
	stackDir := c.GetStackDir(name)
	configPath := filepath.Join(stackDir, "config.json")
	_, err := os.Stat(configPath)
	return err == nil
}

// ListStacks returns all stacks in the repository
func (c *Client) ListStacks() ([]*Stack, error) {
	stacksRoot := c.GetStacksRootDir()

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

	// Check if on a stack branch (username/stack-name/TOP)
	if git.IsStackBranch(branch) {
		ctx.StackName = git.ExtractStackName(branch)

		// Load stack metadata
		if ctx.StackName != "" {
			stack, err := c.LoadStack(ctx.StackName)
			if err == nil {
				ctx.Stack = stack
			}
		}

		return ctx, nil
	}

	// Check if on a UUID branch (username/stack-name/uuid)
	if git.IsUUIDBranch(branch) {
		stackName, uuid := git.ExtractUUIDFromBranch(branch)
		ctx.StackName = stackName

		// Load stack metadata
		if ctx.StackName != "" {
			stack, err := c.LoadStack(ctx.StackName)
			if err == nil {
				ctx.Stack = stack
			}

			// Load the change being edited
			if uuid != "" && stack != nil {
				change, err := c.getChangeByUUID(stack, uuid)
				if err == nil {
					ctx.Change = change
				}
			}
		}

		return ctx, nil
	}

	// Not on a stack-related branch
	return ctx, nil
}

// getChangeByUUID finds the change with the given UUID in the stack
func (c *Client) getChangeByUUID(stack *Stack, uuid string) (*Change, error) {
	// Get all commits on the stack
	gitCommits, err := c.git.GetCommits(stack.Branch, stack.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	// Load PR tracking data
	prs, err := c.LoadPRs(stack.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to load PRs: %w", err)
	}

	// Find the commit with matching UUID
	for i, commit := range gitCommits {
		if commit.Trailers["PR-UUID"] == uuid {
			var pr *PR
			if p, ok := prs[uuid]; ok {
				pr = p
			}

			return &Change{
				Position:    i + 1,
				Title:       commit.Title,
				Description: commit.Body,
				CommitHash:  commit.Hash,
				UUID:        uuid,
				PR:          pr,
			}, nil
		}
	}

	return nil, fmt.Errorf("change with UUID %s not found", uuid)
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
	branchName := git.FormatStackBranch(username, name)

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

// GetStackDetails returns comprehensive details about a stack including all changes
func (c *Client) GetStackDetails(name string) (*StackDetails, error) {
	if name == "" {
		return nil, fmt.Errorf("stack name is required")
	}

	// Load stack metadata
	s, err := c.LoadStack(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load stack '%s': %w", name, err)
	}

	// Get commits from git
	gitCommits, err := c.git.GetCommits(s.Branch, s.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	// Load PR tracking data
	prs, err := c.LoadPRs(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load PRs: %w", err)
	}

	// Convert git commits to Changes
	changes := make([]Change, len(gitCommits))
	for i, commit := range gitCommits {
		uuid := commit.Trailers["PR-UUID"]
		var pr *PR
		if uuid != "" {
			if p, ok := prs[uuid]; ok {
				pr = p
			}
		}

		changes[i] = Change{
			Position:    i + 1,
			Title:       commit.Title,
			Description: commit.Body,
			CommitHash:  commit.Hash,
			UUID:        uuid,
			PR:          pr,
		}
	}

	return &StackDetails{
		Stack:   s,
		Changes: changes,
	}, nil
}

// DeleteStack deletes a stack and its metadata
func (c *Client) DeleteStack(name string) error {
	stackDir := c.GetStackDir(name)
	if err := os.RemoveAll(stackDir); err != nil {
		return fmt.Errorf("failed to delete stack directory: %w", err)
	}

	return nil
}

// LoadPRs loads PR tracking data for a stack
func (c *Client) LoadPRs(stackName string) (PRMap, error) {
	stackDir := c.GetStackDir(stackName)
	prsPath := filepath.Join(stackDir, "prs.json")

	// If file doesn't exist, return empty map
	if _, err := os.Stat(prsPath); os.IsNotExist(err) {
		return make(PRMap), nil
	}

	data, err := os.ReadFile(prsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRs file: %w", err)
	}

	var prs PRMap
	if err := json.Unmarshal(data, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PRs file: %w", err)
	}

	return prs, nil
}

// SavePRs saves PR tracking data for a stack
func (c *Client) SavePRs(stackName string, prs PRMap) error {
	stackDir := c.GetStackDir(stackName)

	// Create stack directory if it doesn't exist
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	prsPath := filepath.Join(stackDir, "prs.json")
	data, err := json.MarshalIndent(prs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRs: %w", err)
	}

	if err := os.WriteFile(prsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write PRs file: %w", err)
	}

	return nil
}

// GetPR gets PR information for a UUID
func (c *Client) GetPR(stackName string, uuid string) (*PR, error) {
	prs, err := c.LoadPRs(stackName)
	if err != nil {
		return nil, err
	}

	pr, ok := prs[uuid]
	if !ok {
		return nil, fmt.Errorf("PR not found for UUID %s", uuid)
	}

	return pr, nil
}

// SetPR sets PR information for a UUID
func (c *Client) SetPR(stackName string, uuid string, pr *PR) error {
	prs, err := c.LoadPRs(stackName)
	if err != nil {
		return err
	}

	prs[uuid] = pr

	return c.SavePRs(stackName, prs)
}

// DeletePR deletes PR information for a UUID
func (c *Client) DeletePR(stackName string, uuid string) error {
	prs, err := c.LoadPRs(stackName)
	if err != nil {
		return err
	}

	delete(prs, uuid)

	return c.SavePRs(stackName, prs)
}
