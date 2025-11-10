package stack

import (
	"github.com/bjulian5/stack/internal/gh"
	"github.com/stretchr/testify/mock"
)

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
