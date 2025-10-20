package git

import (
	"fmt"
	"strings"
)

// IsStackBranch checks if a branch name matches the stack branch pattern
func IsStackBranch(branch string) bool {
	// Stack branches follow pattern: username/stack-<name>/TOP
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return false
	}

	// Check second part starts with "stack-" and third part is "TOP"
	return strings.HasPrefix(parts[1], "stack-") && parts[2] == "TOP"
}

// IsUUIDBranch checks if a branch name matches the UUID branch pattern
func IsUUIDBranch(branch string) bool {
	// UUID branches follow pattern: username/stack-<name>/<uuid>
	// where <uuid> is 16 hex characters (NOT "TOP")
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return false
	}

	// Check if the second part starts with "stack-"
	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return false
	}

	// Check if the third part looks like a UUID (16 hex chars)
	// Must NOT be "TOP" (that's the stack branch)
	possibleUUID := parts[2]
	if possibleUUID == "TOP" || len(possibleUUID) != 16 {
		return false
	}

	// Check if it's all hex characters
	for _, c := range possibleUUID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}

	return true
}

// ExtractStackName extracts the stack name from a stack branch
func ExtractStackName(branch string) string {
	// Branch format: username/stack-<name>/TOP
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return ""
	}

	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return ""
	}

	// Verify it's a stack branch (ends with /TOP)
	if parts[2] != "TOP" {
		return ""
	}

	return strings.TrimPrefix(secondPart, "stack-")
}

// ExtractUUIDFromBranch extracts stack name and UUID from a UUID branch
func ExtractUUIDFromBranch(branch string) (stackName string, uuid string) {
	// Branch format: username/stack-<name>/<uuid>
	parts := strings.Split(branch, "/")
	if len(parts) != 3 {
		return "", ""
	}

	secondPart := parts[1]
	if !strings.HasPrefix(secondPart, "stack-") {
		return "", ""
	}

	stackName = strings.TrimPrefix(secondPart, "stack-")
	uuid = parts[2]

	return stackName, uuid
}

// FormatStackBranch formats a stack branch name
func FormatStackBranch(username string, stackName string) string {
	return fmt.Sprintf("%s/stack-%s/TOP", username, stackName)
}

// FormatUUIDBranch formats a UUID branch name
func FormatUUIDBranch(username string, stackName string, uuid string) string {
	return fmt.Sprintf("%s/stack-%s/%s", username, stackName, uuid)
}
