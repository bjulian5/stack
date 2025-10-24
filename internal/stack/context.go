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

	// currentUUID is the UUID where we are positioned in this stack.
	// Set when on UUID branch OR TOP branch.
	// Empty only when loaded by name (not currently on this stack).
	currentUUID string

	// onUUIDBranch indicates if we're on a UUID branch (editing a specific change).
	// false when on TOP branch or loaded by name.
	onUUIDBranch bool

	// stackActive indicates if this stack is the currently active stack in the repo.
	// i.e., if the current Git branch is part of this stack.
	stackActive bool
}

// IsStack returns true if this context represents a stack.
func (s *StackContext) IsStack() bool {
	return s.StackName != ""
}

// OnUUIDBranch returns true if this context represents editing a specific change.
// This means we're on a UUID branch (not TOP branch).
func (s *StackContext) OnUUIDBranch() bool {
	return s.onUUIDBranch
}

// CurrentChange returns the change at the current position (where HEAD is),
// or nil if not on this stack's branches or currentUUID is not set.
func (s *StackContext) CurrentChange() *Change {
	if s.currentUUID == "" {
		return nil
	}
	return s.FindChange(s.currentUUID)
}

// GetCurrentPositionUUID returns the UUID where the arrow should point.
// Returns empty string if we're not on this stack's branches.
func (s *StackContext) GetCurrentPositionUUID() string {
	return s.currentUUID
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
func ValidateBottomUpMerges(activeChanges []Change, mergedPRNumbers map[int]bool) error {
	if len(mergedPRNumbers) == 0 {
		return nil
	}

	firstUnmergedIdx := -1
	for i, change := range activeChanges {
		if change.IsLocal() || !mergedPRNumbers[change.PR.PRNumber] {
			firstUnmergedIdx = i
			break
		}
	}

	if firstUnmergedIdx == -1 {
		return nil
	}

	for i := firstUnmergedIdx + 1; i < len(activeChanges); i++ {
		change := activeChanges[i]
		if !change.IsLocal() && mergedPRNumbers[change.PR.PRNumber] {
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

// IsUUIDBranch checks if a branch name matches the UUID branch pattern.
// Pattern: username/stack-<name>/<uuid> where <uuid> is 16 hex characters (not "TOP").
func IsUUIDBranch(branch string) bool {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return false
	}

	uuid := parts[2]
	if uuid == "TOP" || len(uuid) != 16 {
		return false
	}

	return validUUID(uuid)
}

func validUUID(uuid string) bool {
	if len(uuid) != 16 {
		return false
	}
	for _, c := range uuid {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ExtractStackName extracts the stack name from a stack branch.
// Branch format: username/stack-<name>/TOP or username/stack-<name>/<uuid>
func ExtractStackName(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return ""
	}

	if parts[2] != "TOP" && !validUUID(parts[2]) {
		return ""
	}

	return strings.TrimPrefix(parts[1], "stack-")
}

// ExtractUUIDFromBranch extracts stack name and UUID from a UUID branch.
// Branch format: username/stack-<name>/<uuid>
func ExtractUUIDFromBranch(branch string) (stackName string, uuid string) {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return "", ""
	}

	return strings.TrimPrefix(parts[1], "stack-"), parts[2]
}
