package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/hooks"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestInstallReusesCurrentSetupWithoutWrites(t *testing.T) {
	main := testutil.NewTestGitClient(t)
	wt := testutil.NewWorktree(t, main, "feature")
	command := &Command{Git: wt, Stack: stack.NewClient(wt, nil)}
	require.NoError(t, command.Run(nil, nil))
	paths := []string{
		filepath.Join(main.GitCommonDir(), "hooks", "prepare-commit-msg"),
		filepath.Join(main.GitCommonDir(), "hooks", "post-commit"),
		filepath.Join(main.GitCommonDir(), "hooks", "commit-msg"),
		filepath.Join(main.GitCommonDir(), "config"),
		filepath.Join(main.GitCommonDir(), "stack", "config.json"),
	}
	infos := make([]os.FileInfo, len(paths))
	for i, path := range paths {
		info, err := os.Stat(path)
		require.NoError(t, err)
		infos[i] = info
		mode := os.FileMode(0444)
		if filepath.Base(filepath.Dir(path)) == "hooks" {
			mode = 0555
		}
		require.NoError(t, os.Chmod(path, mode))
		t.Cleanup(func() { _ = os.Chmod(path, info.Mode()) })
	}
	require.NoError(t, command.Run(nil, nil))
	for i, path := range paths {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, infos[i].ModTime(), info.ModTime(), path)
	}
}

func TestInstallRepairsSetup(t *testing.T) {
	cases := []struct {
		name   string
		damage func(*testing.T, *Command)
	}{
		{name: "missing hook", damage: func(t *testing.T, c *Command) {
			require.NoError(t, os.Remove(filepath.Join(c.Git.GitCommonDir(), "hooks", "commit-msg")))
		}},
		{name: "outdated hook", damage: func(t *testing.T, c *Command) {
			require.NoError(t, os.WriteFile(filepath.Join(c.Git.GitCommonDir(), "hooks", "commit-msg"), []byte("#!/bin/sh\n# Installed by stack\n"), 0755))
		}},
		{name: "non executable hook", damage: func(t *testing.T, c *Command) {
			require.NoError(t, os.Chmod(filepath.Join(c.Git.GitCommonDir(), "hooks", "commit-msg"), 0644))
		}},
		{name: "git configuration", damage: func(t *testing.T, c *Command) { require.NoError(t, c.Git.SetConfig("core.commentChar", "#")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := testutil.NewTestGitClient(t)
			c := &Command{Git: git, Stack: stack.NewClient(git, nil)}
			require.NoError(t, c.Run(nil, nil))
			tc.damage(t, c)
			require.NoError(t, c.Run(nil, nil))
			assert.True(t, hooks.CheckHooksCurrent(git.GitCommonDir()))
			value, err := git.GetConfig("core.commentChar")
			require.NoError(t, err)
			assert.Equal(t, ";", value)
		})
	}
}
