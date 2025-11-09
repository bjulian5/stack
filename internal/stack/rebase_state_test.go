package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveRebaseState(t *testing.T) {
	tests := []struct {
		name      string
		stackName string
		state     RebaseState
		expected  RebaseState
		setup     func(*testing.T, *Client)
	}{
		{
			name:      "Success",
			stackName: "test-stack",
			state: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-01-15T10:00:00Z",
			},
			expected: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-01-15T10:00:00Z",
			},
		},
		{
			name:      "AutoGeneratesTimestamp",
			stackName: "test-stack",
			state: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
			},
			expected: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "1999-12-31T19:00:00-05:00",
			},
		},
		{
			name:      "PreservesProvidedTimestamp",
			stackName: "test-stack",
			state: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-01-01T00:00:00Z",
			},
			expected: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-01-01T00:00:00Z",
			},
		},
		{
			name:      "CreatesDirectory",
			stackName: "new-stack",
			state: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-new-stack/TOP",
				Timestamp:         "2024-01-15T00:00:00Z",
			},
			expected: RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-new-stack/TOP",
				Timestamp:         "2024-01-15T00:00:00Z",
			},
			setup: func(t *testing.T, client *Client) {
				stackDir := client.getStackDir("new-stack")
				_, err := os.Stat(stackDir)
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name:      "OverwritesExisting",
			stackName: "test-stack",
			state: RebaseState{
				OriginalStackHead: "xyz000",
				NewCommitHash:     "uvw111",
				OldCommitHash:     "rst222",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-02-01T00:00:00Z",
			},
			expected: RebaseState{
				OriginalStackHead: "xyz000",
				NewCommitHash:     "uvw111",
				OldCommitHash:     "rst222",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-02-01T00:00:00Z",
			},
			setup: func(t *testing.T, client *Client) {
				initialState := RebaseState{
					OriginalStackHead: "abc123",
					NewCommitHash:     "def456",
					OldCommitHash:     "ghi789",
					StackBranch:       "user/stack-test/TOP",
					Timestamp:         "2024-01-01T00:00:00Z",
				}
				err := client.SaveRebaseState("test-stack", initialState)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &MockGithubClient{}
				stackClient := newTestStackClient(t, mockGithubClient)

				if tt.setup != nil {
					tt.setup(t, stackClient)
				}

				err := stackClient.SaveRebaseState(tt.stackName, tt.state)
				require.NoError(t, err)

				loaded, err := stackClient.LoadRebaseState(tt.stackName)
				require.NoError(t, err)

				assert.Equal(t, &tt.expected, loaded)

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestLoadRebaseState(t *testing.T) {
	tests := []struct {
		name        string
		stackName   string
		setup       func(*testing.T, *Client)
		expected    *RebaseState
		expectError error
	}{
		{
			name:      "Success",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client) {
				state := RebaseState{
					OriginalStackHead: "abc123",
					NewCommitHash:     "def456",
					OldCommitHash:     "ghi789",
					StackBranch:       "user/stack-test/TOP",
					Timestamp:         "2024-01-01T00:00:00Z",
				}
				err := client.SaveRebaseState("test-stack", state)
				require.NoError(t, err)
			},
			expected: &RebaseState{
				OriginalStackHead: "abc123",
				NewCommitHash:     "def456",
				OldCommitHash:     "ghi789",
				StackBranch:       "user/stack-test/TOP",
				Timestamp:         "2024-01-01T00:00:00Z",
			},
		},
		{
			name:        "NotExists",
			stackName:   "nonexistent-stack",
			expectError: fmt.Errorf("no rebase state found"),
		},
		{
			name:      "MalformedJSON",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client) {
				stackDir := client.getStackDir("test-stack")
				err := os.MkdirAll(stackDir, 0755)
				require.NoError(t, err)

				statePath := filepath.Join(stackDir, "rebase-state.json")
				err = os.WriteFile(statePath, []byte("invalid json{"), 0644)
				require.NoError(t, err)
			},
			expectError: fmt.Errorf("failed to parse rebase state"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &MockGithubClient{}

				stackClient := newTestStackClient(t, mockGithubClient)

				if tt.setup != nil {
					tt.setup(t, stackClient)
				}

				loaded, err := stackClient.LoadRebaseState(tt.stackName)

				if tt.expectError != nil {
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expected, loaded)
				}
				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestClearRebaseState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}
		stackClient := newTestStackClient(t, mockGithubClient)

		state := RebaseState{
			OriginalStackHead: "abc123",
			NewCommitHash:     "def456",
			OldCommitHash:     "ghi789",
			StackBranch:       "user/stack-test/TOP",
		}
		err := stackClient.SaveRebaseState("test-stack", state)
		require.NoError(t, err)

		err = stackClient.ClearRebaseState("test-stack")
		require.NoError(t, err)

		assert.False(t, stackClient.HasRebaseState("test-stack"))

		mockGithubClient.AssertExpectations(t)
	})
}

func TestHasRebaseState(t *testing.T) {
	tests := []struct {
		name         string
		stackName    string
		setup        func(*testing.T, *Client)
		expectExists bool
	}{
		{
			name:      "Exists",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client) {
				state := RebaseState{
					OriginalStackHead: "abc123",
					NewCommitHash:     "def456",
					OldCommitHash:     "ghi789",
					StackBranch:       "user/stack-test/TOP",
				}
				err := client.SaveRebaseState("test-stack", state)
				require.NoError(t, err)
			},
			expectExists: true,
		},
		{
			name:      "DirectoryExistsFileDoesNot",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client) {
				stackDir := client.getStackDir("test-stack")
				err := os.MkdirAll(stackDir, 0755)
				require.NoError(t, err)
			},
			expectExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &MockGithubClient{}

				stackClient := newTestStackClient(t, mockGithubClient)

				if tt.setup != nil {
					tt.setup(t, stackClient)
				}

				exists := stackClient.HasRebaseState(tt.stackName)
				assert.Equal(t, tt.expectExists, exists)

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}
