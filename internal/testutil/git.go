package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/git"
)

// NewTestGitClient creates a new git client in a temporary directory with an initial commit
func NewTestGitClient(t *testing.T) *git.Client {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init failed: %s", string(output))

	// Set user name and email for reproducible commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	cmd.Run()

	// Create git client early so we can use createCommitWithTrailers
	gitClient, err := git.NewClientAt(tempDir)
	require.NoError(t, err)

	// Create an initial commit on main using the helper
	_ = CreateCommitWithTrailers(t, gitClient, "Initial commit", "", map[string]string{})

	return gitClient
}

// createCommitWithTrailers creates a commit with the specified message and trailers
func CreateCommitWithTrailers(t *testing.T, gitClient *git.Client, title, body string, trailers map[string]string) string {
	msg := git.CommitMessage{
		Title:    title,
		Body:     body,
		Trailers: trailers,
	}

	// Write a test file to commit - use title + body for uniqueness
	// (time.Now() doesn't work in synctest as time is frozen)
	testFile := filepath.Join(gitClient.GitRoot(), fmt.Sprintf("file-%s.txt", title))
	err := os.WriteFile(testFile, fmt.Appendf(nil, "%s\n%s", title, body), 0644)
	require.NoError(t, err)

	// Stage the file
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = gitClient.GitRoot()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	// Create commit with message including trailers
	cmd = exec.Command("git", "commit", "-m", msg.String())
	cmd.Dir = gitClient.GitRoot()
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2024-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2024-01-01T00:00:00Z",
	)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitClient.GitRoot()
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git rev-parse failed: %s", string(output))

	return strings.TrimSpace(string(output))
}
