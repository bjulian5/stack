package stack

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

// newTestStackClient creates a new stack client for testing with a mock GitHub client
func newTestStackClient(t *testing.T, gh GithubClient) *Client {
	mockGitClient := newTestClient(t)
	c := NewClient(mockGitClient, gh)
	c.username = "test-user"
	return c
}

// newTestClient creates a new git client in a temporary directory with an initial commit
func newTestClient(t *testing.T) *git.Client {
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
	_ = createCommitWithTrailers(t, gitClient, "Initial commit", "", map[string]string{})

	return gitClient
}

// createCommitWithTrailers creates a commit with the specified message and trailers
func createCommitWithTrailers(t *testing.T, gitClient *git.Client, title, body string, trailers map[string]string) string {
	msg := git.CommitMessage{
		Title:    title,
		Body:     body,
		Trailers: trailers,
	}

	// Write a test file to commit - use title + body for uniqueness
	// (time.Now() doesn't work in synctest as time is frozen)
	testFile := filepath.Join(gitClient.GitRoot(), fmt.Sprintf("file-%s.txt", title))
	err := os.WriteFile(testFile, []byte(fmt.Sprintf("%s\n%s", title, body)), 0644)
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

// simulateCommitRewrite simulates the scenario where a commit has been amended/rewritten,
// causing the UUID branch to point to a stale commit hash.
// This is used to test the CheckoutChangeForEditing flow when a UUID branch exists but
// points to the wrong commit (e.g., after a rebase or commit amend).
func simulateCommitRewrite(t *testing.T, client *Client, gitClient *git.Client, stackName, uuid string) string {
	// Create UUID branch at current commit (before amend)
	currentCommitHash, err := gitClient.GetCommitHash("HEAD")
	require.NoError(t, err)

	uuidBranch := fmt.Sprintf("test-user/stack-%s/%s", stackName, uuid)
	err = gitClient.CreateAndCheckoutBranchAt(uuidBranch, currentCommitHash)
	require.NoError(t, err)

	// Return to TOP branch
	topBranch := fmt.Sprintf("test-user/stack-%s/TOP", stackName)
	err = gitClient.CheckoutBranch(topBranch)
	require.NoError(t, err)

	// Reset to parent commit
	cmd := exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = gitClient.GitRoot()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git reset failed: %s", string(output))

	// Amend the commit to create a new hash
	cmd = exec.Command("git", "commit", "--amend", "-m",
		fmt.Sprintf("First change\n\nAmended description\n\nPR-UUID: %s\nPR-Stack: %s", uuid, stackName))
	cmd.Dir = gitClient.GitRoot()
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2024-01-02T00:00:00Z",
		"GIT_COMMITTER_DATE=2024-01-02T00:00:00Z",
	)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit --amend failed: %s", string(output))

	// Get the new commit hash
	newCommitHash, err := gitClient.GetCommitHash("HEAD")
	require.NoError(t, err)

	// Recreate the second commit (if there was one)
	// This is done by the caller in the test

	return newCommitHash
}
