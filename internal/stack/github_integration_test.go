package stack

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
)

func TestIsChangeMerged(t *testing.T) {
	tests := []struct {
		name     string
		change   *model.Change
		expected bool
	}{
		{
			name: "local change returns false",
			change: &model.Change{
				UUID: "1111111111111111",
				PR:   nil,
			},
			expected: false,
		},
		{
			name: "change with nil PR returns false",
			change: &model.Change{
				UUID: "1111111111111111",
				PR:   nil,
			},
			expected: false,
		},
		{
			name: "change with open PR returns false",
			change: &model.Change{
				UUID: "1111111111111111",
				PR: &model.PR{
					PRNumber: 101,
					State:    "open",
				},
			},
			expected: false,
		},
		{
			name: "change with draft PR returns false",
			change: &model.Change{
				UUID: "1111111111111111",
				PR: &model.PR{
					PRNumber: 101,
					State:    "draft",
				},
			},
			expected: false,
		},
		{
			name: "change with closed PR returns false",
			change: &model.Change{
				UUID: "1111111111111111",
				PR: &model.PR{
					PRNumber: 101,
					State:    "closed",
				},
			},
			expected: false,
		},
		{
			name: "change with merged PR returns true",
			change: &model.Change{
				UUID: "1111111111111111",
				PR: &model.PR{
					PRNumber: 101,
					State:    "merged",
				},
			},
			expected: true,
		},
		{
			name: "change with MERGED uppercase returns true",
			change: &model.Change{
				UUID: "1111111111111111",
				PR: &model.PR{
					PRNumber: 101,
					State:    "MERGED",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGithubClient := &MockGithubClient{}
			stackClient := newTestStackClient(t, mockGithubClient)

			result := stackClient.IsChangeMerged(tt.change)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func createTestStackContextWithChange(t *testing.T, stackClient *Client, change *model.Change) *StackContext {
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	return &StackContext{
		StackName:  "test-stack",
		Stack:      stack,
		changes:    map[string]*model.Change{change.UUID: change},
		AllChanges: []*model.Change{change},
		username:   "test-user",
		client:     stackClient,
	}
}

func TestMarkChangeStatus(t *testing.T) {
	tests := []struct {
		name           string
		change         *model.Change
		isDraft        bool
		setupMocks     func(*MockGithubClient)
		expectedResult *MarkChangeStatusResult
		expectedChange *model.Change
		expectError    bool
		errorContains  string
	}{
		{
			name: "local change marked draft - no GitHub sync",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Local change",
				PR:    nil,
			},
			isDraft: true,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: false,
				PRNumber:       0,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Local change",
				PR: &model.PR{
					LocalDraftStatus:  true,
					RemoteDraftStatus: false,
				},
			},
		},
		{
			name: "local change marked ready - no GitHub sync",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Local change",
				PR:    nil,
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: false,
				PRNumber:       0,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Local change",
				PR: &model.PR{
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
		},
		{
			name: "remote open PR marked draft - syncs to GitHub",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          101,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			isDraft: true,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("MarkPRDraft", 101).Return(nil).Once()
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 101, mock.AnythingOfType("string")).Return("comment-123", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: true,
				PRNumber:       101,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          101,
					State:             "draft",
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
					VizCommentID:      "comment-123",
				},
			},
		},
		{
			name: "remote open PR marked ready - syncs to GitHub",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          101,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("MarkPRReady", 101).Return(nil).Once()
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 101, mock.AnythingOfType("string")).Return("comment-123", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: true,
				PRNumber:       101,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          101,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
					VizCommentID:      "comment-123",
				},
			},
		},
		{
			name: "remote draft PR marked ready - syncs to GitHub",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          102,
					State:             "draft",
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
				},
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("MarkPRReady", 102).Return(nil).Once()
				m.On("ListPRComments", 102).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 102, mock.AnythingOfType("string")).Return("comment-456", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: true,
				PRNumber:       102,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          102,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
					VizCommentID:      "comment-456",
				},
			},
		},
		{
			name: "remote merged PR - does not sync to GitHub",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Merged change",
				PR: &model.PR{
					PRNumber:          103,
					State:             "merged",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			isDraft: true,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("ListPRComments", 103).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 103, mock.AnythingOfType("string")).Return("comment-123", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: false,
				PRNumber:       0,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Merged change",
				PR: &model.PR{
					PRNumber:          103,
					State:             "merged",
					LocalDraftStatus:  true,
					RemoteDraftStatus: false,
					VizCommentID:      "comment-123",
				},
			},
		},
		{
			name: "remote closed PR - does not sync to GitHub",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Closed change",
				PR: &model.PR{
					PRNumber:          104,
					State:             "closed",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("ListPRComments", 104).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 104, mock.AnythingOfType("string")).Return("comment-456", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: false,
				PRNumber:       0,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Closed change",
				PR: &model.PR{
					PRNumber:          104,
					State:             "closed",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
					VizCommentID:      "comment-456",
				},
			},
		},
		{
			name: "LocalDraftStatus != RemoteDraftStatus - updates local only, no GitHub call",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Out of sync change",
				PR: &model.PR{
					PRNumber:          105,
					State:             "open",
					LocalDraftStatus:  true,
					RemoteDraftStatus: false,
				},
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("ListPRComments", 105).Return([]gh.Comment{}, nil).Once()
				m.On("CreatePRComment", 105, mock.AnythingOfType("string")).Return("comment-789", nil).Once()
			},
			expectedResult: &MarkChangeStatusResult{
				SyncedToGitHub: true,
				PRNumber:       105,
			},
			expectedChange: &model.Change{
				UUID:  "1111111111111111",
				Title: "Out of sync change",
				PR: &model.PR{
					PRNumber:          105,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
					VizCommentID:      "comment-789",
				},
			},
		},
		{
			name: "GitHub API failure when marking draft",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          106,
					State:             "open",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			isDraft: true,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("MarkPRDraft", 106).Return(fmt.Errorf("network error")).Once()
			},
			expectError:   true,
			errorContains: "failed to mark PR #106 as draft on GitHub",
		},
		{
			name: "GitHub API failure when marking ready",
			change: &model.Change{
				UUID:  "1111111111111111",
				Title: "Remote change",
				PR: &model.PR{
					PRNumber:          107,
					State:             "draft",
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
				},
			},
			isDraft: false,
			setupMocks: func(m *MockGithubClient) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("MarkPRReady", 107).Return(fmt.Errorf("permission denied")).Once()
			},
			expectError:   true,
			errorContains: "failed to mark PR #107 as ready on GitHub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &MockGithubClient{}
				if tt.setupMocks != nil {
					tt.setupMocks(mockGithubClient)
				}

				stackClient := newTestStackClient(t, mockGithubClient)
				stackCtx := createTestStackContextWithChange(t, stackClient, tt.change)

				result, err := stackClient.markChangeStatus(stackCtx, tt.change, tt.isDraft)

				if tt.expectError {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.errorContains)
					mockGithubClient.AssertExpectations(t)
					return
				}

				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
				assert.Equal(t, tt.expectedChange, tt.change)

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}

func TestMarkChangeDraft(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}
		mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
		mockGithubClient.On("MarkPRDraft", 101).Return(nil).Once()
		mockGithubClient.On("ListPRComments", 101).Return([]gh.Comment{}, nil).Once()
		mockGithubClient.On("CreatePRComment", 101, mock.AnythingOfType("string")).Return("comment-123", nil).Once()

		stackClient := newTestStackClient(t, mockGithubClient)
		stack, err := stackClient.CreateStack("test-stack", "main")
		require.NoError(t, err)

		change := &model.Change{
			UUID:  "1111111111111111",
			Title: "Test change",
			PR: &model.PR{
				PRNumber:          101,
				State:             "open",
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		}

		stackCtx := &StackContext{
			StackName:  "test-stack",
			Stack:      stack,
			changes:    map[string]*model.Change{change.UUID: change},
			AllChanges: []*model.Change{change},
			username:   "test-user",
			client:     stackClient,
		}

		result, err := stackClient.MarkChangeDraft(stackCtx, change)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.SyncedToGitHub)
		assert.Equal(t, 101, result.PRNumber)
		assert.True(t, change.PR.LocalDraftStatus)
		assert.True(t, change.PR.RemoteDraftStatus)
		assert.Equal(t, "draft", change.PR.State)

		mockGithubClient.AssertExpectations(t)
	})
}

func TestMarkChangeReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}
		mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
		mockGithubClient.On("MarkPRReady", 102).Return(nil).Once()
		mockGithubClient.On("ListPRComments", 102).Return([]gh.Comment{}, nil).Once()
		mockGithubClient.On("CreatePRComment", 102, mock.AnythingOfType("string")).Return("comment-456", nil).Once()

		stackClient := newTestStackClient(t, mockGithubClient)
		stack, err := stackClient.CreateStack("test-stack", "main")
		require.NoError(t, err)

		change := &model.Change{
			UUID:  "2222222222222222",
			Title: "Test change",
			PR: &model.PR{
				PRNumber:          102,
				State:             "draft",
				LocalDraftStatus:  true,
				RemoteDraftStatus: true,
			},
		}

		stackCtx := &StackContext{
			StackName:  "test-stack",
			Stack:      stack,
			changes:    map[string]*model.Change{change.UUID: change},
			AllChanges: []*model.Change{change},
			username:   "test-user",
			client:     stackClient,
		}

		result, err := stackClient.MarkChangeReady(stackCtx, change)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.SyncedToGitHub)
		assert.Equal(t, 102, result.PRNumber)
		assert.False(t, change.PR.LocalDraftStatus)
		assert.False(t, change.PR.RemoteDraftStatus)
		assert.Equal(t, "open", change.PR.State)

		mockGithubClient.AssertExpectations(t)
	})
}

func TestSyncPRMetadata(t *testing.T) {
	tests := []struct {
		name            string
		changes         []*model.Change
		setupMocks      func(*MockGithubClient, []*model.Change)
		expectError     error
		expectedResult  *RefreshResult
		expectedChanges []*model.Change
	}{
		{
			name: "empty stack - no changes",
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     0,
				StaleMergedChanges: nil,
			},
		},

		{
			name: "all local changes - no PRs",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Local change 1",
					PR:    nil,
				},
				{
					UUID:  "2222222222222222",
					Title: "Local change 2",
					PR:    nil,
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Local change 1",
				},
				{
					UUID:  "2222222222222222",
					Title: "Local change 2",
				},
			},
		},
		{
			name: "mixed local and remote changes",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Local change",
					PR:    nil,
				},
				{
					UUID:  "2222222222222222",
					Title: "Remote change",
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "OPEN",
							IsMerged: false,
							IsDraft:  false,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Local change",
					PR:    nil,
				},
				{
					UUID:  "2222222222222222",
					Title: "Remote change",
					PR: &model.PR{
						PRNumber:          101,
						State:             "open",
						RemoteDraftStatus: false,
					},
				},
			},
		},
		{
			name: "all PRs open",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "First PR",
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:  "2222222222222222",
					Title: "Second PR",
					PR: &model.PR{
						PRNumber: 102,
						State:    "draft",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101, 102}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "OPEN",
							IsMerged: false,
							IsDraft:  false,
						},
						102: {
							Number:   102,
							State:    "OPEN",
							IsMerged: false,
							IsDraft:  true,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "First PR",
					PR: &model.PR{
						PRNumber:          101,
						State:             "open",
						RemoteDraftStatus: false,
					},
				},
				{
					UUID:  "2222222222222222",
					Title: "Second PR",
					PR: &model.PR{
						PRNumber:          102,
						State:             "open",
						RemoteDraftStatus: true,
					},
				},
			},
		},
		{
			name: "some PRs merged - bottom PR merged first",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First PR - merged",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second PR - still open",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						State:    "open",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101, 102}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "MERGED",
							IsMerged: true,
							MergedAt: time.Now(),
						},
						102: {
							Number:   102,
							State:    "OPEN",
							IsMerged: false,
							IsDraft:  false,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First PR - merged",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						State:    "merged",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second PR - still open",
					Position: 2,
					PR: &model.PR{
						PRNumber:          102,
						State:             "open",
						RemoteDraftStatus: false,
					},
				},
			},
		},
		{
			name: "bottom-up merge validation failure - top merged before bottom",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First PR - not merged",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second PR - merged (invalid!)",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						State:    "open",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101, 102}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "OPEN",
							IsMerged: false,
						},
						102: {
							Number:   102,
							State:    "MERGED",
							IsMerged: true,
							MergedAt: time.Now(),
						},
					},
				}, nil).Once()
			},
			expectError: fmt.Errorf("out-of-order merge detected"),
		},
		{
			name: "PR state transitions - open to merged, draft to closed",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "PR 1 - will be merged",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "PR 2 - will be closed",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						State:    "draft",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101, 102}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "MERGED",
							IsMerged: true,
							MergedAt: time.Now(),
						},
						102: {
							Number:   102,
							State:    "CLOSED",
							IsMerged: false,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "PR 1 - will be merged",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						State:    "merged",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "PR 2 - will be closed",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						State:    "closed",
					},
				},
			},
		},
		{
			name: "draft status updates from GitHub",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "PR marked as draft on GitHub",
					PR: &model.PR{
						PRNumber:          101,
						State:             "open",
						RemoteDraftStatus: false,
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "OPEN",
							IsMerged: false,
							IsDraft:  true,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     1,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "PR marked as draft on GitHub",
					PR: &model.PR{
						PRNumber:          101,
						State:             "open",
						RemoteDraftStatus: true,
					},
				},
			},
		},
		{
			name: "GitHub BatchGetPRs failure",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Remote PR",
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()
				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101}).Return(nil, fmt.Errorf("network error")).Once()
			},
			expectError: fmt.Errorf("failed to batch query PRs"),
		},
		{
			name: "PR not found in GitHub response - skipped",
			changes: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Existing PR",
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:  "2222222222222222",
					Title: "Deleted PR",
					PR: &model.PR{
						PRNumber: 102,
						State:    "open",
					},
				},
			},
			setupMocks: func(m *MockGithubClient, changes []*model.Change) {
				m.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()

				m.On("BatchGetPRs", "test-owner", "test-repo", []int{101, 102}).Return(&gh.BatchPRsResult{
					PRStates: map[int]*gh.PRState{
						101: {
							Number:   101,
							State:    "OPEN",
							IsMerged: false,
						},
					},
				}, nil).Once()
			},
			expectedResult: &RefreshResult{
				StaleMergedCount:   0,
				RemainingCount:     2,
				StaleMergedChanges: nil,
			},
			expectedChanges: []*model.Change{
				{
					UUID:  "1111111111111111",
					Title: "Existing PR",
					PR: &model.PR{
						PRNumber: 101,
						State:    "open",
					},
				},
				{
					UUID:  "2222222222222222",
					Title: "Deleted PR",
					PR: &model.PR{
						PRNumber: 102,
						State:    "open",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockGithubClient := &MockGithubClient{}
				if tt.setupMocks != nil {
					tt.setupMocks(mockGithubClient, tt.changes)
				}

				stackClient := newTestStackClient(t, mockGithubClient)

				stack, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				changeMap := make(map[string]*model.Change)
				activeChanges := []*model.Change{}
				for _, change := range tt.changes {
					changeMap[change.UUID] = change
					// Only include unmerged changes in ActiveChanges
					if change.IsLocal() || change.PR == nil || !change.PR.IsMerged() {
						activeChanges = append(activeChanges, change)
					}
				}

				stackCtx := &StackContext{
					StackName:     "test-stack",
					Stack:         stack,
					changes:       changeMap,
					AllChanges:    tt.changes,
					ActiveChanges: activeChanges,
					username:      "test-user",
					client:        stackClient,
				}

				result, err := stackClient.SyncPRMetadata(stackCtx)

				if tt.expectError != nil {
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.expectError.Error())
					mockGithubClient.AssertExpectations(t)
					return
				}

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedResult, result)

				assert.False(t, stackCtx.Stack.LastSynced.IsZero())

				assert.Equal(t, tt.expectedChanges, stackCtx.AllChanges)

				mockGithubClient.AssertExpectations(t)
			})
		})
	}
}
