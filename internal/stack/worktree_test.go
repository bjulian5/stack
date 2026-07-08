package stack

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/testutil"
)

// Constructing a stack client inside a linked worktree must route metadata to
// the shared git common dir — not to the worktree's per-worktree git dir,
// which is in fact a *file* pointer and would error with "not a directory" on
// any os.WriteFile against it. This is the exact failure mode the original bug
// report hit.
func TestStackClient_MetadataInWorktreeLandsInCommonDir(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")

	wtStack := NewClient(wt, &gh.MockGithubClient{})

	stack := &model.Stack{
		Name:     "demo",
		Branch:   "user/stack-demo/TOP",
		Base:     "main",
		Owner:    "owner",
		RepoName: "repo",
	}
	require.NoError(t, wtStack.SaveStack(stack))

	// The config.json must land under the shared common dir…
	expected := filepath.Join(main.GitCommonDir(), "stack", "demo", "config.json")
	_, err := os.Stat(expected)
	require.NoError(t, err, "stack config.json should exist at shared path %s", expected)

	// …and explicitly NOT under the worktree's working-tree .git pointer file.
	// Note: the worktree's .git is a regular file (containing `gitdir: ...`),
	// so any os.Stat under it errors with ENOTDIR — we just need *some* error,
	// not specifically os.IsNotExist, since the pre-fix code crashed precisely
	// because os.WriteFile against this path returned ENOTDIR.
	wrong := filepath.Join(wt.GitRoot(), ".git", "stack", "demo", "config.json")
	_, err = os.Stat(wrong)
	assert.Error(t, err,
		"stack config.json must not be reachable via the worktree's .git pointer at %s", wrong)
}

// A stack created from a worktree must be visible to a client constructed in
// the main checkout (and vice versa). This is the "shared metadata" contract:
// branches and refs are shared across worktrees, so stack metadata must be too.
func TestStackClient_MetadataVisibleAcrossWorktrees(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")

	mainStack := NewClient(main, &gh.MockGithubClient{})
	wtStack := NewClient(wt, &gh.MockGithubClient{})

	require.NoError(t, wtStack.SaveStack(&model.Stack{
		Name:     "shared",
		Branch:   "user/stack-shared/TOP",
		Base:     "main",
		Owner:    "owner",
		RepoName: "repo",
	}))

	// Main checkout should see the stack created from the worktree.
	loaded, err := mainStack.LoadStack("shared")
	require.NoError(t, err)
	assert.Equal(t, "shared", loaded.Name)
	assert.True(t, mainStack.StackExists("shared"))

	// And ListStacks from either side returns the same set.
	mainList, err := mainStack.ListStacks()
	require.NoError(t, err)
	wtList, err := wtStack.ListStacks()
	require.NoError(t, err)
	assert.Len(t, mainList, 1)
	assert.Len(t, wtList, 1)
}

// Reproduces the exact bug from the user's report: `stack new` checks
// installation status, which reads RepositoryConfig from disk. Pre-fix this
// failed in a worktree with "not a directory" because the path resolved to a
// segment inside `.git` (a pointer file). Post-fix it must succeed and return
// the not-installed default.
func TestStackClient_IsInstalledInWorktree(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")
	wtStack := NewClient(wt, &gh.MockGithubClient{})

	installed, err := wtStack.IsInstalled()
	require.NoError(t, err, "IsInstalled in a worktree must not error")
	assert.False(t, installed)
}

func TestRestackFetchFromWorktreeDoesNotMoveCheckedOutBaseBranch(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	originalMain, err := main.GetCommitHash("main")
	require.NoError(t, err)

	remotePath := filepath.Join(t.TempDir(), "remote.git")
	runGitForWorktreeTest(t, "", "init", "--bare", remotePath)
	runGitForWorktreeTest(t, main.GitRoot(), "remote", "add", "origin", remotePath)
	runGitForWorktreeTest(t, main.GitRoot(), "push", "-u", "origin", "main")

	wt := testutil.NewWorktree(t, main, "stack-worktree")
	mockGithubClient := &gh.MockGithubClient{}
	mockGithubClient.On("GetRepoInfo").Return("owner", "repo", nil)
	wtStack := NewTestStackWithClients(t, mockGithubClient, wt)

	stack, err := wtStack.CreateStack("demo", "main")
	require.NoError(t, err)
	_ = testutil.CreateCommitWithTrailers(t, wt, "Stack change", "Description", map[string]string{
		"PR-UUID":  "1111111111111111",
		"PR-Stack": "demo",
	})
	stackCtx, err := wtStack.GetStackContextByName("demo")
	require.NoError(t, err)
	require.NoError(t, wt.CheckoutBranch(stack.Branch))

	clonePath := filepath.Join(t.TempDir(), "clone")
	runGitForWorktreeTest(t, "", "clone", remotePath, clonePath)
	runGitForWorktreeTest(t, clonePath, "config", "user.email", "test@example.com")
	runGitForWorktreeTest(t, clonePath, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(clonePath, "remote-change.txt"), []byte("remote\n"), 0644))
	runGitForWorktreeTest(t, clonePath, "add", ".")
	runGitForWorktreeTest(t, clonePath, "commit", "-m", "Remote main change")
	runGitForWorktreeTest(t, clonePath, "push", "origin", "main")
	remoteMain := runGitForWorktreeTest(t, clonePath, "rev-parse", "HEAD")

	err = wtStack.Restack(stackCtx, RestackOptions{Onto: "main", Fetch: true})
	require.NoError(t, err)

	mainAfterRestack, err := main.GetCommitHash("main")
	require.NoError(t, err)
	assert.Equal(t, originalMain, mainAfterRestack, "restacking from a linked worktree must not move the checked-out local main ref")

	fetchedOriginMain, err := main.GetCommitHash("origin/main")
	require.NoError(t, err)
	assert.Equal(t, remoteMain, fetchedOriginMain)

	reloadedStack, err := wtStack.LoadStack("demo")
	require.NoError(t, err)
	assert.Equal(t, "main", reloadedStack.Base)
	assert.Equal(t, remoteMain, reloadedStack.BaseRef)

	stackParent, err := wt.GetCommitHash("HEAD~1")
	require.NoError(t, err)
	assert.Equal(t, remoteMain, stackParent, "stack commit should be rebased onto fetched origin/main")

	mockGithubClient.AssertExpectations(t)
}

func runGitForWorktreeTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(output))
	return strings.TrimSpace(string(output))
}
