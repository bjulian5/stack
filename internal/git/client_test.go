package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/testutil"
)

func TestClient_PushAtomicRunsPrePushHookOnce(t *testing.T) {
	client := testutil.NewTestGitClient(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	require.NoError(t, exec.Command("git", "init", "--bare", remote).Run())
	command := exec.Command("git", "remote", "add", "origin", remote)
	command.Dir = client.GitRoot()
	require.NoError(t, command.Run())

	head, err := client.GetCommitHash("HEAD")
	require.NoError(t, err)
	require.NoError(t, client.UpdateRef("stack-one", head))
	require.NoError(t, client.UpdateRef("stack-two", head))
	counter := filepath.Join(t.TempDir(), "hook-count")
	hook := filepath.Join(client.GitCommonDir(), "hooks", "pre-push")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\necho run >> \""+counter+"\"\n"), 0o755))

	require.NoError(t, client.PushAtomic([]string{"stack-one", "stack-two"}, true))
	runs, err := os.ReadFile(counter)
	require.NoError(t, err)
	assert.Equal(t, "run\n", string(runs))
}

func TestClient_PathsInMainCheckout(t *testing.T) {
	c := testutil.NewTestGitClient(t)

	assert.True(t, filepath.IsAbs(c.GitRoot()), "GitRoot must be absolute")
	assert.True(t, filepath.IsAbs(c.GitDir()), "GitDir must be absolute")
	assert.True(t, filepath.IsAbs(c.GitCommonDir()), "GitCommonDir must be absolute")

	// In a main (non-worktree) checkout the per-worktree git dir and the
	// shared common dir point at the same place: <root>/.git.
	assert.Equal(t, c.GitDir(), c.GitCommonDir(),
		"in a main checkout GitDir and GitCommonDir should be identical")
}

func TestClient_PathsInWorktree(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")

	// The worktree has its own working tree root, distinct from main.
	assert.NotEqual(t, main.GitRoot(), wt.GitRoot(),
		"worktree GitRoot must differ from main GitRoot")

	// The shared common dir is the same from either side — that's the whole
	// point of "common". Without this guarantee, metadata written in one
	// checkout would be invisible from another.
	assert.Equal(t, main.GitCommonDir(), wt.GitCommonDir(),
		"GitCommonDir must be identical between main and worktree clients")

	// The worktree's per-worktree git dir lives under the main repo's
	// .git/worktrees/<name>/ — not under the worktree's own working tree.
	wtGitDir := wt.GitDir()
	expectedPrefix := filepath.Join(main.GitCommonDir(), "worktrees") + string(filepath.Separator)
	assert.True(t, strings.HasPrefix(wtGitDir, expectedPrefix),
		"worktree GitDir %q should live under %q", wtGitDir, expectedPrefix)

	// And it must differ from the common dir — they're the same in main, but
	// distinct in a worktree.
	assert.NotEqual(t, wt.GitDir(), wt.GitCommonDir(),
		"in a worktree GitDir and GitCommonDir must differ")
}

func TestClient_IsRebaseInProgress_UsesWorktreeGitDir(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")

	// Initially neither client sees a rebase in progress.
	assert.False(t, main.IsRebaseInProgress())
	assert.False(t, wt.IsRebaseInProgress())

	// Simulate git starting a rebase in the worktree — rebase state lives
	// under the worktree's per-worktree git dir, not the shared common dir.
	rebaseDir := filepath.Join(wt.GitDir(), "rebase-merge")
	require.NoError(t, os.MkdirAll(rebaseDir, 0755))

	// The worktree client should now see the rebase…
	assert.True(t, wt.IsRebaseInProgress(),
		"worktree client must read rebase state from its per-worktree git dir")

	// …but the main checkout's client should NOT, because the rebase is
	// per-worktree state, not shared. If we accidentally pointed
	// IsRebaseInProgress at the common dir, both would trip — and that's the
	// bug this assertion guards against.
	assert.False(t, main.IsRebaseInProgress(),
		"main client must not see another worktree's rebase state")
}
