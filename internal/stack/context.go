package stack

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/model"
)

// StackContext represents a snapshot of stack state for the current branch or a stack loaded by name.
// It provides both the complete change history and the current editing context.
type StackContext struct {
	StackName          string
	Stack              *model.Stack
	changes            map[string]*model.Change // Source of truth: all changes indexed by UUID
	client             *Client                  // Client for persistence operations
	AllChanges         []*model.Change          // Complete history (merged + active)
	ActiveChanges      []*model.Change          // Only unmerged changes from TOP branch
	StaleMergedChanges []*model.Change          // Changes merged on GitHub but still on TOP branch
	currentUUID        string                   // UUID of the current editing position
	onUUIDBranch       bool                     // Whether positioned on a UUID branch
	stackActive        bool                     // Whether this stack is the active stack in the repo
	username           string                   // Username for branch naming
}

// IsStack returns true if this context represents a stack (vs a regular branch).
func (s *StackContext) IsStack() bool {
	return s.StackName != ""
}

// OnUUIDBranch returns true if positioned on a UUID branch (editing a specific change).
func (s *StackContext) OnUUIDBranch() bool {
	return s.onUUIDBranch
}

// CurrentChange returns the change at the current editing position, or nil if at TOP.
func (s *StackContext) CurrentChange() *model.Change {
	if s.currentUUID == "" {
		return nil
	}
	return s.FindChange(s.currentUUID)
}

// ChangeID returns the UUID of the current editing position.
func (s *StackContext) ChangeID() string {
	return s.currentUUID
}

// FindChange looks up a change by UUID in the stack.
func (s *StackContext) FindChange(uuid string) *model.Change {
	return s.changes[uuid]
}

// FindChangeInActive looks up a change by UUID and verifies it's in the active (unmerged) set.
func (s *StackContext) FindChangeInActive(uuid string) *model.Change {
	change := s.changes[uuid]
	if change == nil {
		return nil
	}

	for _, activeChange := range s.ActiveChanges {
		if activeChange.UUID == uuid {
			return change
		}
	}
	return nil
}

// FormatUUIDBranch returns the branch name for a UUID in this stack.
func (s *StackContext) FormatUUIDBranch(uuid string) string {
	return fmt.Sprintf("%s/stack-%s/%s", s.username, s.StackName, uuid)
}

// Save persists the current state to disk, including PR metadata and stack configuration.
func (ctx *StackContext) Save() error {
	if ctx.client == nil {
		return fmt.Errorf("cannot save: StackContext has no client reference")
	}

	prData := &model.PRData{
		Version: 1,
		PRs:     make(map[string]*model.PR),
	}

	for uuid, change := range ctx.changes {
		if change.PR != nil {
			prData.PRs[uuid] = change.PR
		}
	}

	if err := ctx.client.savePRs(ctx.StackName, prData); err != nil {
		return fmt.Errorf("failed to save PR data: %w", err)
	}

	if ctx.Stack != nil {
		if err := ctx.client.SaveStack(ctx.Stack); err != nil {
			return fmt.Errorf("failed to save stack metadata: %w", err)
		}
	}

	return nil
}

// Branch formatting and validation helpers

func formatStackBranch(username, stackName string) string {
	return fmt.Sprintf("%s/stack-%s/TOP", username, stackName)
}

// validateBottomUpMerges ensures that only bottom PRs are merged (no out-of-order merges).
func validateBottomUpMerges(activeChanges []*model.Change, mergedPRNumbers map[int]bool) error {
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
				"out-of-order merge detected: PR #%d (change #%d) is merged, but change #%d is not\n\n"+
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

// Branch parsing helpers

func isUUIDBranch(branch string) bool {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return false
	}

	uuid := parts[2]
	return uuid != "TOP" && validUUID(uuid)
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

func extractStackName(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return ""
	}

	suffix := parts[2]
	if suffix != "TOP" && !validUUID(suffix) {
		return ""
	}

	return strings.TrimPrefix(parts[1], "stack-")
}

func extractUUIDFromBranch(branch string) (stackName string, uuid string) {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return "", ""
	}

	return strings.TrimPrefix(parts[1], "stack-"), parts[2]
}
