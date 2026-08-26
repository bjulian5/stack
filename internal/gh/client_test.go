package gh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncPRDoesNotEditExistingPRBase(t *testing.T) {
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	require.NoError(t, os.WriteFile(gh, []byte(`#!/bin/sh
if [ "$1 $2" = "pr view" ]; then
  printf '%s' '{"number":3049,"url":"https://example.test/3049","state":"OPEN","isDraft":false}'
  exit 0
fi
if [ "$1 $2" = "pr edit" ]; then
  for arg in "$@"; do
    if [ "$arg" = "--base" ]; then
      echo 'Cannot change the base branch because the pull request is part of a stack.' >&2
      exit 1
    fi
  done
  exit 0
fi
exit 1
`), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := NewClient().SyncPR(PRSpec{
		Number: 3049,
		Title:  "updated title",
		Body:   "updated body",
		Base:   "main",
	})
	require.NoError(t, err)
}
