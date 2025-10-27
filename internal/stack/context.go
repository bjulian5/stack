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
	changes            map[string]*model.Change // Source of truth: changes indexed by UUID
	client             *Client                  // Client for persistence operations
	AllChanges         []*model.Change          // Complete history (merged + active) - pointers into changes map
	ActiveChanges      []*model.Change          // Only unmerged changes from TOP branch - pointers into changes map
	StaleMergedChanges []*model.Change          // Active changes that are merged on GitHub but still on TOP branch (need restack) - pointers into changes map
	currentUUID        string                   // UUID where we are positioned
	onUUIDBranch       bool                     // true if on UUID branch (editing specific change)
	stackActive        bool                     // true if this stack is currently active in repo
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

func (s *StackContext) FindChange(uuid string) *model.Change {
	return s.changes[uuid]
}

func (s *StackContext) FindChangeInActive(uuid string) *model.Change {
	// Check if the change exists in the map first
	change := s.changes[uuid]
	if change == nil {
		return nil
	}
	// Verify it's actually in the active changes
	for _, activeChange := range s.ActiveChanges {
		if activeChange.UUID == uuid {
			return change
		}
	}
	return nil
}

func (s *StackContext) FormatUUIDBranch(username string, uuid string) string {
	return fmt.Sprintf("%s/stack-%s/%s", username, s.StackName, uuid)
}

// Save persists the current state of all changes to disk (prs.json and config.json)
func (ctx *StackContext) Save() error {
	if ctx.client == nil {
		return fmt.Errorf("StackContext has no client reference for persistence")
	}

	// Build PRData from the changes map
	prData := &model.PRData{
		Version: 1,
		PRs:     make(map[string]*model.PR),
	}

	for uuid, change := range ctx.changes {
		if change.PR != nil {
			prData.PRs[uuid] = change.PR
		}
	}

	// Save PR data
	if err := ctx.client.SavePRs(ctx.StackName, prData); err != nil {
		return err
	}

	// Save Stack metadata (includes merged changes, sync metadata, etc.)
	if ctx.Stack != nil {
		if err := ctx.client.SaveStack(ctx.Stack); err != nil {
			return err
		}
	}

	return nil
}

func formatStackBranch(username string, stackName string) string {
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

func isUUIDBranch(branch string) bool {
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

func extractStackName(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], "stack-") {
		return ""
	}

	if parts[2] != "TOP" && !validUUID(parts[2]) {
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
