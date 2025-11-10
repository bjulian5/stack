package stack

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/testutil"
)

// NewTestStackClient creates a new stack client for testing with a mock GitHub client
func NewTestStack(t *testing.T, gh GithubClient) *Client {
	mockGitClient := testutil.NewTestGitClient(t)
	c := NewClient(mockGitClient, gh)
	c.username = "test-user"
	return c
}

// simulateCommitRewrite simulates the scenario where a commit has been amended/rewritten,
// causing the UUID branch to point to a stale commit hash.
// This is used to test the CheckoutChangeForEditing flow when a UUID branch exists but
// points to the wrong commit (e.g., after a rebase or commit amend).
func simulateCommitRewrite(t *testing.T, gitClient *git.Client, stackName, uuid string) string {
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
