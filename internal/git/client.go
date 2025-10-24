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

func (c *Client) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) CheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", name, err)
	}
	return nil
}

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

func (c *Client) BranchExists(name string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", name)
	return cmd.Run() == nil
}

func (c *Client) GetCommitHash(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash for %s: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) GetCommits(branch string, base string) ([]Commit, error) {
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

func (c *Client) GetCommit(hash string) (Commit, error) {
	actualHash, err := c.GetCommitHash(hash)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to resolve %s: %w", hash, err)
	}

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

func (c *Client) CherryPick(commitHash string) error {
	cmd := exec.Command("git", "cherry-pick", commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to cherry-pick %s: %w", commitHash, err)
	}
	return nil
}

func (c *Client) ResetHard(ref string) error {
	cmd := exec.Command("git", "reset", "--hard", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to %s: %w", ref, err)
	}
	return nil
}

func (c *Client) AmendCommitMessage(message string) error {
	cmd := exec.Command("git", "commit", "--amend", "-m", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	return nil
}

func (c *Client) RebaseOnto(newBase string, upstream string, branch string) error {
	cmd := exec.Command("git", "rebase", "--onto", newBase, upstream, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rebase: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (c *Client) GetParentCommit(commitHash string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commitHash+"^")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get parent of %s: %w", commitHash, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) GetCommitTree(commitHash string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commitHash+"^{tree}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get tree for %s: %w", commitHash, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) CommitTree(treeHash string, parentHash string, message string) (string, error) {
	cmd := exec.Command("git", "commit-tree", treeHash, "-p", parentHash, "-m", message)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to commit tree: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Client) IsRebaseInProgress() bool {
	rebaseMerge := filepath.Join(c.gitRoot, ".git", "rebase-merge")
	rebaseApply := filepath.Join(c.gitRoot, ".git", "rebase-apply")

	if _, err := os.Stat(rebaseMerge); err == nil {
		return true
	}
	if _, err := os.Stat(rebaseApply); err == nil {
		return true
	}

	return false
}

func (c *Client) UpdateRef(branchName string, commitHash string) error {
	cmd := exec.Command("git", "update-ref", "refs/heads/"+branchName, commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update ref %s to %s: %w", branchName, commitHash, err)
	}
	return nil
}

func (c *Client) HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func (c *Client) HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}
	return false, nil
}

func (c *Client) CommitFixup(commitHash string) error {
	cmd := exec.Command("git", "commit", "--fixup", commitHash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create fixup commit: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// RebaseInteractiveAutosquash runs an interactive rebase with autosquash from the specified commit.
// Uses GIT_SEQUENCE_EDITOR=true to automatically apply the rebase plan without user interaction.
func (c *Client) RebaseInteractiveAutosquash(fromCommit string) error {
	cmd := exec.Command("git", "rebase", "-i", "--autosquash", fromCommit)
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase has conflicts\n\nTo resolve:\n  1. Fix conflicts in the affected files\n  2. Stage resolved files: git add <files>\n  3. Continue rebase: git rebase --continue\n  4. Or abort: git rebase --abort\n\nError: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (c *Client) Push(branch string, force bool) error {
	args := []string{"push"}

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

	return remotes[0], nil
}

func (c *Client) Fetch(remote string) error {
	cmd := exec.Command("git", "fetch", remote)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fetch from %s: %w\nOutput: %s", remote, err, string(output))
	}
	return nil
}

func (c *Client) CreateBranchAt(branchName string, ref string) error {
	cmd := exec.Command("git", "branch", branchName, ref)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s at %s: %w\nOutput: %s", branchName, ref, err, string(output))
	}
	return nil
}

func (c *Client) Rebase(onto string) error {
	cmd := exec.Command("git", "rebase", onto)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (c *Client) DeleteBranch(branchName string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, branchName)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w\nOutput: %s", branchName, err, string(output))
	}
	return nil
}

func (c *Client) DeleteRemoteBranch(branchName string) error {
	remote, err := c.GetRemoteName()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "push", remote, "--delete", branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "remote ref does not exist") {
			return nil
		}
		return fmt.Errorf("failed to delete remote branch %s: %w\nOutput: %s", branchName, err, string(output))
	}
	return nil
}

func (c *Client) SetConfig(key string, value string) error {
	cmd := exec.Command("git", "config", key, value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set config %s=%s: %w", key, value, err)
	}
	return nil
}

// StripComments removes git comment lines from a message using git stripspace.
// Respects the configured comment character (core.commentChar).
func (c *Client) StripComments(message string) (string, error) {
	cmd := exec.Command("git", "stripspace", "--strip-comments")
	cmd.Stdin = strings.NewReader(message)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to strip comments: %w", err)
	}
	return string(output), nil
}

// GetUpstreamBranch returns the upstream tracking branch for a given branch.
// Returns empty string if no upstream is configured.
func (c *Client) GetUpstreamBranch(branch string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", branch+"@{u}")
	output, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}
