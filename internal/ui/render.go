package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
)

// RenderStackList renders a list of all stacks with styling
// Now uses tree visualization by default via RenderStackListTree
func RenderStackList(stacks []*stack.Stack, currentStackName string, stackChanges map[string][]model.Change) string {
	// Use the new tree-based visualization
	return RenderStackListTree(stacks, stackChanges, currentStackName)
}

// RenderStackDetails renders detailed information about a stack
// Now uses tree visualization by default via RenderStackTree
// Accepts currentUUID to show current position indicator
func RenderStackDetails(s *stack.Stack, changes []model.Change, currentUUID string) string {
	var output strings.Builder

	// Render the tree visualization with current position
	treeView := RenderStackTree(s, changes, currentUUID)
	output.WriteString(treeView)
	output.WriteString("\n\n")

	// Add summary statistics
	if len(changes) > 0 {
		output.WriteString(buildSummaryLine(changes))
		output.WriteString("\n\n")
	}

	// Add legend
	legendContent := FormatStatus("open") + " - PR is open and ready for review\n" +
		FormatStatus("draft") + " - PR is in draft state\n" +
		FormatStatus("merged") + " - PR has been merged (tracked in stack metadata)\n" +
		FormatStatus("local") + " - Not yet pushed to GitHub\n" +
		Dim("(modified)") + " - PR has local changes that need to be pushed"

	legend := RenderPanel(legendContent)
	output.WriteString(Dim("Legend:"))
	output.WriteString("\n")
	output.WriteString(legend)

	return output.String()
}

// RenderStackSummary renders a brief summary of a stack
func RenderStackSummary(s *stack.Stack, changes []model.Change) string {
	open, draft, merged, _, local, needsPush := CountPRsByState(changes)
	totalPRs := len(changes)

	summary := Bold(s.Name) + "\n" +
		Dim("Base: ") + Muted(s.Base) + "\n" +
		Dim("PRs:  ")

	if totalPRs == 0 {
		summary += Muted("none")
	} else {
		summary += FormatPRSummary(open, draft, merged, local, needsPush)
	}

	return RenderPanel(summary)
}

// RenderSwitchSuccess renders a success message after switching stacks
func RenderSwitchSuccess(stackName string) string {
	return SuccessStyle.Render("✓ " + fmt.Sprintf("Switched to stack: %s", Bold(stackName)))
}

// RenderEditSuccess renders a success message after starting to edit a change
func RenderEditSuccess(position int, title string, branch string) string {
	var output strings.Builder
	output.WriteString(SuccessStyle.Render("✓ " + fmt.Sprintf("Checked out change #%d: %s", position, title)))
	output.WriteString("\n")
	output.WriteString(SuccessStyle.Render("✓ " + fmt.Sprintf("Branch: %s", branch)))
	output.WriteString("\n")
	output.WriteString(InfoStyle.Render("ℹ " + "Make your changes and commit"))
	output.WriteString("\n")
	output.WriteString(Dim("  • Use 'git commit --amend' to update this change"))
	output.WriteString("\n")
	output.WriteString(Dim("  • Use 'git commit' to insert a new change after this one"))
	return output.String()
}

// NavigationSuccess contains data for rendering navigation success output
type NavigationSuccess struct {
	Message     string
	Stack       *stack.Stack
	Changes     []model.Change
	CurrentUUID string
	IsEditing   bool
}

// RenderNavigationSuccess renders a success message with compact stack tree after navigation
func RenderNavigationSuccess(data NavigationSuccess) string {
	var output strings.Builder

	// Success message
	output.WriteString(SuccessStyle.Render("✓ " + data.Message))
	output.WriteString("\n\n")

	// Compact stack tree
	treeView := RenderStackTree(data.Stack, data.Changes, data.CurrentUUID)
	output.WriteString(treeView)

	// Add summary line
	if len(data.Changes) > 0 {
		output.WriteString("\n\n")
		output.WriteString(buildSummaryLine(data.Changes))
	}

	// Add editing instructions if on UUID branch
	if data.IsEditing {
		output.WriteString("\n\n")
		output.WriteString(InfoStyle.Render("ℹ " + "Make your changes and commit"))
		output.WriteString("\n")
		output.WriteString(Dim("  • Use 'git commit --amend' to update this change"))
		output.WriteString("\n")
		output.WriteString(Dim("  • Use 'git commit' to insert a new change after this one"))
	}

	return output.String()
}

// RenderNoStacksMessage renders a message when no stacks exist
func RenderNoStacksMessage() string {
	return RenderPanel(
		Dim("No stacks found.\n\n") +
			Muted("Create your first stack with:\n  ") +
			Highlight("stack new <name>"),
	)
}

// RenderNotOnStackMessage renders a message when not on a stack branch
func RenderNotOnStackMessage() string {
	return ErrorStyle.Render("✗ " + "Not on a stack branch. Use 'stack switch' to switch to a stack.")
}

// RenderError renders an error message with styling
func RenderError(err error) string {
	return ErrorStyle.Render("✗ " + err.Error())
}

// PushProgress contains data for rendering push progress
type PushProgress struct {
	Position int    // Current position (1-indexed)
	Total    int    // Total number of PRs
	Title    string // PR title
	PRNumber int    // PR number
	URL      string // PR URL
	Action   string // "created", "updated", or "skipped"
	Reason   string // Optional reason for update (e.g., "commit changed")
}

// RenderPushProgress renders progress for pushing a PR
func RenderPushProgress(progress PushProgress) string {
	var output strings.Builder

	actionText := "Updated"
	switch progress.Action {
	case "created":
		actionText = "Created"
	case "updated":
		actionText = "Updated"
	case "skipped":
		actionText = "Skipped (unchanged)"
	}

	output.WriteString(SuccessStyle.Render(fmt.Sprintf("✓ %d/%d", progress.Position, progress.Total)))
	output.WriteString(" ")
	output.WriteString(Bold(progress.Title))
	output.WriteString("\n")
	output.WriteString("      ")
	output.WriteString(Dim(fmt.Sprintf("%s PR #%d:", actionText, progress.PRNumber)))
	output.WriteString(" ")
	output.WriteString(Muted(progress.URL))

	if progress.Reason != "" {
		output.WriteString("\n")
		output.WriteString("      ")
		output.WriteString(Dim(fmt.Sprintf("(%s)", progress.Reason)))
	}

	return output.String()
}

// RenderPushSummary renders a summary after pushing PRs
func RenderPushSummary(created, updated, skipped int) string {
	var output strings.Builder

	output.WriteString("\n")
	output.WriteString(SuccessStyle.Render("✓ " + "Push complete!"))
	output.WriteString("\n\n")

	var parts []string
	if created > 0 {
		parts = append(parts, fmt.Sprintf("%d created", created))
	}
	if updated > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", updated))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (unchanged)", skipped))
	}

	if len(parts) > 0 {
		output.WriteString(Dim("PRs: "))
		output.WriteString(strings.Join(parts, ", "))
	}

	return output.String()
}

// RenderMergedPRsTable renders a table of merged PRs for the refresh command
func RenderMergedPRsTable(mergedChanges []model.Change) string {
	if len(mergedChanges) == 0 {
		return ""
	}

	rows := make([][]string, len(mergedChanges))
	for i, change := range mergedChanges {
		prLabel := "-"
		if !change.IsLocal() {
			prLabel = StatusMergedStyle.Render(fmt.Sprintf("#%d", change.PR.PRNumber))
		}

		mergedAt := "-"
		if !change.MergedAt.IsZero() {
			mergedAt = change.MergedAt.Format("2006-01-02 15:04")
		}

		rows[i] = []string{
			fmt.Sprintf("%d", i+1),
			prLabel,
			Truncate(change.Title, 40),
			mergedAt,
		}
	}

	t := NewStackTable().
		Headers("#", "PR", "TITLE", "MERGED AT").
		Rows(rows...)

	return "\n" + t.String() + "\n"
}

// RenderStackDetailsTable renders a detailed table view of a single stack
// Accepts currentUUID to highlight the current row
func RenderStackDetailsTable(s *stack.Stack, changes []model.Change, currentUUID string) string {
	if len(changes) == 0 {
		return RenderPanel(Dim("No changes in this stack"))
	}

	var output strings.Builder

	output.WriteString(Bold(s.Name) + "  " + Dim("→") + "  " + Muted(s.Base) + "\n\n")

	rows := make([][]string, len(changes))
	for i, change := range changes {
		position := fmt.Sprintf("%d", change.Position)
		statusText := GetChangeStatus(change).Render()

		prLabel := "-"
		if !change.IsLocal() {
			prLabel = fmt.Sprintf("#%d", change.PR.PRNumber)
		}

		commit := change.CommitHash
		if len(commit) > 7 {
			commit = commit[:7]
		}

		url := "-"
		if !change.IsLocal() && change.PR.URL != "" {
			url = change.PR.URL
		}

		// Highlight current row with bold styling
		if currentUUID != "" && change.UUID == currentUUID {
			position = BoldStyle.Render(position)
			statusText = BoldStyle.Render(statusText)
			prLabel = BoldStyle.Render(prLabel)
			change.Title = BoldStyle.Render(change.Title)
			commit = BoldStyle.Render(commit)
			url = BoldStyle.Render(url)
		}

		rows[i] = []string{position, statusText, prLabel, change.Title, commit, url}
	}

	t := NewStackTable().
		Headers("#", "STATUS", "PR", "TITLE", "COMMIT", "URL").
		Rows(rows...)

	output.WriteString(t.String() + "\n\n")
	output.WriteString(buildSummaryLine(changes) + "\n\n")
	output.WriteString(Dim("Legend:") + "\n" + buildLegendPanel())

	return output.String()
}

func buildSummaryLine(changes []model.Change) string {
	open, draft, merged, closed, local, needsPush := CountPRsByState(changes)
	totalPRs := len(changes)

	prWord := "PR"
	if totalPRs != 1 {
		prWord = "PRs"
	}
	summary := Bold(fmt.Sprintf("%d %s", totalPRs, prWord)) + " " + Dim("total")

	if open+draft+merged+closed+local+needsPush == 0 {
		return summary
	}

	var states []string
	if open > 0 {
		states = append(states, StatusOpenStyle.Render(fmt.Sprintf("%d open", open)))
	}
	if draft > 0 {
		states = append(states, StatusDraftStyle.Render(fmt.Sprintf("%d draft", draft)))
	}
	if merged > 0 {
		states = append(states, StatusMergedStyle.Render(fmt.Sprintf("%d merged", merged)))
	}
	if closed > 0 {
		states = append(states, StatusClosedStyle.Render(fmt.Sprintf("%d closed", closed)))
	}
	if needsPush > 0 {
		states = append(states, StatusModifiedStyle.Render(fmt.Sprintf("%d modified", needsPush)))
	}
	if local > 0 {
		states = append(states, StatusLocalStyle.Render(fmt.Sprintf("%d local", local)))
	}

	return summary + " " + Dim("(") + strings.Join(states, Dim(", ")) + Dim(")")
}

func buildLegendPanel() string {
	content := FormatStatus("open") + " - PR is open and ready for review\n" +
		FormatStatus("draft") + " - PR is in draft state\n" +
		FormatStatus("merged") + " - PR has been merged (tracked in stack metadata)\n" +
		FormatStatus("local") + " - Not yet pushed to GitHub\n" +
		Dim("(modified)") + " - PR has local changes that need to be pushed"
	return RenderPanel(content)
}

// RenderStackListTable renders a table comparing multiple stacks
func RenderStackListTable(stacks []*stack.Stack, allChanges map[string][]model.Change, currentStackName string) string {
	if len(stacks) == 0 {
		return RenderNoStacksMessage()
	}

	rows := make([][]string, len(stacks))
	for i, s := range stacks {
		changes := allChanges[s.Name]
		open, draft, merged, _, local, _ := CountPRsByState(changes)

		name := s.Name
		if s.Name == currentStackName {
			name = "● " + name
		}

		rows[i] = []string{
			Truncate(name, 20),
			fmt.Sprintf("%d", open),
			fmt.Sprintf("%d", draft),
			fmt.Sprintf("%d", merged),
			fmt.Sprintf("%d", local),
			s.Base,
			Truncate(s.Branch, 27),
		}
	}

	t := NewStackTable().
		Headers("STACK", "OPEN", "DRAFT", "MERGED", "LOCAL", "BASE", "BRANCH").
		Rows(rows...)

	plural := ""
	if len(stacks) != 1 {
		plural = "s"
	}

	return t.String() + "\n\n" + Bold(fmt.Sprintf("%d stack%s total", len(stacks), plural)) + "\n"
}
