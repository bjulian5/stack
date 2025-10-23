package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/stack"
)

// RenderStackList renders a list of all stacks with styling
// Now uses tree visualization by default via RenderStackListTree
func RenderStackList(stacks []*stack.Stack, currentStackName string, stackChanges map[string][]stack.Change) string {
	// Use the new tree-based visualization
	return RenderStackListTree(stacks, stackChanges, currentStackName)
}

// RenderStackDetails renders detailed information about a stack
// Now uses tree visualization by default via RenderStackTree
func RenderStackDetails(s *stack.Stack, changes []stack.Change) string {
	var output strings.Builder

	// Render the tree visualization
	treeView := RenderStackTree(s, changes)
	output.WriteString(treeView)
	output.WriteString("\n\n")

	// Add summary statistics
	if len(changes) > 0 {
		open, draft, merged, closed, local, needsPush := CountPRsByState(changes)
		totalPRs := len(changes)

		var summaryParts []string
		summaryParts = append(summaryParts, Bold(fmt.Sprintf("%d PR", totalPRs)))
		if totalPRs != 1 {
			summaryParts[0] = Bold(fmt.Sprintf("%d PRs", totalPRs))
		}
		summaryParts = append(summaryParts, Dim("total"))

		if open > 0 || draft > 0 || merged > 0 || local > 0 || needsPush > 0 {
			summaryParts = append(summaryParts, Dim("("))
			var stateParts []string
			if open > 0 {
				stateParts = append(stateParts, StatusOpenStyle.Render(fmt.Sprintf("%d open", open)))
			}
			if draft > 0 {
				stateParts = append(stateParts, StatusDraftStyle.Render(fmt.Sprintf("%d draft", draft)))
			}
			if merged > 0 {
				stateParts = append(stateParts, StatusMergedStyle.Render(fmt.Sprintf("%d merged", merged)))
			}
			if closed > 0 {
				stateParts = append(stateParts, StatusClosedStyle.Render(fmt.Sprintf("%d closed", closed)))
			}
			if needsPush > 0 {
				stateParts = append(stateParts, StatusModifiedStyle.Render(fmt.Sprintf("%d modified", needsPush)))
			}
			if local > 0 {
				stateParts = append(stateParts, StatusLocalStyle.Render(fmt.Sprintf("%d local", local)))
			}
			for i, part := range stateParts {
				if i > 0 {
					summaryParts = append(summaryParts, Dim(", "))
				}
				summaryParts = append(summaryParts, part)
			}
			summaryParts = append(summaryParts, Dim(")"))
		}

		summary := strings.Join(summaryParts, " ")
		output.WriteString(summary)
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
func RenderStackSummary(s *stack.Stack, changes []stack.Change) string {
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
