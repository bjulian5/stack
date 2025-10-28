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
)

func newTestStackClient(t *testing.T, gh GithubClient) *Client {
	mockGitClient := newTestClient(t)
	c := NewClient(mockGitClient, gh)
	c.username = "test-user"
	return c
}

func TestClient(t *testing.T) {
	t.Run("Install", func(t *testing.T) {
		mockGithubClient := &MockGithubClient{}
		mockGitClient := newTestClient(t)
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
			mockGithubClient := &MockGithubClient{}
			mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

			stackClient := newTestStackClient(t, mockGithubClient)

			v, err := stackClient.CreateStack("test-stack", "main")
			assert.NoError(t, err, "CreateStack should not return an error")

			expectStack := &model.Stack{
				Name:          "test-stack",
				Branch:        "test-user/stack-test-stack/TOP",
				Base:          "main",
				Owner:         "test-owner",
				RepoName:      "test-repo",
				Created:       time.Now(), // clock is paused during synctest
				SyncHash:      "f635465c16516362eed06541e0168a07c364e21a",
				BaseRef:       "f635465c16516362eed06541e0168a07c364e21a",
				MergedChanges: []model.Change{},
			}
			assert.Equal(t, expectStack, v, "CreateStack should return expected Stack object")
		})
	})
}

func newTestClient(t *testing.T) *git.Client {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init failed: %s", string(output))

	// Set user name and email for reproducible commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	cmd.Run()

	// Create git client early so we can use createCommitWithTrailers
	gitClient, err := git.NewClientAt(tempDir)
	require.NoError(t, err)

	// Create an initial commit on main using the helper
	_ = createCommitWithTrailers(t, gitClient, "Initial commit", "", map[string]string{})

	return gitClient
}

type MockGithubClient struct {
	mock.Mock
}

// BatchGetPRs implements GithubClient.
func (m *MockGithubClient) BatchGetPRs(owner string, repoName string, prNumbers []int) (*gh.BatchPRsResult, error) {
	args := m.Called(owner, repoName, prNumbers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gh.BatchPRsResult), args.Error(1)
}

// CreatePRComment implements GithubClient.
func (m *MockGithubClient) CreatePRComment(prNumber int, body string) (string, error) {
	args := m.Called(prNumber, body)
	return args.String(0), args.Error(1)
}

// GetRepoInfo implements GithubClient.
func (m *MockGithubClient) GetRepoInfo() (owner string, repoName string, err error) {
	args := m.Called()
	return args.String(0), args.String(1), args.Error(2)
}

// ListPRComments implements GithubClient.
func (m *MockGithubClient) ListPRComments(prNumber int) ([]gh.Comment, error) {
	args := m.Called(prNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gh.Comment), args.Error(1)
}

// MarkPRDraft implements GithubClient.
func (m *MockGithubClient) MarkPRDraft(prNumber int) error {
	args := m.Called(prNumber)
	return args.Error(0)
}

// MarkPRReady implements GithubClient.
func (m *MockGithubClient) MarkPRReady(prNumber int) error {
	args := m.Called(prNumber)
	return args.Error(0)
}

// UpdatePRComment implements GithubClient.
func (m *MockGithubClient) UpdatePRComment(commentID string, body string) error {
	args := m.Called(commentID, body)
	return args.Error(0)
}

var _ GithubClient = (*MockGithubClient)(nil)

// createCommitWithTrailers creates a commit with the specified message and trailers
func createCommitWithTrailers(t *testing.T, gitClient *git.Client, title, body string, trailers map[string]string) string {
	msg := git.CommitMessage{
		Title:    title,
		Body:     body,
		Trailers: trailers,
	}

	// Write a test file to commit
	testFile := filepath.Join(gitClient.GitRoot(), fmt.Sprintf("file-%d.txt", time.Now().UnixNano()))
	err := os.WriteFile(testFile, []byte(body), 0644)
	require.NoError(t, err)

	// Stage the file
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = gitClient.GitRoot()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	// Create commit with message including trailers
	cmd = exec.Command("git", "commit", "-m", msg.String())
	cmd.Dir = gitClient.GitRoot()
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2024-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2024-01-01T00:00:00Z",
	)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitClient.GitRoot()
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git rev-parse failed: %s", string(output))

	return strings.TrimSpace(string(output))
}

func TestGetStackContext_WithMultipleActiveChanges(t *testing.T) {
	mockGithubClient := &MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := newTestStackClient(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 3 commits with trailers
	uuid1 := "1111111111111111"
	uuid2 := "2222222222222222"
	uuid3 := "3333333333333333"

	_ = createCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	_ = createCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
		"PR-UUID":  uuid2,
		"PR-Stack": "test-stack",
	})

	_ = createCommitWithTrailers(t, stackClient.git.(*git.Client), "Third change", "Description of third change", map[string]string{
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
	mockGithubClient := &MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := newTestStackClient(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 3 commits with trailers
	uuid1 := "aaaa111111111111"
	uuid2 := "bbbb222222222222"
	uuid3 := "cccc333333333333"

	hash1 := createCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	_ = createCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
		"PR-UUID":  uuid2,
		"PR-Stack": "test-stack",
	})

	_ = createCommitWithTrailers(t, stackClient.git.(*git.Client), "Third change", "Description of third change", map[string]string{
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
	mockGithubClient := &MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

	stackClient := newTestStackClient(t, mockGithubClient)

	// Create a stack
	stack, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	// Create 2 commits with trailers
	uuid1 := "dddd111111111111"
	uuid2 := "eeee222222222222"

	hash1 := createCommitWithTrailers(t, stackClient.git.(*git.Client), "First change", "Description of first change", map[string]string{
		"PR-UUID":  uuid1,
		"PR-Stack": "test-stack",
	})

	hash2 := createCommitWithTrailers(t, stackClient.git.(*git.Client), "Second change", "Description of second change", map[string]string{
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
