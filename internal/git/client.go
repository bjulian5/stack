package git

import (
	"fmt"
	"os/exec"
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

// IsGitRepo checks if the current directory is in a git repository
func (c *Client) IsGitRepo() bool {
	return c.gitRoot != ""
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

// CreateBranch creates a new branch at the specified commit
func (c *Client) CreateBranch(name string, commitHash string) error {
	cmd := exec.Command("git", "branch", name, commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}
	return nil
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

// DeleteBranch deletes the specified branch
func (c *Client) DeleteBranch(name string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, name)

	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", name, err)
	}
	return nil
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
	// Get commit message
	cmd := exec.Command("git", "log", "--format=%B", "-n", "1", hash)
	output, err := cmd.Output()
	if err != nil {
		return Commit{}, fmt.Errorf("failed to get commit %s: %w", hash, err)
	}

	message := string(output)
	return ParseCommitMessage(hash, message), nil
}

// GetCommitCount returns the number of commits between base and branch
func (c *Client) GetCommitCount(branch string, base string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", base, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetLocalBranches returns all local branches
func (c *Client) GetLocalBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(branches) == 1 && branches[0] == "" {
		return []string{}, nil
	}

	return branches, nil
}

// GetStackBranches returns all stack branches
func (c *Client) GetStackBranches() ([]string, error) {
	branches, err := c.GetLocalBranches()
	if err != nil {
		return nil, err
	}

	stackBranches := []string{}
	for _, branch := range branches {
		if IsStackBranch(branch) {
			stackBranches = append(stackBranches, branch)
		}
	}

	return stackBranches, nil
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
