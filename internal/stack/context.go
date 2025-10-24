package stack

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/model"
)

// StackContext represents a snapshot of stack state for the current branch or a stack loaded by name.
type StackContext struct {
	StackName          string
	Stack              *model.Stack
	AllChanges         []model.Change // Complete history (merged + active)
	ActiveChanges      []model.Change // Only unmerged changes from TOP branch
	StaleMergedChanges []model.Change // Active changes that are merged on GitHub but still on TOP branch (need restack)
	currentUUID        string         // UUID where we are positioned
	onUUIDBranch       bool           // true if on UUID branch (editing specific change)
	stackActive        bool           // true if this stack is currently active in repo
}

func (s *StackContext) IsStack() bool {
	return s.StackName != ""
}

func (s *StackContext) OnUUIDBranch() bool {
	return s.onUUIDBranch
}

func (s *StackContext) CurrentChange() *model.Change {
	if s.currentUUID == "" {
		return nil
	}
	return s.FindChange(s.currentUUID)
}

func (s *StackContext) GetCurrentPositionUUID() string {
	return s.currentUUID
}

// GetCurrentPosition returns the current position in the stack.
// - If on a UUID branch (editing a specific change), returns that change's position
// - If on TOP branch, returns the highest position (len(ActiveChanges))
// Returns an error if the position cannot be determined.
func (s *StackContext) GetCurrentPosition() (int, error) {
	if s.onUUIDBranch {
		currentChange := s.CurrentChange()
		if currentChange == nil {
			return 0, fmt.Errorf("failed to determine current position")
		}
		return currentChange.Position, nil
	}
	// On TOP branch means we're at the highest position
	return len(s.AllChanges), nil
}

func (s *StackContext) FindChange(uuid string) *model.Change {
	for i := range s.AllChanges {
		if s.AllChanges[i].UUID == uuid {
			return &s.AllChanges[i]
		}
	}
	return nil
}

func (s *StackContext) FindChangeInActive(uuid string) *model.Change {
	for i := range s.ActiveChanges {
		if s.ActiveChanges[i].UUID == uuid {
			return &s.ActiveChanges[i]
		}
	}
	return nil
}

func (s *StackContext) FormatUUIDBranch(username string, uuid string) string {
	return fmt.Sprintf("%s/stack-%s/%s", username, s.StackName, uuid)
}

func FormatStackBranch(username string, stackName string) string {
	return fmt.Sprintf("%s/stack-%s/TOP", username, stackName)
}

// ValidateBottomUpMerges ensures that only bottom PRs are merged (no out-of-order merges).
func ValidateBottomUpMerges(activeChanges []model.Change, mergedPRNumbers map[int]bool) error {
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

func ExtractUUIDFromBranch(branch string) (stackName string, uuid string) {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return "", ""
	}

	return strings.TrimPrefix(parts[1], "stack-"), parts[2]
}
