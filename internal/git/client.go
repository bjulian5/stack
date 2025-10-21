package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client provides git operations for a repository
type Client struct {
	gitRoot string
}

// NewClient creates a new git client for the current directory
func NewClient() (*Client, error) {
	gitRoot, err := getGitRoot()
	if err != nil {
		return nil, err
	}
	return &Client{gitRoot: gitRoot}, nil
}

// GitRoot returns the root directory of the git repository
func (c *Client) GitRoot() string {
	return c.gitRoot
}

// GetCurrentBranch returns the name of the current git branch
func (c *Client) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CheckoutBranch checks out the specified branch
func (c *Client) CheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", name, err)
	}
	return nil
}

// CreateAndCheckoutBranch creates a new branch and checks it out
func (c *Client) CreateAndCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s: %w", name, err)
	}
	return nil
}

// CreateAndCheckoutBranchAt creates a new branch at a specific commit and checks it out.
// This is equivalent to: git checkout -b <name> <commitHash>
//
// Preconditions:
//   - The branch must not already exist (use BranchExists() to check first)
//   - The commitHash must be a valid commit reference
//
// If the branch already exists, git will return an error and this function will fail.
// Use CheckoutBranch() if you want to checkout an existing branch.
func (c *Client) CreateAndCheckoutBranchAt(name string, commitHash string) error {
	cmd := exec.Command("git", "checkout", "-b", name, commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s at %s: %w", name, commitHash, err)
	}
	return nil
}

// BranchExists checks if a branch exists
func (c *Client) BranchExists(name string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", name)
	return cmd.Run() == nil
}

// GetCommitHash returns the commit hash for a given ref
func (c *Client) GetCommitHash(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash for %s: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCommits returns all commits on the given branch that are not on the base branch
func (c *Client) GetCommits(branch string, base string) ([]Commit, error) {
	// Get commit hashes
	cmd := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..%s", base, branch))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	hashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(hashes) == 1 && hashes[0] == "" {
		return []Commit{}, nil
	}

	commits := make([]Commit, 0, len(hashes))
	for _, hash := range hashes {
		commit, err := c.GetCommit(hash)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

// GetCommit returns a commit by hash
func (c *Client) GetCommit(hash string) (Commit, error) {
	// Resolve the hash to an actual SHA (in case it's "HEAD" or another ref)
	actualHash, err := c.GetCommitHash(hash)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to resolve %s: %w", hash, err)
	}

	// Get commit message
	cmd := exec.Command("git", "log", "--format=%B", "-n", "1", actualHash)
	output, err := cmd.Output()
	if err != nil {
		return Commit{}, fmt.Errorf("failed to get commit %s: %w", actualHash, err)
	}

	messageStr := string(output)
	return Commit{
		Hash:    actualHash,
		Message: ParseCommitMessage(messageStr),
	}, nil
}

// getGitRoot is a private helper to get the git root directory
func getGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CherryPick cherry-picks a commit
func (c *Client) CherryPick(commitHash string) error {
	cmd := exec.Command("git", "cherry-pick", commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to cherry-pick %s: %w", commitHash, err)
	}
	return nil
}

// ResetHard resets the current branch to a specific ref
func (c *Client) ResetHard(ref string) error {
	cmd := exec.Command("git", "reset", "--hard", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to %s: %w", ref, err)
	}
	return nil
}

// AmendCommitMessage amends the HEAD commit with a new message
func (c *Client) AmendCommitMessage(message string) error {
	cmd := exec.Command("git", "commit", "--amend", "-m", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	return nil
}

// RebaseOnto performs a rebase with --onto
func (c *Client) RebaseOnto(newBase string, upstream string, branch string) error {
	cmd := exec.Command("git", "rebase", "--onto", newBase, upstream, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rebase: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// GetParentCommit returns the parent commit hash
func (c *Client) GetParentCommit(commitHash string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commitHash+"^")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get parent of %s: %w", commitHash, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCommitTree returns the tree hash for a commit
func (c *Client) GetCommitTree(commitHash string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commitHash+"^{tree}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get tree for %s: %w", commitHash, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CommitTree creates a commit from a tree with a specific message and parent
func (c *Client) CommitTree(treeHash string, parentHash string, message string) (string, error) {
	cmd := exec.Command("git", "commit-tree", treeHash, "-p", parentHash, "-m", message)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to commit tree: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// IsRebaseInProgress checks if a rebase is currently in progress
func (c *Client) IsRebaseInProgress() bool {
	rebaseMerge := filepath.Join(c.gitRoot, ".git", "rebase-merge")
	rebaseApply := filepath.Join(c.gitRoot, ".git", "rebase-apply")

	// Check if either rebase directory exists
	if _, err := os.Stat(rebaseMerge); err == nil {
		return true
	}
	if _, err := os.Stat(rebaseApply); err == nil {
		return true
	}

	return false
}

// UpdateRef updates a branch reference to point to a specific commit
func (c *Client) UpdateRef(branchName string, commitHash string) error {
	// Use git update-ref to update the branch without checking it out
	cmd := exec.Command("git", "update-ref", "refs/heads/"+branchName, commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update ref %s to %s: %w", branchName, commitHash, err)
	}
	return nil
}

// HasUncommittedChanges checks if there are any uncommitted changes in the working directory
func (c *Client) HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Push pushes a branch to the remote repository
func (c *Client) Push(branch string, force bool) error {
	args := []string{"push"}

	// Get remote name
	remote, err := c.GetRemoteName()
	if err != nil {
		return err
	}
	args = append(args, remote, branch)

	if force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w\nOutput: %s", branch, err, string(output))
	}
	return nil
}

// GetRemoteName returns the default remote name (usually "origin")
func (c *Client) GetRemoteName() (string, error) {
	cmd := exec.Command("git", "remote")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote: %w", err)
	}

	remotes := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remote configured")
	}

	// Return the first remote (usually "origin")
	return remotes[0], nil
}
