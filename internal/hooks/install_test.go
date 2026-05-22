package hooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/hooks"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestInstallAndCheckAndUninstall(t *testing.T) {
	c := testutil.NewTestGitClient(t)

	assert.False(t, hooks.CheckHooksInstalled(c.GitCommonDir()),
		"hooks should not be installed in a fresh repo")

	require.NoError(t, hooks.InstallHooks(c.GitCommonDir()))

	for _, hook := range []string{"prepare-commit-msg", "post-commit", "commit-msg"} {
		path := filepath.Join(c.GitCommonDir(), "hooks", hook)
		info, err := os.Stat(path)
		require.NoError(t, err, "hook %s should exist at %s", hook, path)
		// Hooks must be executable for git to run them.
		assert.NotZero(t, info.Mode()&0111, "hook %s must be executable", hook)
	}

	assert.True(t, hooks.CheckHooksInstalled(c.GitCommonDir()))

	require.NoError(t, hooks.UninstallHooks(c.GitCommonDir()))
	assert.False(t, hooks.CheckHooksInstalled(c.GitCommonDir()),
		"hooks should be gone after uninstall")
}

// Hooks installed from a linked worktree must land in the shared hooks dir of
// the main repo — not under the worktree's per-worktree git dir — so that they
// fire for every checkout. This exercises the worktree-correctness of the
// install path: passing GitCommonDir() (rather than GitRoot()+".git") routes
// the install to the shared location.
func TestInstallHooks_FromWorktree_LandsInSharedDir(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature-branch")

	require.NoError(t, hooks.InstallHooks(wt.GitCommonDir()))

	// Hooks live in the main repo's .git/hooks…
	mainHooks := filepath.Join(main.GitCommonDir(), "hooks", "prepare-commit-msg")
	_, err := os.Stat(mainHooks)
	require.NoError(t, err, "hook should be installed in shared hooks dir: %s", mainHooks)

	// …and NOT under the worktree's per-worktree git dir, which would make
	// them invisible to the main checkout.
	wtHooks := filepath.Join(wt.GitDir(), "hooks", "prepare-commit-msg")
	_, err = os.Stat(wtHooks)
	assert.True(t, os.IsNotExist(err),
		"hook must not be installed under worktree-private git dir at %s", wtHooks)

	// And from the main client we should see the install reflected.
	assert.True(t, hooks.CheckHooksInstalled(main.GitCommonDir()),
		"main client must see hooks installed from a worktree")
}

func TestUninstallHooks_LeavesForeignHooksAlone(t *testing.T) {
	c := testutil.NewTestGitClient(t)
	hooksDir := filepath.Join(c.GitCommonDir(), "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// A pre-existing user hook unrelated to stack — uninstall must not touch it.
	foreign := filepath.Join(hooksDir, "prepare-commit-msg")
	require.NoError(t, os.WriteFile(foreign, []byte("#!/bin/sh\necho user hook\n"), 0755))

	require.NoError(t, hooks.UninstallHooks(c.GitCommonDir()))

	_, err := os.Stat(foreign)
	assert.NoError(t, err, "foreign hook must survive UninstallHooks")
}
