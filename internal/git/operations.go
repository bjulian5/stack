package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentBranch returns the name of the current git branch
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitRoot returns the root directory of the git repository
func GetGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// IsGitRepo checks if the current directory is in a git repository
func IsGitRepo() bool {
	_, err := GetGitRoot()
	return err == nil
}

// CreateBranch creates a new branch at the specified commit
func CreateBranch(name string, commitHash string) error {
	cmd := exec.Command("git", "branch", name, commitHash)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}
	return nil
}

// CheckoutBranch checks out the specified branch
func CheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", name, err)
	}
	return nil
}

// CreateAndCheckoutBranch creates a new branch and checks it out
func CreateAndCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s: %w", name, err)
	}
	return nil
}

// BranchExists checks if a branch exists
func BranchExists(name string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", name)
	return cmd.Run() == nil
}

// GetCommitHash returns the commit hash for a given ref
func GetCommitHash(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash for %s: %w", ref, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// DeleteBranch deletes the specified branch
func DeleteBranch(name string, force bool) error {
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
