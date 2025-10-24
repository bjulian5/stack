package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/charmbracelet/lipgloss/tree"
)

// RenderStackTree renders a single stack as a tree with its changes
// Example output:
//
//	auth-refactor
//	╰─┬ main
//	  ├─● #123 Add JWT auth (a1b2c3d)
//	  ├─◐ #124 Refresh tokens (b2c3d4e)
//	  ╰─◯ Unit tests (c3d4e5f) [local]
func RenderStackTree(s *stack.Stack, changes []model.Change, currentUUID string) string {
	if len(changes) == 0 {
		return TreeRootStyle.Render(s.Name) + "\n" + Dim("  No changes yet")
	}

	// Create root with stack name
	t := tree.Root(TreeRootStyle.Render(s.Name))

	// Add base branch as intermediate node
	baseNode := tree.Root(Muted(s.Base))

	// Add each change as a child of the base
	for _, change := range changes {
		changeLabel := formatChangeForTree(change, currentUUID)
		baseNode.Child(changeLabel)
	}

	// Add the base node to the main tree
	t.Child(baseNode)

	// Configure tree styling
	t.Enumerator(getRoundedEnumerator()).
		EnumeratorStyle(TreeEnumeratorStyle).
		Indenter(RenderTreeIndenter())

	return t.String()
}

// RenderStackTreeCompact renders a stack tree in compact form (no base branch node)
// Example output:
//
//	auth-refactor
//	├─● #123 Add JWT auth (a1b2c3d)
//	├─◐ #124 Refresh tokens (b2c3d4e)
//	╰─◯ Unit tests (c3d4e5f) [local]
func RenderStackTreeCompact(s *stack.Stack, changes []model.Change, currentUUID string) string {
	if len(changes) == 0 {
		return TreeRootStyle.Render(s.Name) + "\n" + Dim("  No changes yet")
	}

	// Create root with stack name
	t := tree.Root(TreeRootStyle.Render(s.Name))

	// Add each change directly as a child
	for _, change := range changes {
		changeLabel := formatChangeForTree(change, currentUUID)
		t.Child(changeLabel)
	}

	// Configure tree styling
	t.Enumerator(getRoundedEnumerator()).
		EnumeratorStyle(TreeEnumeratorStyle).
		Indenter(RenderTreeIndenter())

	return t.String()
}

// RenderStackListTree renders multiple stacks as a tree
// Example output:
//
//	📚 Stacks (3 total)
//	├─► auth-refactor (current)
//	│  ├─ 2 open, 1 draft, 3 total
//	│  ╰─ bjulian5/stack-auth-refactor/TOP → main
//	├─ database-migration
//	│  ├─ 1 merged, 2 open, 3 total
//	│  ╰─ bjulian5/stack-database-migration/TOP → main
//	╰─ ui-improvements
//	   ├─ 5 local
//	   ╰─ bjulian5/stack-ui-improvements/TOP → main
func RenderStackListTree(stacks []*stack.Stack, allChanges map[string][]model.Change, currentStackName string) string {
	if len(stacks) == 0 {
		return Dim("No stacks yet. Create one with: ") + Highlight("stack new <name>")
	}

	// Create root
	title := fmt.Sprintf("📚 Stacks (%d total)", len(stacks))
	t := tree.Root(HeaderStyle.Render(title))

	// Add each stack
	for _, s := range stacks {
		stackLabel := formatStackNameForTree(s.Name, currentStackName)
		stackNode := tree.Root(stackLabel)

		// Get changes for this stack
		changes, ok := allChanges[s.Name]
		if !ok {
			changes = []model.Change{}
		}

		// Add summary line
		summaryLine := formatStackSummary(changes)
		stackNode.Child(Dim(summaryLine))

		// Add branch info
		branchLine := fmt.Sprintf("%s → %s", s.Branch, s.Base)
		stackNode.Child(Muted(branchLine))

		t.Child(stackNode)
	}

	// Configure tree styling
	t.Enumerator(getRoundedEnumerator()).
		EnumeratorStyle(TreeEnumeratorStyle).
		Indenter(RenderTreeIndenter())

	return t.String()
}

// RenderChangeListTree renders a list of changes as a flat tree (for interactive selection)
// Example output:
//
//	Select a change:
//	├─ 1. ● #123 Add JWT auth (a1b2c3d)
//	├─ 2. ◐ #124 Refresh tokens (b2c3d4e)
//	╰─ 3. ◯ Unit tests (c3d4e5f) [local]
func RenderChangeListTree(changes []model.Change) string {
	if len(changes) == 0 {
		return Dim("No changes in this stack")
	}

	t := tree.Root(HeaderStyle.Render("Select a change:"))

	for i, change := range changes {
		// Don't show current indicator in selection menu
		label := fmt.Sprintf("%d. %s", i+1, formatChangeForTree(change, ""))
		t.Child(label)
	}

	// Configure tree styling
	t.Enumerator(getRoundedEnumerator()).
		EnumeratorStyle(TreeEnumeratorStyle).
		Indenter(RenderTreeIndenter())

	return t.String()
}

// formatChangeForTree formats a change for display in a tree
// If currentUUID matches this change's UUID, adds a green arrow indicator
func formatChangeForTree(change model.Change, currentUUID string) string {
	status := GetChangeStatus(change)
	icon := status.RenderCompact()

	// Format PR label
	var prLabel string
	if !change.IsLocal() {
		prLabel = fmt.Sprintf("#%d", change.PR.PRNumber)
	} else {
		prLabel = "[local]"
	}

	// Format commit hash
	commitHash := change.CommitHash
	if len(commitHash) > Display.CommitHashDisplayLength {
		commitHash = commitHash[:Display.CommitHashDisplayLength]
	}

	// Truncate title if needed
	title := change.Title
	if len(title) > Display.MaxTitleLength {
		title = title[:Display.MaxTitleLength-3] + "..."
	}

	// Build the line: "● #123 Add JWT auth (a1b2c3d)"
	line := fmt.Sprintf("%s %s %s %s",
		icon,
		Highlight(prLabel),
		title,
		Dim(fmt.Sprintf("(%s)", commitHash)),
	)

	// Add current position arrow if this is the current change
	if currentUUID != "" && change.UUID == currentUUID {
		line += " " + CurrentPositionArrowStyle.Render("←")
	}

	return line
}

// formatStackNameForTree formats a stack name with current marker
func formatStackNameForTree(stackName string, currentStackName string) string {
	if stackName == currentStackName {
		return TreeCurrentMarkerStyle.Render("► ") + TreeRootStyle.Render(stackName) + TreeCurrentMarkerStyle.Render(" (current)")
	}
	return TreeItemStyle.Render(stackName)
}

// formatStackSummary creates a summary line for a stack
func formatStackSummary(changes []model.Change) string {
	if len(changes) == 0 {
		return "No changes"
	}

	open, draft, merged, _, local, needsPush := CountPRsByState(changes)

	var parts []string
	if open > 0 {
		parts = append(parts, fmt.Sprintf("%d open", open))
	}
	if draft > 0 {
		parts = append(parts, fmt.Sprintf("%d draft", draft))
	}
	if merged > 0 {
		parts = append(parts, fmt.Sprintf("%d merged", merged))
	}
	if local > 0 {
		parts = append(parts, fmt.Sprintf("%d local", local))
	}
	if needsPush > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", needsPush))
	}

	summary := strings.Join(parts, ", ")
	total := len(changes)

	if summary == "" {
		return fmt.Sprintf("%d total", total)
	}

	return fmt.Sprintf("%s, %d total", summary, total)
}

// getRoundedEnumerator returns a custom rounded enumerator for trees
func getRoundedEnumerator() tree.Enumerator {
	return func(children tree.Children, i int) string {
		if children.Length() == 0 {
			return ""
		}

		// Check if this is the last child
		isLast := i == children.Length()-1

		if isLast {
			return "╰─ "
		}
		return "├─ "
	}
}

// getDefaultEnumerator returns the default tree enumerator
func getDefaultEnumerator() tree.Enumerator {
	return func(children tree.Children, i int) string {
		if children.Length() == 0 {
			return ""
		}

		// Check if this is the last child
		isLast := i == children.Length()-1

		if isLast {
			return "└─ "
		}
		return "├─ "
	}
}

// RenderTreeIndenter returns an indenter function for trees
func RenderTreeIndenter() tree.Indenter {
	return func(children tree.Children, i int) string {
		if children.Length() == 0 {
			return ""
		}

		// Check if this is the last child
		isLast := i == children.Length()-1

		if isLast {
			return "   " // No vertical line after last child
		}
		return "│  " // Vertical line for non-last children
	}
}
