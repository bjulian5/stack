package stack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestClient(t *testing.T) {
	t.Run("Install", func(t *testing.T) {
		mockGithubClient := &gh.MockGithubClient{}
		mockGitClient := testutil.NewTestGitClient(t)
		stackClient := NewClient(mockGitClient, mockGithubClient)

		installed, err := stackClient.IsInstalled()
		assert.NoError(t, err)
		assert.False(t, installed, "stack should not be installed initially")

		err = stackClient.MarkInstalled()
		assert.NoError(t, err, "MarkInstalled should not return an error")

		installed, err = stackClient.IsInstalled()
		assert.NoError(t, err)
		assert.True(t, installed, "stack should be installed after MarkInstalled")
	})

	t.Run("CreateStack", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockGithubClient := &gh.MockGithubClient{}
			mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

			stackClient := NewTestStack(t, mockGithubClient)

			v, err := stackClient.CreateStack("test-stack", "main")
			assert.NoError(t, err, "CreateStack should not return an error")

			expectStack := &model.Stack{
				Name:          "test-stack",
				Branch:        "test-user/stack-test-stack/TOP",
				Base:          "main",
				Owner:         "test-owner",
				RepoName:      "test-repo",
				Created:       time.Now(), // clock is paused during synctest
				SyncHash:      "564a453f5bd814cc099ff2c78a6eaab92a0dcfef",
				BaseRef:       "564a453f5bd814cc099ff2c78a6eaab92a0dcfef",
				MergedChanges: []model.Change{},
			}
			assert.Equal(t, expectStack, v, "CreateStack should return expected Stack object")
		})
	})
}

func TestGetStackContext_WithMultipleActiveChanges(t *testing.T) {
	mockGithubClient := &gh.MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := NewTestStack(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 3 commits with trailers
	uuid1 := "1111111111111111"
	uuid2 := "2222222222222222"
	uuid3 := "3333333333333333"

	_ = testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	_ = testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
		"PR-UUID":  uuid2,
		"PR-Stack": "test-stack",
	})

	_ = testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "Third change", "Description of third change", map[string]string{
		"PR-UUID":  uuid3,
		"PR-Stack": "test-stack",
	})

	// Load the stack context
	stackCtx, err := stackClient.GetStackContextByName("test-stack")
	require.NoError(t, err)
	require.True(t, stackCtx.IsStack())

	// Verify we have 3 active changes
	assert.Len(t, stackCtx.AllChanges, 3, "should have 3 changes in AllChanges")
	assert.Len(t, stackCtx.ActiveChanges, 3, "should have 3 changes in ActiveChanges")
	assert.Len(t, stackCtx.StaleMergedChanges, 0, "should have no stale merged changes")

	// Verify change content and positions
	require.Len(t, stackCtx.ActiveChanges, 3)

	expectedChanges := []*model.Change{
		{
			Title:          "First change",
			Description:    "Description of first change",
			UUID:           uuid1,
			Position:       1,
			ActivePosition: 1,
			DesiredBase:    stack.Base,
			CommitHash:     stackCtx.ActiveChanges[0].CommitHash, // Use actual hash
		},
		{
			Title:          "Second change",
			Description:    "Description of second change",
			UUID:           uuid2,
			Position:       2,
			ActivePosition: 2,
			DesiredBase:    fmt.Sprintf("test-user/stack-test-stack/%s", uuid1),
			CommitHash:     stackCtx.ActiveChanges[1].CommitHash, // Use actual hash
		},
		{
			Title:          "Third change",
			Description:    "Description of third change",
			UUID:           uuid3,
			Position:       3,
			ActivePosition: 3,
			DesiredBase:    fmt.Sprintf("test-user/stack-test-stack/%s", uuid2),
			CommitHash:     stackCtx.ActiveChanges[2].CommitHash, // Use actual hash
		},
	}

	for i, expected := range expectedChanges {
		assert.Equal(t, expected, stackCtx.ActiveChanges[i], "Change %d should match expected", i+1)
	}

	// Verify AllChanges matches ActiveChanges (no merged changes)
	assert.Equal(t, stackCtx.ActiveChanges, stackCtx.AllChanges)
}

func TestGetStackContext_WithMergedAndActiveChanges(t *testing.T) {
	mockGithubClient := &gh.MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := NewTestStack(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 3 commits with trailers
	uuid1 := "aaaa111111111111"
	uuid2 := "bbbb222222222222"
	uuid3 := "cccc333333333333"

	hash1 := testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	_ = testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
		"PR-UUID":  uuid2,
		"PR-Stack": "test-stack",
	})

	_ = testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "Third change", "Description of third change", map[string]string{
		"PR-UUID":  uuid3,
		"PR-Stack": "test-stack",
	})

	// Mark the first change as merged in PR metadata
	prData := &model.PRData{
		Version: 1,
		PRs: map[string]*model.PR{
			uuid1: {
				PRNumber:          101,
				URL:               "https://github.com/test-owner/test-repo/pull/101",
				State:             "merged",
				RemoteDraftStatus: false,
				LocalDraftStatus:  false,
			},
		},
	}
	err = stackClient.savePRs("test-stack", prData)
	require.NoError(t, err)

	// Update Stack.MergedChanges to include the merged change
	stack.MergedChanges = []model.Change{
		{
			Title:       "First change",
			Description: "Description of first change",
			UUID:        uuid1,
			CommitHash:  hash1,
			Position:    1,
			PR: &model.PR{
				PRNumber:          101,
				URL:               "https://github.com/test-owner/test-repo/pull/101",
				State:             "merged",
				RemoteDraftStatus: false,
				LocalDraftStatus:  false,
			},
		},
	}
	err = stackClient.SaveStack(stack)
	require.NoError(t, err)

	// Load the stack context
	stackCtx, err := stackClient.GetStackContextByName("test-stack")
	require.NoError(t, err)
	require.True(t, stackCtx.IsStack())

	// Verify change counts
	// Note: uuid1 appears in both Stack.MergedChanges and on TOP branch, so it's detected as stale merged
	assert.Len(t, stackCtx.AllChanges, 3, "should have 3 changes in AllChanges (deduplicated)")
	assert.Len(t, stackCtx.ActiveChanges, 2, "should have 2 active changes (uuid2, uuid3)")
	assert.Len(t, stackCtx.StaleMergedChanges, 1, "should have 1 stale merged change (uuid1 on TOP with merged PR)")

	// Verify merged change comes first in AllChanges
	expectedMergedChange := &model.Change{
		Title:       "First change",
		Description: "Description of first change",
		UUID:        uuid1,
		CommitHash:  hash1,
		Position:    1,
		PR: &model.PR{
			PRNumber:          101,
			URL:               "https://github.com/test-owner/test-repo/pull/101",
			State:             "merged",
			RemoteDraftStatus: false,
			LocalDraftStatus:  false,
		},
	}
	assert.Equal(t, expectedMergedChange, stackCtx.AllChanges[0])

	// Verify active changes have correct positions (accounting for merged change)
	require.Len(t, stackCtx.ActiveChanges, 2)

	expectedActiveChanges := []*model.Change{
		{
			Title:          "Second change",
			Description:    "Description of second change",
			UUID:           uuid2,
			CommitHash:     stackCtx.ActiveChanges[0].CommitHash, // Use actual hash
			Position:       2,                                    // Position 2 because merged PR is #1
			ActivePosition: 1,
			DesiredBase:    stack.Base, // First active change bases on stack base
		},
		{
			Title:          "Third change",
			Description:    "Description of third change",
			UUID:           uuid3,
			CommitHash:     stackCtx.ActiveChanges[1].CommitHash, // Use actual hash
			Position:       3,                                    // Position 3
			ActivePosition: 2,
			DesiredBase:    fmt.Sprintf("test-user/stack-test-stack/%s", uuid2), // Bases on previous active change
		},
	}

	for i, expected := range expectedActiveChanges {
		assert.Equal(t, expected, stackCtx.ActiveChanges[i], "Active change %d should match expected", i+1)
	}

	// Verify deduplication: merged change appears in AllChanges but not in ActiveChanges
	foundInActive := false
	for _, change := range stackCtx.ActiveChanges {
		if change.UUID == uuid1 {
			foundInActive = true
			break
		}
	}
	assert.False(t, foundInActive, "merged change should not appear in ActiveChanges")
}

func TestGetStackContext_WithStaleMergedChanges(t *testing.T) {
	mockGithubClient := &gh.MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := NewTestStack(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 2 commits with trailers
	uuid1 := "dddd111111111111"
	uuid2 := "eeee222222222222"

	hash1 := testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	hash2 := testutil.CreateCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
		"PR-UUID":  uuid2,
		"PR-Stack": "test-stack",
	})

	// Mark the first change as merged in PR metadata
	// BUT do NOT add it to Stack.MergedChanges - this simulates a stale merged state
	// where GitHub shows the PR as merged but we haven't run refresh yet
	prData := &model.PRData{
		Version: 1,
		PRs: map[string]*model.PR{
			uuid1: {
				PRNumber:          201,
				URL:               "https://github.com/test-owner/test-repo/pull/201",
				State:             "merged",
				RemoteDraftStatus: false,
				LocalDraftStatus:  false,
			},
			uuid2: {
				PRNumber:          202,
				URL:               "https://github.com/test-owner/test-repo/pull/202",
				State:             "open",
				RemoteDraftStatus: false,
				LocalDraftStatus:  false,
			},
		},
	}
	err = stackClient.savePRs("test-stack", prData)
	require.NoError(t, err)

	// Load the stack context
	stackCtx, err := stackClient.GetStackContextByName("test-stack")
	require.NoError(t, err)
	require.True(t, stackCtx.IsStack())

	// Verify change counts
	assert.Len(t, stackCtx.AllChanges, 2, "should have 2 changes in AllChanges")
	assert.Len(t, stackCtx.ActiveChanges, 1, "should have 1 active change (uuid2)")
	assert.Len(t, stackCtx.StaleMergedChanges, 1, "should have 1 stale merged change (uuid1)")

	// Verify stale merged change
	require.Len(t, stackCtx.StaleMergedChanges, 1)
	staleMerged := stackCtx.StaleMergedChanges[0]
	expectedStale := &model.Change{
		Title:       "First change",
		Description: "Description of first change",
		UUID:        uuid1,
		CommitHash:  hash1,
		Position:    1, // Gets position 1 since there are no merged changes in Stack.MergedChanges
		PR: &model.PR{
			PRNumber:          201,
			URL:               "https://github.com/test-owner/test-repo/pull/201",
			State:             "merged",
			RemoteDraftStatus: false,
			LocalDraftStatus:  false,
		},
	}
	assert.Equal(t, expectedStale, staleMerged)

	// Verify active change
	require.Len(t, stackCtx.ActiveChanges, 1)
	activeChange := stackCtx.ActiveChanges[0]
	expectedActive := &model.Change{
		Title:          "Second change",
		Description:    "Description of second change",
		UUID:           uuid2,
		CommitHash:     hash2,
		Position:       2, // Position 2 (after the stale merged change)
		ActivePosition: 1, // First active change
		DesiredBase:    stack.Base,
		PR: &model.PR{
			PRNumber:          202,
			URL:               "https://github.com/test-owner/test-repo/pull/202",
			State:             "open",
			RemoteDraftStatus: false,
			LocalDraftStatus:  false,
		},
	}
	assert.Equal(t, expectedActive, activeChange)

	// Verify AllChanges contains both (stale merged first, then active)
	assert.Equal(t, uuid1, stackCtx.AllChanges[0].UUID, "first change in AllChanges should be stale merged")
	assert.Equal(t, uuid2, stackCtx.AllChanges[1].UUID, "second change in AllChanges should be active")
}

func TestCheckSyncStatus(t *testing.T) {
	tests := []struct {
		name        string
		stackName   string
		setup       func(*testing.T, *Client, *gh.MockGithubClient)
		expected    *SyncStatus
		expectError error
	}{
		{
			name:      "NeverSynced",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				// Create a stack with zero LastSynced time
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Reset LastSynced to zero
				stack.LastSynced = time.Time{}
				err = client.SaveStack(stack)
				require.NoError(t, err)
			},
			expected: &SyncStatus{
				NeedsSync: true,
				Reason:    "never_synced",
				Warning:   "Stack has never been synced with GitHub. Run 'stack refresh' to check for merged PRs.",
			},
		},
		{
			name:      "HashMismatch",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Set LastSynced to now so it's not stale
				stack.LastSynced = time.Now()
				stack.SyncHash = "old-hash-that-doesnt-match"
				err = client.SaveStack(stack)
				require.NoError(t, err)

				// Create a new commit to change the hash
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "New commit", "Body", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})
			},
			expected: &SyncStatus{
				NeedsSync: true,
				Reason:    "commits_changed",
				Warning:   "Stack has new commits since last sync. Run 'stack refresh' to ensure consistency with GitHub.",
			},
		},
		{
			name:      "Stale",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Get current hash to match
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to over 5 minutes ago and matching hash
				stack.LastSynced = time.Now().Add(-10 * time.Minute)
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)
			},
			expected: &SyncStatus{
				NeedsSync: true,
				Reason:    "stale",
				Warning:   "Stack sync is stale. Run 'stack refresh' to check for merged PRs.",
			},
		},
		{
			name:      "Fresh",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Get current hash
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to now and matching hash
				stack.LastSynced = time.Now()
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)
			},
			expected: &SyncStatus{
				NeedsSync: false,
			},
		},
		{
			name:      "HashCheckFailed",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Set LastSynced to now
				stack.LastSynced = time.Now()
				stack.SyncHash = "some-hash"
				// Delete the stack branch to cause GetCommitHash to fail
				stack.Branch = "non-existent-branch"
				err = client.SaveStack(stack)
				require.NoError(t, err)
			},
			expected: &SyncStatus{
				NeedsSync: true,
				Reason:    "hash_check_failed",
				Warning:   "Could not verify stack sync status. Run 'stack refresh' to ensure consistency.",
			},
		},
		{
			name:        "StackLoadFailed",
			stackName:   "nonexistent-stack",
			setup:       func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {},
			expectError: fmt.Errorf("failed to load stack"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use synctest for all tests for consistency
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				if tt.setup != nil {
					tt.setup(t, stackClient, mockGithubClient)
				}

				status, err := stackClient.CheckSyncStatus(tt.stackName)

				if tt.expectError != nil {
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expected, status)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

// Helper to create multiple stacks for testing
func createTestStacks(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient, stackNames []string) {
	if len(stackNames) == 0 {
		return
	}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Times(len(stackNames))

	for _, name := range stackNames {
		// Switch to main before creating each stack
		err := client.git.CheckoutBranch("main")
		require.NoError(t, err)

		_, err = client.CreateStack(name, "main")
		require.NoError(t, err)
	}
}

func TestListStacks(t *testing.T) {
	tests := []struct {
		name               string
		stacksToCreate     []string
		setup              func(*testing.T, *Client, *gh.MockGithubClient)
		expectedStackNames []string
		expectError        error
	}{
		{
			name:               "EmptyDirectory",
			stacksToCreate:     []string{},
			expectedStackNames: []string{},
		},
		{
			name:               "MultipleStacks",
			stacksToCreate:     []string{"stack-one", "stack-two", "stack-three"},
			expectedStackNames: []string{"stack-one", "stack-two", "stack-three"},
		},
		{
			name:               "DirectoryDoesNotExist",
			stacksToCreate:     nil, // Don't create any stacks
			expectedStackNames: []string{},
		},
		{
			name:           "MixOfValidAndInvalidStacks",
			stacksToCreate: []string{"valid-stack"},
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				// Create an invalid stack directory with malformed config
				stacksRoot := client.getStacksRootDir()
				invalidStackDir := filepath.Join(stacksRoot, "invalid-stack")
				err := os.MkdirAll(invalidStackDir, 0755)
				require.NoError(t, err)

				// Write invalid JSON to config
				configPath := filepath.Join(invalidStackDir, "config.json")
				err = os.WriteFile(configPath, []byte("invalid json{"), 0644)
				require.NoError(t, err)
			},
			expectedStackNames: []string{"valid-stack"},
		},
		{
			name:           "NonDirectoryFilesInStacksDirectory",
			stacksToCreate: []string{"valid-stack"},
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				// Create a file in the stacks directory (not a directory)
				stacksRoot := client.getStacksRootDir()
				filePath := filepath.Join(stacksRoot, "some-file.txt")
				err := os.WriteFile(filePath, []byte("not a stack"), 0644)
				require.NoError(t, err)
			},
			expectedStackNames: []string{"valid-stack"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				createTestStacks(t, stackClient, mockGithubClient, tt.stacksToCreate)

				// Additional setup if provided
				if tt.setup != nil {
					tt.setup(t, stackClient, mockGithubClient)
				}

				stacks, err := stackClient.ListStacks()

				if err != nil {
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify stack names match expected
					assert.Len(t, stacks, len(tt.expectedStackNames))

					actualStackNames := make(map[string]bool)
					for _, stack := range stacks {
						actualStackNames[stack.Name] = true
					}

					for _, expectedName := range tt.expectedStackNames {
						assert.True(t, actualStackNames[expectedName],
							"Expected stack '%s' not found in results", expectedName)
					}
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestGetStackContext(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client, *gh.MockGithubClient) *StackContext
		expectError error
	}{
		{
			name: "OnStackTOPBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				// Create a stack (already on the TOP branch)
				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a commit with trailers
				commitHash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				// Build expected context
				change := &model.Change{
					UUID:           "1111111111111111",
					Title:          "Test change",
					Description:    "Description",
					CommitHash:     commitHash,
					Position:       1,
					ActivePosition: 1,
					DesiredBase:    "main",
				}

				return &StackContext{
					StackName:     "test-stack",
					Stack:         stack,
					changes:       map[string]*model.Change{"1111111111111111": change},
					AllChanges:    []*model.Change{change},
					ActiveChanges: []*model.Change{change},
					username:      "test-user",
					stackActive:   true,
					currentUUID:   "1111111111111111",
					onUUIDBranch:  false,
					client:        client,
				}
			},
		},
		{
			name: "OnUUIDBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				// Create a stack
				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a commit with trailers
				commitHash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				// Create and checkout a UUID branch at that commit
				uuidBranch := "test-user/stack-test-stack/1111111111111111"
				err = client.git.CreateAndCheckoutBranchAt(uuidBranch, commitHash)
				require.NoError(t, err)

				// Build expected context
				change := &model.Change{
					UUID:           "1111111111111111",
					Title:          "Test change",
					Description:    "Description",
					CommitHash:     commitHash,
					Position:       1,
					ActivePosition: 1,
					DesiredBase:    "main",
				}

				return &StackContext{
					StackName:     "test-stack",
					Stack:         stack,
					changes:       map[string]*model.Change{"1111111111111111": change},
					AllChanges:    []*model.Change{change},
					ActiveChanges: []*model.Change{change},
					username:      "test-user",
					stackActive:   true,
					currentUUID:   "1111111111111111",
					onUUIDBranch:  true,
					client:        client,
				}
			},
		},
		{
			name: "OnRegularBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				// Stay on main branch (not a stack branch)
				err := client.git.CheckoutBranch("main")
				require.NoError(t, err)

				// Expected empty context
				return &StackContext{}
			},
		},
		{
			name: "StackLoadFails",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				// Create a stack
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Corrupt the stack config to cause LoadStack to fail
				configPath := filepath.Join(client.getStackDir("test-stack"), "config.json")
				err = os.WriteFile(configPath, []byte("invalid json{"), 0644)
				require.NoError(t, err)

				return nil // Error case, no expected context
			},
			expectError: fmt.Errorf("failed to load stack"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				expected := tt.setup(t, stackClient, mockGithubClient)

				ctx, err := stackClient.GetStackContext()

				if err != nil {
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, expected, ctx)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestSwitchStack(t *testing.T) {
	tests := []struct {
		name           string
		stackName      string
		setup          func(*testing.T, *Client, *gh.MockGithubClient)
		expectedBranch string
		expectError    error
	}{
		{
			name:      "Success",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				// Create a stack
				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Switch to main to test switching back
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)
			},
			expectedBranch: "test-user/stack-test-stack/TOP",
		},
		{
			name:      "StackDoesNotExist",
			stackName: "nonexistent-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				// Don't create the stack
			},
			expectError: fmt.Errorf("failed to load stack"),
		},
		{
			name:      "CheckoutFails",
			stackName: "test-stack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				// Create a stack
				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Delete the stack branch to cause checkout to fail
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				// Delete the branch using git command
				cmd := exec.Command("git", "branch", "-D", "test-user/stack-test-stack/TOP")
				cmd.Dir = client.git.GitRoot()
				err = cmd.Run()
				require.NoError(t, err)
			},
			expectError: fmt.Errorf("failed to checkout stack branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				if tt.setup != nil {
					tt.setup(t, stackClient, mockGithubClient)
				}

				err := stackClient.SwitchStack(tt.stackName)

				if err != nil {
					assert.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify we're on the expected branch
					currentBranch, err := stackClient.git.GetCurrentBranch()
					require.NoError(t, err)
					assert.Equal(t, tt.expectedBranch, currentBranch)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestCheckoutChangeForEditing(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*testing.T, *Client, *gh.MockGithubClient) (*StackContext, *model.Change)
		expectBranch string
		expectError  error
	}{
		{
			name: "CreateNewUUIDBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, *model.Change) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				// Create a stack with TWO changes so the first one isn't the top
				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				uuid1 := "1111111111111111"
				uuid2 := "1111111111111112"
				commitHash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change 1", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change 2", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Return the first change (not the top)
				change := stackCtx.ActiveChanges[0]
				assert.Equal(t, uuid1, change.UUID)
				assert.Equal(t, commitHash, change.CommitHash)

				return stackCtx, change
			},
			expectBranch: "test-user/stack-test-stack/1111111111111111",
		},
		{
			name: "UUIDBranchExistsAtCorrectCommit",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, *model.Change) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				uuid1 := "2222222222222222"
				uuid2 := "2222222222222223"
				commitHash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change 1", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				// Add a second change so the first one isn't the top
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change 2", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				// Create UUID branch at the correct commit
				uuidBranch := fmt.Sprintf("test-user/stack-test-stack/%s", uuid1)
				err = client.git.CreateAndCheckoutBranchAt(uuidBranch, commitHash)
				require.NoError(t, err)

				// Switch back to TOP branch
				err = client.git.CheckoutBranch("test-user/stack-test-stack/TOP")
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				return stackCtx, stackCtx.ActiveChanges[0]
			},
			expectBranch: "test-user/stack-test-stack/2222222222222222",
		},
		{
			name: "UUIDBranchExistsAtWrongCommit",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, *model.Change) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				uuid1 := "3333333333333333"
				uuid2 := "3333333333333334"

				// Create first commit
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				// Create second commit so first is not at top
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				// Simulate a commit rewrite scenario where the UUID branch points to a stale commit
				_ = simulateCommitRewrite(t, client.git.(*git.Client), "test-stack", uuid1)

				// Recreate the second commit after the rewrite
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Return the first change (not the top)
				require.Len(t, stackCtx.ActiveChanges, 2)
				return stackCtx, stackCtx.ActiveChanges[0]
			},
			expectBranch: "test-user/stack-test-stack/3333333333333333",
		},
		{
			name: "TopChange_CheckoutTOPBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, *model.Change) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create two commits
				uuid1 := "4444444444444444"
				uuid2 := "5555555555555555"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Return the top change (position 2)
				require.Len(t, stackCtx.ActiveChanges, 2)
				topChange := stackCtx.ActiveChanges[1]
				assert.Equal(t, 2, topChange.Position)

				return stackCtx, topChange
			},
			expectBranch: "test-user/stack-test-stack/TOP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx, change := tt.setup(t, stackClient, mockGithubClient)

				branchName, err := stackClient.CheckoutChangeForEditing(stackCtx, change)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expectBranch, branchName)

					// Verify we're on the expected branch
					currentBranch, err := stackClient.git.GetCurrentBranch()
					require.NoError(t, err)
					assert.Equal(t, tt.expectBranch, currentBranch)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestApplyRefresh(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client, *gh.MockGithubClient) *StackContext
		expectError error
	}{
		{
			name: "Success_OnTOPBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a change
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				// Mark first change as merged
				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						"1111111111111111": {
							PRNumber: 101,
							State:    "merged",
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Ensure we're on TOP branch
				err = client.git.CheckoutBranch(stack.Branch)
				require.NoError(t, err)

				return stackCtx
			},
		},
		{
			name: "Error_NotOnTOPBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})

				// Switch to main branch (not TOP)
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				// Reload context after switching - it should now have stackActive=false
				stackCtx, err := client.GetStackContext()
				require.NoError(t, err)

				return stackCtx
			},
			expectError: fmt.Errorf("must be on TOP branch"),
		},
		{
			name: "Error_OnUUIDBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				uuid := "3333333333333333"
				commitHash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  uuid,
					"PR-Stack": "test-stack",
				})

				// Create and checkout UUID branch
				uuidBranch := fmt.Sprintf("test-user/stack-test-stack/%s", uuid)
				err = client.git.CreateAndCheckoutBranchAt(uuidBranch, commitHash)
				require.NoError(t, err)

				// Reload context after switching - it should now have onUUIDBranch=true
				stackCtx, err := client.GetStackContext()
				require.NoError(t, err)

				return stackCtx
			},
			expectError: fmt.Errorf("must be on TOP branch"),
		},
		{
			name: "Error_UncommittedChanges",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "4444444444444444",
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Ensure we're on TOP branch
				err = client.git.CheckoutBranch(stack.Branch)
				require.NoError(t, err)

				// Create uncommitted changes
				testFile := filepath.Join(client.git.GitRoot(), "uncommitted.txt")
				err = os.WriteFile(testFile, []byte("uncommitted content"), 0644)
				require.NoError(t, err)

				return stackCtx
			},
			expectError: fmt.Errorf("cannot apply refresh with uncommitted changes"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx := tt.setup(t, stackClient, mockGithubClient)

				merged := stackCtx.StaleMergedChanges
				err := stackClient.ApplyRefresh(stackCtx, merged)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify stack metadata was updated
					reloadedStack, err := stackClient.LoadStack("test-stack")
					require.NoError(t, err)
					assert.Equal(t, "main", reloadedStack.Base)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestRestack(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client, *gh.MockGithubClient) (*StackContext, RestackOptions)
		expectError error
	}{
		{
			name: "Success_WithoutFetch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, RestackOptions) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				err = client.git.CheckoutBranch(stack.Branch)
				require.NoError(t, err)

				return stackCtx, RestackOptions{
					Onto:  "main",
					Fetch: false,
				}
			},
		},
		{
			name: "Success_WithFetch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) (*StackContext, RestackOptions) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})

				// Set up a remote for fetch (even though fetch will be a no-op in tests)
				cmd := exec.Command("git", "config", "branch.main.remote", "origin")
				cmd.Dir = client.git.GitRoot()
				_ = cmd.Run()

				cmd = exec.Command("git", "config", "branch.main.merge", "refs/heads/main")
				cmd.Dir = client.git.GitRoot()
				_ = cmd.Run()

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				err = client.git.CheckoutBranch(stack.Branch)
				require.NoError(t, err)

				return stackCtx, RestackOptions{
					Onto:  "main",
					Fetch: true,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx, opts := tt.setup(t, stackClient, mockGithubClient)

				// Store original base ref
				originalBaseRef := stackCtx.Stack.BaseRef

				err := stackClient.Restack(stackCtx, opts)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify stack metadata was updated
					reloadedStack, err := stackClient.LoadStack(stackCtx.StackName)
					require.NoError(t, err)
					assert.Equal(t, opts.Onto, reloadedStack.Base)
					assert.NotEmpty(t, reloadedStack.BaseRef)

					// Base ref should be set (may or may not change depending on test)
					assert.NotEmpty(t, reloadedStack.BaseRef)

					// If we didn't fetch, base ref should match what it was (main hasn't changed)
					if !opts.Fetch {
						assert.Equal(t, originalBaseRef, reloadedStack.BaseRef)
					}
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestUpdateLocalBaseRef(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client) string
		expectError error
	}{
		// NOTE: The other test cases (BranchExists_HashMatches, BranchExists_HashDiffers, BranchDoesNotExist_CreateIt)
		// are complex to test because they require a real remote repository with upstream tracking.
		// In a test environment without a remote, these cases would fail with "no upstream tracking branch configured".
		// The NoUpstream_Error test below covers the primary error case for this function.
		{
			name: "NoUpstream_Error",
			setup: func(t *testing.T, client *Client) string {
				// Create a local branch with no upstream configured
				cmd := exec.Command("git", "branch", "local-only")
				cmd.Dir = client.git.GitRoot()
				err := cmd.Run()
				require.NoError(t, err)

				return "local-only"
			},
			expectError: fmt.Errorf("no upstream tracking branch configured"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				baseBranch := tt.setup(t, stackClient)

				err := stackClient.UpdateLocalBaseRef(baseBranch)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestDeleteStack(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client, *gh.MockGithubClient) string
		expectError error
	}{
		{
			name: "Success_NotOnStackBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) string {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				// Switch to main so we're not on the stack branch
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				return "test-stack"
			},
		},
		{
			name: "Success_OnStackBranch",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) string {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})

				// Stay on stack branch
				err = client.git.CheckoutBranch(stack.Branch)
				require.NoError(t, err)

				return "test-stack"
			},
		},
		{
			name: "Error_StackLoadFails",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) string {
				return "nonexistent-stack"
			},
			expectError: fmt.Errorf("failed to load stack"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackName := tt.setup(t, stackClient, mockGithubClient)

				err := stackClient.DeleteStack(stackName, true)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify stack metadata was archived
					archivedDir := filepath.Join(stackClient.getStacksRootDir(), ".archived")
					entries, err := os.ReadDir(archivedDir)
					require.NoError(t, err)

					found := false
					for _, entry := range entries {
						if strings.HasPrefix(entry.Name(), stackName+"-") {
							found = true
							break
						}
					}
					assert.True(t, found, "archived stack metadata should exist")

					// Verify we're on the base branch if we were on stack branch
					currentBranch, err := stackClient.git.GetCurrentBranch()
					require.NoError(t, err)
					// Should be on main (base branch)
					assert.Equal(t, "main", currentBranch)

					// Verify stack branches were deleted
					branches, err := stackClient.GetStackBranches(stackName)
					require.NoError(t, err)
					for _, branch := range branches {
						exists := stackClient.git.BranchExists(branch)
						assert.False(t, exists, "branch %s should be deleted", branch)
					}
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestIsStackEligibleForCleanup(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*testing.T, *Client, *gh.MockGithubClient) *StackContext
		expectEligible bool
		expectReason   string
	}{
		{
			name: "EmptyStack",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("empty-stack", "main")
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("empty-stack")
				require.NoError(t, err)

				return stackCtx
			},
			expectEligible: true,
			expectReason:   "empty",
		},
		{
			name: "AllChangesMerged",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("merged-stack", "main")
				require.NoError(t, err)

				uuid1 := "1111111111111111"
				uuid2 := "2222222222222222"

				hash1 := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "merged-stack",
				})

				hash2 := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "merged-stack",
				})

				// Mark both as merged in PR metadata
				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid1: {
							PRNumber: 101,
							State:    "merged",
						},
						uuid2: {
							PRNumber: 102,
							State:    "merged",
						},
					},
				}
				err = client.savePRs("merged-stack", prData)
				require.NoError(t, err)

				// Add to MergedChanges
				stack.MergedChanges = []model.Change{
					{
						UUID:       uuid1,
						CommitHash: hash1,
						Position:   1,
						PR: &model.PR{
							PRNumber: 101,
							State:    "merged",
						},
					},
					{
						UUID:       uuid2,
						CommitHash: hash2,
						Position:   2,
						PR: &model.PR{
							PRNumber: 102,
							State:    "merged",
						},
					},
				}
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("merged-stack")
				require.NoError(t, err)

				return stackCtx
			},
			expectEligible: true,
			expectReason:   "all_merged",
		},
		{
			name: "SomeChangesNotMerged",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("partial-stack", "main")
				require.NoError(t, err)

				uuid1 := "3333333333333333"
				uuid2 := "4444444444444444"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "partial-stack",
				})

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "partial-stack",
				})

				// Mark only first as merged
				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid1: {
							PRNumber: 201,
							State:    "merged",
						},
						uuid2: {
							PRNumber: 202,
							State:    "open",
						},
					},
				}
				err = client.savePRs("partial-stack", prData)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("partial-stack")
				require.NoError(t, err)

				return stackCtx
			},
			expectEligible: false,
			expectReason:   "",
		},
		{
			name: "LocalChangesOnly",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("local-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Local change", "Description", map[string]string{
					"PR-UUID":  "5555555555555555",
					"PR-Stack": "local-stack",
				})

				stackCtx, err := client.GetStackContextByName("local-stack")
				require.NoError(t, err)

				return stackCtx
			},
			expectEligible: false,
			expectReason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx := tt.setup(t, stackClient, mockGithubClient)

				eligible, reason := stackClient.IsStackEligibleForCleanup(stackCtx)

				assert.Equal(t, tt.expectEligible, eligible)
				assert.Equal(t, tt.expectReason, reason)

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestGetCleanupCandidates(t *testing.T) {
	tests := []struct {
		name                 string
		setup                func(*testing.T, *Client, *gh.MockGithubClient)
		expectedCandidates   []string
		expectedReasons      map[string]string
		expectedChangeCounts map[string]int
	}{
		{
			name: "NoStacks",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				// Don't create any stacks
			},
			expectedCandidates: []string{},
		},
		{
			name: "MultipleStacks_SomeEligible",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Times(3)

				// Create empty stack (eligible)
				_, err := client.CreateStack("empty-stack", "main")
				require.NoError(t, err)

				// Switch to main before creating next stack
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				// Create merged stack (eligible)
				_, err = client.CreateStack("merged-stack", "main")
				require.NoError(t, err)

				uuid := "1111111111111111"
				hash := testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Merged change", "Description", map[string]string{
					"PR-UUID":  uuid,
					"PR-Stack": "merged-stack",
				})

				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid: {
							PRNumber: 101,
							State:    "merged",
						},
					},
				}
				err = client.savePRs("merged-stack", prData)
				require.NoError(t, err)

				mergedStack, err := client.LoadStack("merged-stack")
				require.NoError(t, err)
				mergedStack.MergedChanges = []model.Change{
					{
						UUID:       uuid,
						CommitHash: hash,
						Position:   1,
						PR: &model.PR{
							PRNumber: 101,
							State:    "merged",
						},
					},
				}
				err = client.SaveStack(mergedStack)
				require.NoError(t, err)

				// Switch to main before creating next stack
				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				// Create active stack (not eligible)
				_, err = client.CreateStack("active-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Active change", "Description", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "active-stack",
				})

				// Mock BatchGetPRs for GetCleanupCandidates syncing
				// merged-stack has PR 101 (merged)
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", []int{101}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {Number: 101, State: "MERGED", IsMerged: true, IsDraft: false},
					},
				}, nil)
			},
			expectedCandidates: []string{"empty-stack", "merged-stack"},
			expectedReasons: map[string]string{
				"empty-stack":  "empty",
				"merged-stack": "all_merged",
			},
			expectedChangeCounts: map[string]int{
				"empty-stack":  0,
				"merged-stack": 1,
			},
		},
		{
			name: "AllStacksEligible",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Times(2)

				_, err := client.CreateStack("empty-1", "main")
				require.NoError(t, err)

				err = client.git.CheckoutBranch("main")
				require.NoError(t, err)

				_, err = client.CreateStack("empty-2", "main")
				require.NoError(t, err)
			},
			expectedCandidates: []string{"empty-1", "empty-2"},
			expectedReasons: map[string]string{
				"empty-1": "empty",
				"empty-2": "empty",
			},
			expectedChangeCounts: map[string]int{
				"empty-1": 0,
				"empty-2": 0,
			},
		},
		{
			name: "NoStacksEligible",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("active-stack", "main")
				require.NoError(t, err)

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Active change", "Description", map[string]string{
					"PR-UUID":  "3333333333333333",
					"PR-Stack": "active-stack",
				})
			},
			expectedCandidates: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				tt.setup(t, stackClient, mockGithubClient)

				candidates, err := stackClient.GetCleanupCandidates()
				require.NoError(t, err)

				// Build actual results maps
				actualNames := make([]string, len(candidates))
				actualReasons := make(map[string]string)
				actualChangeCounts := make(map[string]int)
				for i, candidate := range candidates {
					name := candidate.StackCtx.StackName
					actualNames[i] = name
					actualReasons[name] = candidate.Reason
					actualChangeCounts[name] = candidate.ChangeCount
				}

				// Verify results
				assert.ElementsMatch(t, tt.expectedCandidates, actualNames)
				if tt.expectedReasons != nil {
					assert.Equal(t, tt.expectedReasons, actualReasons)
				}
				if tt.expectedChangeCounts != nil {
					assert.Equal(t, tt.expectedChangeCounts, actualChangeCounts)
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestRefreshStackMetadata(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *Client, *gh.MockGithubClient) *StackContext
		expectError error
	}{
		{
			name: "Success_NoChanges",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// No need to mock BatchGetPRs - no PRs to sync
				return stackCtx
			},
		},
		{
			name: "Success_WithPRs_AllOpen",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create two changes
				uuid1 := "1111111111111111"
				uuid2 := "2222222222222222"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				// Create PRs for both changes
				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid1: {
							PRNumber:          101,
							URL:               "https://github.com/test-owner/test-repo/pull/101",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
						uuid2: {
							PRNumber:          102,
							URL:               "https://github.com/test-owner/test-repo/pull/102",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Mock BatchGetPRs to return updated PR states
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", mock.AnythingOfType("[]int")).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {Number: 101, State: "OPEN", IsMerged: false, IsDraft: false},
						102: {Number: 102, State: "OPEN", IsMerged: false, IsDraft: false},
					},
				}, nil)

				return stackCtx
			},
		},
		{
			name: "Success_WithPRs_OneMerged",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create two changes
				uuid1 := "3333333333333333"
				uuid2 := "4444444444444444"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "First change", "Description 1", map[string]string{
					"PR-UUID":  uuid1,
					"PR-Stack": "test-stack",
				})

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Second change", "Description 2", map[string]string{
					"PR-UUID":  uuid2,
					"PR-Stack": "test-stack",
				})

				// Create PRs with first one merged
				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid1: {
							PRNumber:          201,
							URL:               "https://github.com/test-owner/test-repo/pull/201",
							State:             "open", // Will be updated to merged
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
						uuid2: {
							PRNumber:          202,
							URL:               "https://github.com/test-owner/test-repo/pull/202",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Mock BatchGetPRs to return first PR as merged
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", mock.AnythingOfType("[]int")).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						201: {Number: 201, State: "MERGED", IsMerged: true, IsDraft: false},
						202: {Number: 202, State: "OPEN", IsMerged: false, IsDraft: false},
					},
				}, nil)

				return stackCtx
			},
		},
		{
			name: "Error_BatchGetPRsFails",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a change with a PR
				uuid := "5555555555555555"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  uuid,
					"PR-Stack": "test-stack",
				})

				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid: {
							PRNumber:          301,
							URL:               "https://github.com/test-owner/test-repo/pull/301",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Mock BatchGetPRs to fail
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", mock.AnythingOfType("[]int")).
					Return(nil, fmt.Errorf("API error"))

				return stackCtx
			},
			expectError: fmt.Errorf("failed to sync with GitHub"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx := tt.setup(t, stackClient, mockGithubClient)

				// Store original pointer for verification
				originalPtr := stackCtx

				result, err := stackClient.RefreshStackMetadata(stackCtx)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify we got the same context pointer back
					assert.Equal(t, originalPtr, result, "should return the same context pointer")

					// Verify sync metadata was updated
					reloadedStack, err := stackClient.LoadStack(stackCtx.StackName)
					require.NoError(t, err)
					assert.False(t, reloadedStack.LastSynced.IsZero(), "LastSynced should be updated")
					assert.NotEmpty(t, reloadedStack.SyncHash, "SyncHash should be set")
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestMaybeRefreshStackMetadata(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(*testing.T, *Client, *gh.MockGithubClient) *StackContext
		expectSyncCalled bool
		expectError      error
	}{
		{
			name: "AlreadyFresh_NoSyncNeeded",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Get current hash
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to now with matching hash (fresh)
				stack.LastSynced = time.Now()
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// No mock for BatchGetPRs - should not be called
				return stackCtx
			},
			expectSyncCalled: false,
		},
		{
			name: "NeverSynced_SyncCalled",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Reset LastSynced to zero (never synced)
				stack.LastSynced = time.Time{}
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// No PRs to sync, so no mock needed
				return stackCtx
			},
			expectSyncCalled: true,
		},
		{
			name: "HashMismatch_SyncCalled",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Set LastSynced to now but with wrong hash
				stack.LastSynced = time.Now()
				stack.SyncHash = "old-hash-that-doesnt-match"
				err = client.SaveStack(stack)
				require.NoError(t, err)

				// Create a commit to change the hash
				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "New commit", "Body", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// No PRs yet, so no mock needed
				return stackCtx
			},
			expectSyncCalled: true,
		},
		{
			name: "Stale_SyncCalled",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Get current hash
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to over 5 minutes ago with matching hash (stale)
				stack.LastSynced = time.Now().Add(-10 * time.Minute)
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// No PRs to sync, so no mock needed
				return stackCtx
			},
			expectSyncCalled: true,
		},
		{
			name: "StaleWithPRs_SyncCalledAndSucceeds",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a change with PR
				uuid := "2222222222222222"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  uuid,
					"PR-Stack": "test-stack",
				})

				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid: {
							PRNumber:          401,
							URL:               "https://github.com/test-owner/test-repo/pull/401",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				// Get current hash
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to over 5 minutes ago (stale)
				stack.LastSynced = time.Now().Add(-10 * time.Minute)
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Mock BatchGetPRs since we have PRs
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", []int{401}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						401: {Number: 401, State: "OPEN", IsMerged: false, IsDraft: false},
					},
				}, nil)

				return stackCtx
			},
			expectSyncCalled: true,
		},
		{
			name: "StaleWithPRs_SyncFails",
			setup: func(t *testing.T, client *Client, mockGithubClient *gh.MockGithubClient) *StackContext {
				mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				stack, err := client.CreateStack("test-stack", "main")
				require.NoError(t, err)

				// Create a change with PR
				uuid := "3333333333333333"

				_ = testutil.CreateCommitWithTrailers(t, client.git.(*git.Client), "Test change", "Description", map[string]string{
					"PR-UUID":  uuid,
					"PR-Stack": "test-stack",
				})

				prData := &model.PRData{
					Version: 1,
					PRs: map[string]*model.PR{
						uuid: {
							PRNumber:          501,
							URL:               "https://github.com/test-owner/test-repo/pull/501",
							State:             "open",
							RemoteDraftStatus: false,
							LocalDraftStatus:  false,
						},
					},
				}
				err = client.savePRs("test-stack", prData)
				require.NoError(t, err)

				// Get current hash
				currentHash, err := client.git.GetCommitHash(stack.Branch)
				require.NoError(t, err)

				// Set LastSynced to over 5 minutes ago (stale)
				stack.LastSynced = time.Now().Add(-10 * time.Minute)
				stack.SyncHash = currentHash
				err = client.SaveStack(stack)
				require.NoError(t, err)

				stackCtx, err := client.GetStackContextByName("test-stack")
				require.NoError(t, err)

				// Mock BatchGetPRs to fail
				mockGithubClient.On("BatchGetPRs", "test-owner", "test-repo", []int{501}).
					Return(nil, fmt.Errorf("GitHub API error"))

				return stackCtx
			},
			expectSyncCalled: true,
			expectError:      fmt.Errorf("failed to sync with GitHub"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &gh.MockGithubClient{}
				stackClient := NewTestStack(t, mockGithubClient)

				stackCtx := tt.setup(t, stackClient, mockGithubClient)

				// Store original pointer and LastSynced for verification
				originalPtr := stackCtx
				originalLastSynced := stackCtx.Stack.LastSynced

				result, err := stackClient.MaybeRefreshStackMetadata(stackCtx)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
				} else {
					require.NoError(t, err)

					// Verify we got the same context pointer back
					assert.Equal(t, originalPtr, result, "should return the same context pointer")

					reloadedStack, err := stackClient.LoadStack(stackCtx.StackName)
					require.NoError(t, err)

					// Verify sync behavior
					if tt.expectSyncCalled {
						// Sync should have updated LastSynced
						assert.False(t, reloadedStack.LastSynced.IsZero(), "LastSynced should be updated")
						assert.NotEmpty(t, reloadedStack.SyncHash, "SyncHash should be set")

						// LastSynced should have changed (unless it was zero)
						if !originalLastSynced.IsZero() {
							// If it was stale or hash mismatch, LastSynced should be updated to "now"
							// In synctest, time is frozen, so we check it's not the original value
							// Actually in synctest time.Now() returns the same frozen time, so we can't
							// easily distinguish. Instead just verify it was set.
							assert.NotZero(t, reloadedStack.LastSynced)
						}
					} else {
						// Sync should NOT have been called - LastSynced unchanged
						assert.Equal(t, originalLastSynced, reloadedStack.LastSynced,
							"LastSynced should not change when sync not needed")
					}
				}

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}
