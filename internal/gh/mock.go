package gh

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockGithubClient struct {
	mock.Mock
}

func (m *MockGithubClient) FindPRByHead(head string) (*PR, error) {
	args := m.Called(head)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PR), args.Error(1)
}

func (m *MockGithubClient) FindStacks(ctx context.Context, owner, repo string, prNumber int) ([]NativeStack, error) {
	args := m.Called(ctx, owner, repo, prNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]NativeStack), args.Error(1)
}

func (m *MockGithubClient) CreateStack(ctx context.Context, owner, repo string, prs []int) error {
	return m.Called(ctx, owner, repo, prs).Error(0)
}

func (m *MockGithubClient) AddToStack(ctx context.Context, owner, repo string, stackNumber int, prs []int) error {
	return m.Called(ctx, owner, repo, stackNumber, prs).Error(0)
}

func (m *MockGithubClient) Unstack(ctx context.Context, owner, repo string, stackNumber int) (int, []byte, error) {
	args := m.Called(ctx, owner, repo, stackNumber)
	return args.Int(0), args.Get(1).([]byte), args.Error(2)
}

func (m *MockGithubClient) UpdatePRBase(ctx context.Context, owner, repo string, number int, base string) error {
	return m.Called(ctx, owner, repo, number, base).Error(0)
}

func (m *MockGithubClient) GetPRDetails(ctx context.Context, owner, repo string, numbers []int) (map[int]PRDetails, RepoMergeConfig, error) {
	args := m.Called(ctx, owner, repo, numbers)
	if args.Get(0) == nil {
		return nil, RepoMergeConfig{}, args.Error(2)
	}
	return args.Get(0).(map[int]PRDetails), args.Get(1).(RepoMergeConfig), args.Error(2)
}

// BatchGetPRs implements GithubClient.
func (m *MockGithubClient) BatchGetPRs(owner string, repoName string, prNumbers []int) (*BatchPRsResult, error) {
	args := m.Called(owner, repoName, prNumbers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BatchPRsResult), args.Error(1)
}

// GetRepoInfo implements GithubClient.
func (m *MockGithubClient) GetRepoInfo() (owner string, repoName string, err error) {
	args := m.Called()
	return args.String(0), args.String(1), args.Error(2)
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
