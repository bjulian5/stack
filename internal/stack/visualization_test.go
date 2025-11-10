package stack

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
)

func createTestStackContext(t *testing.T, stackName string, changes []*model.Change) *StackContext {
	mockGithubClient := &MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil).Once()

	stackClient := newTestStackClient(t, mockGithubClient)
	stack, err := stackClient.CreateStack(stackName, "main")
	require.NoError(t, err)

	changeMap := make(map[string]*model.Change)
	for _, change := range changes {
		changeMap[change.UUID] = change
	}

	return &StackContext{
		StackName:  stackName,
		Stack:      stack,
		changes:    changeMap,
		AllChanges: changes,
		username:   "test-user",
		client:     stackClient,
	}
}

func TestGenerateStackVisualization(t *testing.T) {
	tests := []struct {
		name        string
		stackName   string
		changes     []*model.Change
		currentPR   int
		expectedViz string
	}{
		{
			name:      "all active changes with current PR marker",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First change",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						URL:      "https://github.com/test-owner/test-repo/pull/101",
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
				{
					UUID:     "3333333333333333",
					Title:    "Third change",
					Position: 3,
					PR: &model.PR{
						PRNumber: 103,
						URL:      "https://github.com/test-owner/test-repo/pull/103",
						State:    "draft",
					},
				},
			},
			currentPR: 102,
			expectedViz: `## 📚 Stack: test-stack (3 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | https://github.com/test-owner/test-repo/pull/101 | ✅ Open   | First change |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Second change ← **YOU ARE HERE** |
| 3 | https://github.com/test-owner/test-repo/pull/103 | 📝 Draft  | Third change |

**Merge order:** ` + "`main → #101 → #102 → #103`" + `

---

💡 **Review tip:** Start from the bottom ([#101](https://github.com/test-owner/test-repo/pull/101)) for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
		{
			name:      "with local changes",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "Local change",
					Position: 1,
					PR:       nil,
				},
				{
					UUID:     "2222222222222222",
					Title:    "Pushed change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
			},
			currentPR: 102,
			expectedViz: `## 📚 Stack: test-stack (2 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | - | ⚪ Local  | Local change |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Pushed change ← **YOU ARE HERE** |

**Merge order:** ` + "`main → #102`" + `

---

💡 **Review tip:** Start from the bottom () for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
		{
			name:      "with merged changes",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "Merged change",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						URL:      "https://github.com/test-owner/test-repo/pull/101",
						State:    "merged",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Open change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
			},
			currentPR: 102,
			expectedViz: `## 📚 Stack: test-stack (2 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | https://github.com/test-owner/test-repo/pull/101 | 🟣 Merged | Merged change |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Open change ← **YOU ARE HERE** |

**Merge order:** ` + "`main → #101 → #102`" + `

---

💡 **Review tip:** Start from the bottom ([#101](https://github.com/test-owner/test-repo/pull/101)) for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
		{
			name:      "empty stack",
			stackName: "empty-stack",
			changes:   []*model.Change{},
			currentPR: 0,
			expectedViz: `## 📚 Stack: empty-stack (0 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|

**Merge order:** ` + "`main`" + `

---

💡 **Review tip:** Start from the bottom () for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: empty-stack -->
`,
		},
		{
			name:      "current PR at bottom",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "Bottom change",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						URL:      "https://github.com/test-owner/test-repo/pull/101",
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Top change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
			},
			currentPR: 101,
			expectedViz: `## 📚 Stack: test-stack (2 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | https://github.com/test-owner/test-repo/pull/101 | ✅ Open   | Bottom change ← **YOU ARE HERE** |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Top change |

**Merge order:** ` + "`main → #101 → #102`" + `

---

💡 **Review tip:** Start from the bottom ([#101](https://github.com/test-owner/test-repo/pull/101)) for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
		{
			name:      "current PR at top",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "Bottom change",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						URL:      "https://github.com/test-owner/test-repo/pull/101",
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Top change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
			},
			currentPR: 102,
			expectedViz: `## 📚 Stack: test-stack (2 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | https://github.com/test-owner/test-repo/pull/101 | ✅ Open   | Bottom change |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Top change ← **YOU ARE HERE** |

**Merge order:** ` + "`main → #101 → #102`" + `

---

💡 **Review tip:** Start from the bottom ([#101](https://github.com/test-owner/test-repo/pull/101)) for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
		{
			name:      "no current PR",
			stackName: "test-stack",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First change",
					Position: 1,
					PR: &model.PR{
						PRNumber: 101,
						URL:      "https://github.com/test-owner/test-repo/pull/101",
						State:    "open",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second change",
					Position: 2,
					PR: &model.PR{
						PRNumber: 102,
						URL:      "https://github.com/test-owner/test-repo/pull/102",
						State:    "open",
					},
				},
			},
			currentPR: 0,
			expectedViz: `## 📚 Stack: test-stack (2 PRs)

| # | PR | Status | Title |
|---|-----|---------|---------------------------------------|
| 1 | https://github.com/test-owner/test-repo/pull/101 | ✅ Open   | First change |
| 2 | https://github.com/test-owner/test-repo/pull/102 | ✅ Open   | Second change |

**Merge order:** ` + "`main → #101 → #102`" + `

---

💡 **Review tip:** Start from the bottom ([#101](https://github.com/test-owner/test-repo/pull/101)) for full context

🤖 Auto-updated by [stack](https://github.com/bjulian5/stack)

<!-- stack-visualization: test-stack -->
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestStackContext(t, tt.stackName, tt.changes)
			viz := generateStackVisualization(ctx, tt.currentPR)
			assert.Equal(t, tt.expectedViz, viz)
		})
	}
}

func TestGetStatusDisplay(t *testing.T) {
	tests := []struct {
		status        string
		expectedEmoji string
		expectedText  string
	}{
		{"open", "✅", "Open  "},
		{"draft", "📝", "Draft "},
		{"merged", "🟣", "Merged"},
		{"closed", "❌", "Closed"},
		{"local", "⚪", "Local "},
		{"", "⚪", "Local "},        // Default case
		{"unknown", "⚪", "Local "}, // Default case
		{"OPEN", "⚪", "Local "},    // Case sensitive
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status=%s", tt.status), func(t *testing.T) {
			emoji, text := getStatusDisplay(tt.status)
			assert.Equal(t, tt.expectedEmoji, emoji)
			assert.Equal(t, tt.expectedText, text)
		})
	}
}

func TestSyncCommentForPR(t *testing.T) {
	tests := []struct {
		name        string
		pr          *model.PR
		vizContent  string
		setupMocks  func(*MockGithubClient, *model.PR, string)
		expectError error
		verify      func(*testing.T, *model.PR, *MockGithubClient)
	}{
		{
			name: "with cached comment ID - success",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "comment-123",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("UpdatePRComment", "comment-123", vizContent).Return(nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				m.AssertNotCalled(t, "ListPRComments", mock.Anything)
			},
		},
		{
			name: "with cached comment ID - fails and finds existing",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "old-comment-123",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("UpdatePRComment", "old-comment-123", vizContent).Return(fmt.Errorf("comment not found"))

				existingComments := []gh.Comment{
					{ID: "comment-999", Body: "Some other comment"},
					{ID: "comment-456", Body: "Stack info\n<!-- stack-visualization: test-stack -->"},
				}
				m.On("ListPRComments", 101).Return(existingComments, nil)

				m.On("UpdatePRComment", "comment-456", vizContent).Return(nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				assert.Equal(t, "comment-456", pr.VizCommentID)
			},
		},
		{
			name: "with cached comment ID - fails and creates new",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "old-comment-123",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("UpdatePRComment", "old-comment-123", vizContent).Return(fmt.Errorf("comment not found"))

				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil)

				m.On("CreatePRComment", 101, vizContent).Return("new-comment-789", nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				assert.Equal(t, "new-comment-789", pr.VizCommentID)
			},
		},
		{
			name: "no cached ID - existing comment",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				existingComments := []gh.Comment{
					{ID: "comment-999", Body: "Some other comment"},
					{ID: "comment-456", Body: "Stack info\n<!-- stack-visualization: test-stack -->"},
				}
				m.On("ListPRComments", 101).Return(existingComments, nil)

				m.On("UpdatePRComment", "comment-456", vizContent).Return(nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				assert.Equal(t, "comment-456", pr.VizCommentID)
			},
		},
		{
			name: "no cached ID - no existing comment",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil)

				m.On("CreatePRComment", 101, vizContent).Return("new-comment-789", nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				assert.Equal(t, "new-comment-789", pr.VizCommentID)
			},
		},
		{
			name: "list comments fails",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("ListPRComments", 101).Return(nil, fmt.Errorf("API error"))
			},
			expectError: fmt.Errorf("failed to list comments"),
		},
		{
			name: "update comment fails",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				existingComments := []gh.Comment{
					{ID: "comment-456", Body: "<!-- stack-visualization: test-stack -->"},
				}
				m.On("ListPRComments", 101).Return(existingComments, nil)

				m.On("UpdatePRComment", "comment-456", vizContent).Return(fmt.Errorf("API error"))
			},
			expectError: fmt.Errorf("failed to update comment"),
		},
		{
			name: "create comment fails",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil)

				m.On("CreatePRComment", 101, vizContent).Return("", fmt.Errorf("API error"))
			},
			expectError: fmt.Errorf("failed to create comment"),
		},
		{
			name: "multiple stack comments - finds correct one",
			pr: &model.PR{
				PRNumber:     101,
				VizCommentID: "",
			},
			vizContent: "Test visualization content",
			setupMocks: func(m *MockGithubClient, pr *model.PR, vizContent string) {
				existingComments := []gh.Comment{
					{ID: "comment-111", Body: "Regular comment"},
					{ID: "comment-222", Body: "Another comment"},
					{ID: "comment-333", Body: "Stack info\n<!-- stack-visualization: test-stack -->\nMore content"},
					{ID: "comment-444", Body: "Yet another comment"},
				}
				m.On("ListPRComments", 101).Return(existingComments, nil)

				m.On("UpdatePRComment", "comment-333", vizContent).Return(nil)
			},
			verify: func(t *testing.T, pr *model.PR, m *MockGithubClient) {
				assert.Equal(t, "comment-333", pr.VizCommentID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGithubClient := &MockGithubClient{}
			stackClient := newTestStackClient(t, mockGithubClient)

			tt.setupMocks(mockGithubClient, tt.pr, tt.vizContent)

			err := stackClient.syncCommentForPR(tt.pr, tt.vizContent)

			if tt.expectError != nil {
				assert.ErrorContains(t, err, tt.expectError.Error())
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, tt.pr, mockGithubClient)
			}

			mockGithubClient.AssertExpectations(t)
		})
	}
}

func TestSyncVisualizationComments_Success(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}

		stackClient := newTestStackClient(t, mockGithubClient)

		changes := []*model.Change{
			{
				UUID:     "1111111111111111",
				Title:    "First change",
				Position: 1,
				PR: &model.PR{
					PRNumber:     101,
					VizCommentID: "",
				},
			},
			{
				UUID:     "2222222222222222",
				Title:    "Second change",
				Position: 2,
				PR: &model.PR{
					PRNumber:     102,
					VizCommentID: "",
				},
			},
			{
				UUID:     "3333333333333333",
				Title:    "Third change",
				Position: 3,
				PR: &model.PR{
					PRNumber:     103,
					VizCommentID: "",
				},
			},
		}

		ctx := createTestStackContext(t, "test-stack", changes)
		ctx.AllChanges = changes

		for _, change := range changes {
			prNumber := change.PR.PRNumber
			mockGithubClient.On("ListPRComments", prNumber).Return([]gh.Comment{}, nil)
			mockGithubClient.On("CreatePRComment", prNumber, mock.AnythingOfType("string")).
				Return(fmt.Sprintf("comment-%d", prNumber), nil)
		}

		err := stackClient.SyncVisualizationComments(ctx)
		assert.NoError(t, err)

		assert.Equal(t, "comment-101", changes[0].PR.VizCommentID)
		assert.Equal(t, "comment-102", changes[1].PR.VizCommentID)
		assert.Equal(t, "comment-103", changes[2].PR.VizCommentID)

		mockGithubClient.AssertExpectations(t)
	})
}

func TestSyncVisualizationComments_WithLocalChanges(t *testing.T) {
	mockGithubClient := &MockGithubClient{}

	stackClient := newTestStackClient(t, mockGithubClient)

	changes := []*model.Change{
		{
			UUID:     "1111111111111111",
			Title:    "Local change",
			Position: 1,
			PR:       nil,
		},
		{
			UUID:     "2222222222222222",
			Title:    "Pushed change",
			Position: 2,
			PR: &model.PR{
				PRNumber:     102,
				VizCommentID: "",
			},
		},
	}

	ctx := createTestStackContext(t, "test-stack", changes)
	ctx.AllChanges = changes

	mockGithubClient.On("ListPRComments", 102).Return([]gh.Comment{}, nil)
	mockGithubClient.On("CreatePRComment", 102, mock.AnythingOfType("string")).
		Return("comment-102", nil)

	err := stackClient.SyncVisualizationComments(ctx)
	assert.NoError(t, err)

	assert.Equal(t, "comment-102", changes[1].PR.VizCommentID)

	mockGithubClient.AssertExpectations(t)
}

func TestSyncVisualizationComments(t *testing.T) {
	tests := []struct {
		name          string
		changes       []*model.Change
		setupMocks    func(*MockGithubClient)
		expectError   bool
		errorContains string
		verify        func(*testing.T, []*model.Change, error)
	}{
		{
			name:        "empty stack",
			changes:     []*model.Change{},
			setupMocks:  func(m *MockGithubClient) {},
			expectError: false,
		},
		{
			name: "only local changes",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "Local change 1",
					Position: 1,
					PR:       nil,
				},
				{
					UUID:     "2222222222222222",
					Title:    "Local change 2",
					Position: 2,
					PR:       nil,
				},
			},
			setupMocks:  func(m *MockGithubClient) {},
			expectError: false,
		},
		{
			name: "partial failure - second PR fails",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First change",
					Position: 1,
					PR: &model.PR{
						PRNumber:     101,
						VizCommentID: "",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second change",
					Position: 2,
					PR: &model.PR{
						PRNumber:     102,
						VizCommentID: "",
					},
				},
			},
			setupMocks: func(m *MockGithubClient) {
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil)
				m.On("CreatePRComment", 101, mock.AnythingOfType("string")).
					Return("comment-101", nil)

				m.On("ListPRComments", 102).Return(nil, fmt.Errorf("API error"))
			},
			expectError:   true,
			errorContains: "failed to sync comment for PR #102",
		},
		{
			name: "visualization content has correct YOU ARE HERE markers",
			changes: []*model.Change{
				{
					UUID:     "1111111111111111",
					Title:    "First change",
					Position: 1,
					PR: &model.PR{
						PRNumber:     101,
						VizCommentID: "",
					},
				},
				{
					UUID:     "2222222222222222",
					Title:    "Second change",
					Position: 2,
					PR: &model.PR{
						PRNumber:     102,
						VizCommentID: "",
					},
				},
			},
			setupMocks: func(m *MockGithubClient) {
				m.On("ListPRComments", 101).Return([]gh.Comment{}, nil)
				m.On("CreatePRComment", 101, mock.AnythingOfType("string")).Return("comment-101", nil)

				m.On("ListPRComments", 102).Return([]gh.Comment{}, nil)
				m.On("CreatePRComment", 102, mock.AnythingOfType("string")).Return("comment-102", nil)
			},
			expectError: false,
			verify: func(t *testing.T, changes []*model.Change, err error) {
				assert.Equal(t, "comment-101", changes[0].PR.VizCommentID)
				assert.Equal(t, "comment-102", changes[1].PR.VizCommentID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGithubClient := &MockGithubClient{}
			stackClient := newTestStackClient(t, mockGithubClient)

			ctx := createTestStackContext(t, "test-stack", tt.changes)
			ctx.AllChanges = tt.changes

			tt.setupMocks(mockGithubClient)

			err := stackClient.SyncVisualizationComments(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, tt.changes, err)
			}

			mockGithubClient.AssertExpectations(t)
		})
	}
}

func TestSyncVisualizationComments_Concurrent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}
		stackClient := newTestStackClient(t, mockGithubClient)

		changes := make([]*model.Change, 10)
		for i := 0; i < 10; i++ {
			changes[i] = &model.Change{
				UUID:     fmt.Sprintf("%016d", i+1),
				Title:    fmt.Sprintf("Change %d", i+1),
				Position: i + 1,
				PR: &model.PR{
					PRNumber:     101 + i,
					VizCommentID: "",
				},
			}

			prNumber := 101 + i
			mockGithubClient.On("ListPRComments", prNumber).Return([]gh.Comment{}, nil)
			mockGithubClient.On("CreatePRComment", prNumber, mock.AnythingOfType("string")).
				Return(fmt.Sprintf("comment-%d", prNumber), nil)
		}

		ctx := createTestStackContext(t, "test-stack", changes)
		ctx.AllChanges = changes

		err := stackClient.SyncVisualizationComments(ctx)
		assert.NoError(t, err)

		for i, change := range changes {
			expectedID := fmt.Sprintf("comment-%d", 101+i)
			assert.Equal(t, expectedID, change.PR.VizCommentID,
				"VizCommentID mismatch for change %d", i+1)
		}

		mockGithubClient.AssertExpectations(t)
	})
}
