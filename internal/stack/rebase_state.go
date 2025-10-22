package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RebaseState stores information needed to recover from a failed rebase operation
type RebaseState struct {
	OriginalStackHead string `json:"original_stack_head"` // HEAD before the operation started
	NewCommitHash     string `json:"new_commit_hash"`     // The new commit (e.g., amended commit)
	OldCommitHash     string `json:"old_commit_hash"`     // The old commit being replaced
	StackBranch       string `json:"stack_branch"`        // The stack branch name (e.g., user/stack-name/TOP)
	Timestamp         string `json:"timestamp"`           // When the operation started
}

// SaveRebaseState saves rebase state for recovery
func (c *Client) SaveRebaseState(stackName string, state RebaseState) error {
	stateDir := c.getStackDir(stackName)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, "rebase-state.json")

	if state.Timestamp == "" {
		state.Timestamp = time.Now().Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rebase state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write rebase state: %w", err)
	}

	return nil
}

// LoadRebaseState loads saved rebase state
func (c *Client) LoadRebaseState(stackName string) (*RebaseState, error) {
	statePath := filepath.Join(c.getStackDir(stackName), "rebase-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no rebase state found")
		}
		return nil, fmt.Errorf("failed to read rebase state: %w", err)
	}

	var state RebaseState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse rebase state: %w", err)
	}

	return &state, nil
}

// ClearRebaseState removes saved rebase state
func (c *Client) ClearRebaseState(stackName string) error {
	statePath := filepath.Join(c.getStackDir(stackName), "rebase-state.json")

	if err := os.Remove(statePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to remove rebase state: %w", err)
	}

	return nil
}

// HasRebaseState checks if rebase state exists for a stack
func (c *Client) HasRebaseState(stackName string) bool {
	statePath := filepath.Join(c.getStackDir(stackName), "rebase-state.json")
	_, err := os.Stat(statePath)
	return err == nil
}
