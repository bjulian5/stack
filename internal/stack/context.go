package stack

import (
	"fmt"
	"strings"
)

// StackContext represents a snapshot of stack state.
// This can represent the current branch context or a stack loaded by name.
type StackContext struct {
	// StackName is the name of the stack.
	// Empty if not a stack-related context.
	StackName string

	// Stack is the loaded stack metadata.
	// Nil if not a stack context.
	Stack *Stack

	// Changes contains all changes in the stack.
	Changes []Change

	// currentUUID is the UUID of the change being edited.
	// Empty if not editing (e.g., on TOP branch or loaded by name).
	currentUUID string
}

// IsStack returns true if this context represents a stack.
func (s *StackContext) IsStack() bool {
	return s.StackName != ""
}

// IsEditing returns true if this context represents editing a specific change.
func (s *StackContext) IsEditing() bool {
	return s.currentUUID != ""
}

// CurrentChange returns the change being edited, or nil if not editing.
func (s *StackContext) CurrentChange() *Change {
	if !s.IsEditing() {
		return nil
	}
	return s.FindChange(s.currentUUID)
}

// FindChange finds a change by UUID in this stack.
func (s *StackContext) FindChange(uuid string) *Change {
	for i := range s.Changes {
		if s.Changes[i].UUID == uuid {
			return &s.Changes[i]
		}
	}
	return nil
}

// FormatUUIDBranch formats a UUID branch name for a change in this stack.
// Returns a branch name in the format: username/stack-<name>/<uuid>
func (s *StackContext) FormatUUIDBranch(username string, uuid string) string {
	return fmt.Sprintf("%s/stack-%s/%s", username, s.StackName, uuid)
}

// FormatStackBranch formats a stack branch name (TOP branch).
// Returns a branch name in the format: username/stack-<name>/TOP
func FormatStackBranch(username string, stackName string) string {
	return fmt.Sprintf("%s/stack-%s/TOP", username, stackName)
}

// isUUIDBranch checks if a branch name matches the UUID branch pattern
func isUUIDBranch(branch string) bool {
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
