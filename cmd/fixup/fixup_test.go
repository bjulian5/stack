package fixup

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestFixupWithCommitRef(t *testing.T) {
	testCases := []struct {
		desc        string
		target      func(firstHash, secondHash string) string
		wantTitles  []string
		expectError string
	}{
		{
			desc: "short commit sha",
			target: func(firstHash, secondHash string) string {
				return firstHash[:7]
			},
			wantTitles: []string{"First change", "Second change"},
		},
		{
			desc: "addressable commit ref",
			target: func(firstHash, secondHash string) string {
				return "HEAD~1"
			},
			wantTitles: []string{"First change", "Second change"},
		},
		{
			desc: "commit outside active stack",
			target: func(firstHash, secondHash string) string {
				return "main"
			},
			expectError: "commit main is not an active change in stack 'test-stack'",
		},
		{
			desc: "invalid commit ref",
			target: func(firstHash, secondHash string) string {
				return "does-not-exist"
			},
			expectError: "failed to resolve commit does-not-exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ghClient, gitClient, stackClient, firstHash, secondHash := setupFixupStack(t)
			stageFixupChange(t, gitClient, "fixup.txt", "fixup content")

			cmd := Command{
				Git:          gitClient,
				Stack:        stackClient,
				TargetCommit: tc.target(firstHash, secondHash),
			}

			err := cmd.Run(t.Context())
			if tc.expectError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.expectError)
				ghClient.AssertExpectations(t)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantTitles, commitTitles(t, gitClient, "main..HEAD"))
			currentBranch, err := gitClient.GetCurrentBranch()
			require.NoError(t, err)
			assert.Equal(t, "test-user/stack-test-stack/TOP", currentBranch)
			ghClient.AssertExpectations(t)
		})
	}
}

func setupFixupStack(t *testing.T) (*gh.MockGithubClient, *git.Client, *stack.Client, string, string) {
	t.Helper()

	ghClient := &gh.MockGithubClient{}
	gitClient := testutil.NewTestGitClient(t)
	stackClient := stack.NewTestStackWithClients(t, ghClient, gitClient)

	ghClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)
	_, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)

	err = stackClient.SwitchStack("test-stack")
	require.NoError(t, err)

	firstHash := testutil.CreateCommitWithTrailers(t, gitClient, "First change", "Description of first change", map[string]string{
		"PR-UUID":  "1111111111111111",
		"PR-Stack": "test-stack",
	})

	secondHash := testutil.CreateCommitWithTrailers(t, gitClient, "Second change", "Description of second change", map[string]string{
		"PR-UUID":  "2222222222222222",
		"PR-Stack": "test-stack",
	})

	return ghClient, gitClient, stackClient, firstHash, secondHash
}

func stageFixupChange(t *testing.T, gitClient *git.Client, filename, content string) {
	t.Helper()

	testutil.WriteFile(t, gitClient.GitRoot(), filename, content)

	cmd := exec.Command("git", "add", filename)
	cmd.Dir = gitClient.GitRoot()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))
}

func commitTitles(t *testing.T, gitClient *git.Client, revRange string) []string {
	t.Helper()

	cmd := exec.Command("git", "log", "--reverse", "--format=%s", revRange)
	cmd.Dir = gitClient.GitRoot()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git log failed: %s", string(output))

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "\n")
}
