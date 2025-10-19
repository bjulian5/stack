package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// IsStackBranch checks if a branch name matches the stack branch pattern
func IsStackBranch(branch string) bool {
	// Stack branches follow pattern: username/stack-<name>
	return strings.Contains(branch, "/stack-") && !IsUUIDBranch(branch)
}

// IsUUIDBranch checks if a branch name matches the UUID branch pattern
func IsUUIDBranch(branch string) bool {
	// UUID branches follow pattern: username/stack-<name>-<uuid>
	parts := strings.Split(branch, "/")
	if len(parts) < 2 {
		return false
	}

	// Check if the second part starts with "stack-" and has a UUID suffix
	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return false
	}

	// Count hyphens - UUID branches have extra hyphen for UUID
	hyphens := strings.Count(secondPart, "-")
	return hyphens >= 2 // "stack-<name>-<uuid>" has at least 2 hyphens
}

// ExtractStackName extracts the stack name from a stack branch
func ExtractStackName(branch string) string {
	// Branch format: username/stack-<name>
	parts := strings.Split(branch, "/")
	if len(parts) < 2 {
		return ""
	}

	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return ""
	}

	return strings.TrimPrefix(secondPart, "stack-")
}

// ExtractUUIDFromBranch extracts stack name and UUID from a UUID branch
func ExtractUUIDFromBranch(branch string) (stackName string, uuid string) {
	// Branch format: username/stack-<name>-<uuid>
	parts := strings.Split(branch, "/")
	if len(parts) < 2 {
		return "", ""
	}

	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return "", ""
	}

	remainder := strings.TrimPrefix(secondPart, "stack-")

	// Find the last hyphen to separate name from UUID
	lastHyphen := strings.LastIndex(remainder, "-")
	if lastHyphen == -1 {
		return "", ""
	}

	stackName = remainder[:lastHyphen]
	uuid = remainder[lastHyphen+1:]

	return stackName, uuid
}

// FormatStackBranch formats a stack branch name
func FormatStackBranch(username string, stackName string) string {
	return fmt.Sprintf("%s/stack-%s", username, stackName)
}

// FormatUUIDBranch formats a UUID branch name
func FormatUUIDBranch(username string, stackName string, uuid string) string {
	return fmt.Sprintf("%s/stack-%s-%s", username, stackName, uuid)
}

// GetLocalBranches returns all local branches
func GetLocalBranches() ([]string, error) {
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
func GetStackBranches() ([]string, error) {
	branches, err := GetLocalBranches()
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
