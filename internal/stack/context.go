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

	// AllChanges contains the complete history (merged + active changes).
	// Use for display purposes to show full stack history.
	AllChanges []Change

	// ActiveChanges contains only unmerged changes from the TOP branch.
	// Use for all operations: navigation, editing, pushing.
	ActiveChanges []Change

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
// Searches AllChanges (both merged and active) to find the change.
func (s *StackContext) FindChange(uuid string) *Change {
	for i := range s.AllChanges {
		if s.AllChanges[i].UUID == uuid {
			return &s.AllChanges[i]
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

// ValidateBottomUpMerges ensures that only bottom PRs are merged (no out-of-order merges).
// Returns an error if any PR in the middle or top is merged while earlier PRs are not.
func ValidateBottomUpMerges(activeChanges []Change, mergedPRNumbers map[int]bool) error {
	if len(mergedPRNumbers) == 0 {
		return nil // No newly merged PRs
	}

	// Find the first change that is NOT newly merged
	firstUnmergedIdx := -1
	for i, change := range activeChanges {
		if change.PR == nil {
			// Local change, not pushed yet - definitely not merged
			firstUnmergedIdx = i
			break
		}
		if !mergedPRNumbers[change.PR.PRNumber] {
			// This PR is not in the newly merged set
			firstUnmergedIdx = i
			break
		}
	}

	// If all changes are merged, that's fine
	if firstUnmergedIdx == -1 {
		return nil
	}

	// Check if any changes after the first unmerged are in the merged set
	for i := firstUnmergedIdx + 1; i < len(activeChanges); i++ {
		change := activeChanges[i]
		if change.PR != nil && mergedPRNumbers[change.PR.PRNumber] {
			// Found an out-of-order merge!
			return fmt.Errorf(
				"out-of-order merge detected: PR #%d (change #%d) is merged, but change #%d is not.\n\n"+
					"Stack requires bottom-up merging. To fix:\n"+
					"  1. Ask for PR #%d to be reverted, OR\n"+
					"  2. Manually merge the earlier PRs first, OR\n"+
					"  3. Use git commands to manually rebase the stack",
				change.PR.PRNumber, i+1, firstUnmergedIdx+1, change.PR.PRNumber,
			)
		}
	}

	return nil
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
