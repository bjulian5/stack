package stack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestImportStackFromAnyMemberBranch(t *testing.T) {
	const (
		stackName = "shared-work"
		firstUUID = "1111111111111111"
		lastUUID  = "2222222222222222"
	)
	tests := []struct {
		name      string
		branch    string
		keepLocal bool
		selected  int
	}{
		{name: "first branch", branch: "test-user/stack-shared-work/" + firstUUID, selected: 41},
		{name: "last remote branch", branch: "origin/test-user/stack-shared-work/" + lastUUID, selected: 42},
		{name: "full remote ref", branch: "refs/remotes/origin/test-user/stack-shared-work/" + firstUUID, selected: 41},
		{name: "local TOP branch", branch: "test-user/stack-shared-work/TOP", keepLocal: true, selected: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := testutil.NewTestGitClient(t)
			baseHash, err := gitClient.GetCommitHash("main")
			require.NoError(t, err)
			github := &gh.MockGithubClient{}
			github.On("GetRepoInfo").Return("repo-owner", "repo", nil)
			client := NewTestStackWithClients(t, github, gitClient)
			_, err = client.CreateStack(stackName, "main")
			require.NoError(t, err)

			firstHash := testutil.CreateCommitWithTrailers(t, gitClient, "First", "First body", map[string]string{
				"PR-UUID": firstUUID, "PR-Stack": stackName,
			})
			lastHash := testutil.CreateCommitWithTrailers(t, gitClient, "Last", "Last body", map[string]string{
				"PR-UUID": lastUUID, "PR-Stack": stackName,
			})
			firstBranch := fmt.Sprintf("test-user/stack-%s/%s", stackName, firstUUID)
			lastBranch := fmt.Sprintf("test-user/stack-%s/%s", stackName, lastUUID)
			require.NoError(t, gitClient.UpdateRef(firstBranch, firstHash))
			require.NoError(t, gitClient.UpdateRef(lastBranch, lastHash))

			remote := filepath.Join(t.TempDir(), "remote.git")
			require.NoError(t, exec.Command("git", "init", "--bare", remote).Run())
			gitCommand(t, gitClient, "remote", "add", "origin", remote)
			require.NoError(t, gitClient.PushAtomic([]string{firstBranch, lastBranch}, false))
			require.NoError(t, gitClient.CheckoutBranch("main"))
			require.NoError(t, gitClient.DeleteBranch(firstBranch, true))
			require.NoError(t, gitClient.DeleteBranch(lastBranch, true))
			if !tt.keepLocal {
				require.NoError(t, gitClient.DeleteBranch(fmt.Sprintf("test-user/stack-%s/TOP", stackName), true))
			}
			require.NoError(t, os.RemoveAll(client.getStackDir(stackName)))

			created := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
			details := map[int]gh.PRDetails{
				41: {Number: 41, State: "OPEN", Draft: false, Title: "First", Body: "First body", URL: "https://example.test/41", HeadRef: firstBranch, HeadSHA: firstHash, BaseRef: "main", CreatedAt: created, UpdatedAt: created},
				42: {Number: 42, State: "OPEN", Draft: true, Title: "Last", Body: "Last body", URL: "https://example.test/42", HeadRef: lastBranch, HeadSHA: lastHash, BaseRef: firstBranch, CreatedAt: created.Add(time.Hour), UpdatedAt: created.Add(time.Hour)},
			}
			selectedBranch := firstBranch
			if tt.selected == 42 {
				selectedBranch = lastBranch
			}
			github.On("FindPRByHead", selectedBranch).Return(&gh.PR{Number: tt.selected}, nil).Once()
			native := gh.NativeStack{Number: 7, Base: gh.NativeStackBase{Ref: "main"}, PullRequests: []gh.NativeStackPR{{Number: 41}, {Number: 42}}}
			github.On("FindStacks", mock.Anything, "repo-owner", "repo", tt.selected).Return([]gh.NativeStack{native}, nil).Once()
			github.On("GetPRDetails", mock.Anything, "repo-owner", "repo", []int{41, 42}).Return(details, gh.RepoMergeConfig{}, nil).Once()

			ctx, err := client.ImportStack(t.Context(), ImportOptions{Branch: tt.branch})
			require.NoError(t, err)
			assert.Equal(t, "test-user/stack-shared-work/TOP", ctx.Stack.Branch)
			assert.Equal(t, "main", ctx.Stack.Base)
			assert.Equal(t, baseHash, ctx.Stack.BaseRef)
			assert.Equal(t, []string{firstUUID, lastUUID}, []string{ctx.AllChanges[0].UUID, ctx.AllChanges[1].UUID})
			assert.Equal(t, 41, ctx.AllChanges[0].PR.PRNumber)
			assert.Equal(t, 42, ctx.AllChanges[1].PR.PRNumber)
			current, err := gitClient.GetCurrentBranch()
			require.NoError(t, err)
			assert.Equal(t, ctx.Stack.Branch, current)
			github.AssertExpectations(t)
		})
	}
}

func gitCommand(t *testing.T, client interface{ GitRoot() string }, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = client.GitRoot()
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, output)
}
