package hook

import (
	"os"
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

func TestPostCommitPreservesTrailersOnMessageOnlyAmend(t *testing.T) {
	const (
		stackName = "test-stack"
		oldUUID   = "1111111111111111"
		newUUID   = "2222222222222222"
	)
	for _, tt := range []struct {
		name string
		edit bool
	}{
		{name: "TOP branch"},
		{name: "edit branch", edit: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := testutil.NewTestGitClient(t)
			github := &gh.MockGithubClient{}
			github.On("GetRepoInfo").Return("owner", "repo", nil)
			stackClient := stack.NewTestStackWithClients(t, github, gitClient)
			_, err := stackClient.CreateStack(stackName, "main")
			require.NoError(t, err)
			oldHash := testutil.CreateCommitWithTrailers(t, gitClient, "Original", "", map[string]string{
				"PR-UUID": oldUUID, "PR-Stack": stackName,
			})
			uuidBranch := "test-user/stack-test-stack/" + oldUUID
			require.NoError(t, gitClient.CreateBranchAt(uuidBranch, oldHash))
			if tt.edit {
				require.NoError(t, gitClient.CheckoutBranch(uuidBranch))
			}

			amendMessage := git.CommitMessage{Title: "Replacement", Trailers: map[string]string{
				"PR-UUID": newUUID, "PR-Stack": stackName,
			}}
			command := exec.Command("git", "commit", "--amend", "--allow-empty", "-m", amendMessage.String())
			command.Dir = gitClient.GitRoot()
			command.Env = append(os.Environ(),
				"GIT_AUTHOR_DATE=2024-01-02T00:00:00Z",
				"GIT_COMMITTER_DATE=2024-01-02T00:00:00Z",
			)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "git commit --amend failed: %s", output)

			require.NoError(t, (&PostCommitCommand{Git: gitClient, Stack: stackClient}).Run())
			head, err := gitClient.GetCommit("HEAD")
			require.NoError(t, err)
			assert.Equal(t, "Replacement", head.Message.Title)
			assert.Equal(t, map[string]string{"PR-UUID": oldUUID, "PR-Stack": stackName}, head.Message.Trailers)
			rawMessage := gitOutput(t, gitClient, "show", "-s", "--format=%B", "HEAD")
			assert.Equal(t, 1, strings.Count(rawMessage, "PR-UUID:"))
			assert.Equal(t, 1, strings.Count(rawMessage, "PR-Stack:"))
			topHash, err := gitClient.GetCommitHash("test-user/stack-test-stack/TOP")
			require.NoError(t, err)
			uuidHash, err := gitClient.GetCommitHash(uuidBranch)
			require.NoError(t, err)
			assert.Equal(t, topHash, uuidHash)
			github.AssertExpectations(t)
		})
	}
}

func TestPostCommitKeepsFreshTrailersForNewCommit(t *testing.T) {
	gitClient := testutil.NewTestGitClient(t)
	github := &gh.MockGithubClient{}
	github.On("GetRepoInfo").Return("owner", "repo", nil)
	stackClient := stack.NewTestStackWithClients(t, github, gitClient)
	_, err := stackClient.CreateStack("test-stack", "main")
	require.NoError(t, err)
	testutil.CreateCommitWithTrailers(t, gitClient, "First", "", map[string]string{
		"PR-UUID": "1111111111111111", "PR-Stack": "test-stack",
	})
	testutil.CreateCommitWithTrailers(t, gitClient, "Second", "", map[string]string{
		"PR-UUID": "2222222222222222", "PR-Stack": "test-stack",
	})

	require.NoError(t, (&PostCommitCommand{Git: gitClient, Stack: stackClient}).Run())
	head, err := gitClient.GetCommit("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "2222222222222222", head.Message.Trailers["PR-UUID"])
}

func gitOutput(t *testing.T, client *git.Client, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = client.GitRoot()
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, output)
	return string(output)
}
