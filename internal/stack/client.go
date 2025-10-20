package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Client provides stack operations
type Client struct {
	gitRoot string
}

// NewClient creates a new stack client
func NewClient(gitRoot string) *Client {
	return &Client{gitRoot: gitRoot}
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

// GetCurrentStack returns the name of the current stack
func (c *Client) GetCurrentStack() (string, error) {
	currentPath := filepath.Join(c.gitRoot, ".git", "stack", "current")
	data, err := os.ReadFile(currentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no current stack set")
		}
		return "", fmt.Errorf("failed to read current stack: %w", err)
	}

	return string(data), nil
}

// SetCurrentStack sets the current stack
func (c *Client) SetCurrentStack(name string) error {
	stacksRoot := c.GetStacksRootDir()
	if err := os.MkdirAll(stacksRoot, 0755); err != nil {
		return fmt.Errorf("failed to create stacks directory: %w", err)
	}

	currentPath := filepath.Join(stacksRoot, "current")
	if err := os.WriteFile(currentPath, []byte(name), 0644); err != nil {
		return fmt.Errorf("failed to write current stack: %w", err)
	}

	return nil
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
