package bottom

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestBottom(t *testing.T) {
	testCases := []struct {
		desc        string
		setup       func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client)
		verify      func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client)
		expectError error
	}{
		{
			desc: "not on a stack branch returns error",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				// On main branch by default
			},
			expectError: fmt.Errorf("not on a stack branch"),
		},
		{
			desc: "no active changes",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)
				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)
			},
			expectError: fmt.Errorf("no active changes in stack"),
		},
		{
			desc: "single active change moves to bottom no-ops",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)

				testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})
			},
			verify: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client) {
				currentBranch, err := gitClient.GetCurrentBranch()
				require.NoError(t, err)
				expectedBranch := "test-user/stack-test-stack/TOP"
				assert.Equal(t, expectedBranch, currentBranch)
			},
		},
		{
			desc: "multiple active changes moves to bottom",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)

				testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				testutil.CreateCommitWithTrailers(t, gitClient, "Second change", "Description of second change", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})
			},
			verify: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client) {
				currentBranch, err := gitClient.GetCurrentBranch()
				require.NoError(t, err)
				expectedBranch := "test-user/stack-test-stack/1111111111111111"
				assert.Equal(t, expectedBranch, currentBranch)
			},
		},
		{
			desc: "uncommitted changes returns error",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)

				testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				// Create uncommitted changes
				testutil.WriteFile(t, gitClient.GitRoot(), "test.txt", "uncommitted content")
			},
			expectError: fmt.Errorf("uncommitted changes detected"),
		},
		{
			desc: "already at bottom stays on bottom",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)

				testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				testutil.CreateCommitWithTrailers(t, gitClient, "Second change", "Description of second change", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})

				// Move to bottom first
				stackCtx, err := stackClient.GetStackContext()
				require.NoError(t, err)
				_, err = stackClient.CheckoutChangeForEditing(stackCtx, stackCtx.ActiveChanges[0])
				require.NoError(t, err)
			},
			verify: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client) {
				currentBranch, err := gitClient.GetCurrentBranch()
				require.NoError(t, err)
				expectedBranch := "test-user/stack-test-stack/1111111111111111"
				assert.Equal(t, expectedBranch, currentBranch)
			},
		},
		{
			desc: "navigates to bottom from middle of stack",
			setup: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client, stackClient *stack.Client) {
				ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

				_, err := stackClient.CreateStack("test-stack", "main")
				require.NoError(t, err)

				err = stackClient.SwitchStack("test-stack")
				require.NoError(t, err)

				testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
					"PR-UUID":  "1111111111111111",
					"PR-Stack": "test-stack",
				})

				testutil.CreateCommitWithTrailers(t, gitClient, "Second change", "Description of second change", map[string]string{
					"PR-UUID":  "2222222222222222",
					"PR-Stack": "test-stack",
				})

				testutil.CreateCommitWithTrailers(t, gitClient, "Third change", "Description of third change", map[string]string{
					"PR-UUID":  "3333333333333333",
					"PR-Stack": "test-stack",
				})

				// Move to middle change
				stackCtx, err := stackClient.GetStackContext()
				require.NoError(t, err)
				_, err = stackClient.CheckoutChangeForEditing(stackCtx, stackCtx.ActiveChanges[1])
				require.NoError(t, err)
			},
			verify: func(t *testing.T, ghClient *gh.MockGithubClient, gitClient *git.Client) {
				currentBranch, err := gitClient.GetCurrentBranch()
				require.NoError(t, err)
				expectedBranch := "test-user/stack-test-stack/1111111111111111"
				assert.Equal(t, expectedBranch, currentBranch)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ghClient := &gh.MockGithubClient{}
			gitClient := testutil.NewTestGitClient(t)
			stackClient := stack.NewTestStackWithClients(t, ghClient, gitClient)
			cmd := Command{
				Git:   gitClient,
				Stack: stackClient,
			}
			tc.setup(t, ghClient, gitClient, stackClient)

			err := cmd.Run(t.Context())
			if tc.expectError != nil {
				assert.ErrorContains(t, err, tc.expectError.Error())
			} else {
				assert.NoError(t, err)
			}

			if tc.verify != nil {
				tc.verify(t, ghClient, gitClient)
			}
			ghClient.AssertExpectations(t)
		})
	}
}
