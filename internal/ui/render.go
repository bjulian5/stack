package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/charmbracelet/lipgloss"
)

// RenderStackList renders a list of all stacks with styling
func RenderStackList(stacks []*stack.Stack, currentStackName string, stackChanges map[string][]stack.Change) string {
	if len(stacks) == 0 {
		return RenderPanel(
			Dim("No stacks found.\n") +
				Muted("Create a new stack with: ") + Highlight("stack new <name>"),
		)
	}

	var output strings.Builder

	output.WriteString(RenderTitle("📚 Available Stacks"))
	output.WriteString("\n\n")

	for _, s := range stacks {
		changes := stackChanges[s.Name]
		open, draft, merged, _, local := CountPRsByState(changes)
		totalPRs := len(changes)

		isCurrent := s.Name == currentStackName

		var panel strings.Builder

		nameStyle := BoldStyle
		if isCurrent {
			nameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)
			panel.WriteString(nameStyle.Render(s.Name))
			panel.WriteString(" ")
			panel.WriteString(SuccessStyle.Render("← current"))
		} else {
			panel.WriteString(nameStyle.Render(s.Name))
		}
		panel.WriteString("\n")

		panel.WriteString(Dim("Branch: "))
		panel.WriteString(Muted(s.Branch))
		panel.WriteString("\n")
		panel.WriteString(Dim("Base:   "))
		panel.WriteString(Muted(s.Base))
		panel.WriteString("\n")

		panel.WriteString(Dim("PRs:    "))
		if totalPRs == 0 {
			panel.WriteString(Muted("none"))
		} else {
			summary := FormatPRSummary(open, draft, merged, local)
			panel.WriteString(summary)
		}

		boxStyle := BoxStyle
		if isCurrent {
			boxStyle = lipgloss.NewStyle().
				Border(BorderRounded).
				BorderForeground(ColorPrimary).
				Padding(0, 1)
		}

		output.WriteString(boxStyle.Render(panel.String()))
		output.WriteString("\n\n")
	}

	// Footer
	totalStacks := len(stacks)
	pluralStacks := "stack"
	if totalStacks != 1 {
		pluralStacks = "stacks"
	}
	footer := Dim(fmt.Sprintf("%d %s total", totalStacks, pluralStacks))
	output.WriteString(footer)
	output.WriteString("\n")

	return output.String()
}

// RenderStackDetails renders detailed information about a stack
func RenderStackDetails(s *stack.Stack, changes []stack.Change) string {
	var output strings.Builder

	headerContent := Bold(s.Name) + "\n" +
		Dim("Branch: ") + Muted(s.Branch) + "\n" +
		Dim("Base:   ") + Muted(s.Base)

	header := RenderBorderedContent(headerContent, "Stack Details")
	output.WriteString(header)
	output.WriteString("\n\n")

	if len(changes) == 0 {
		noChanges := RenderPanel(
			Dim("No PRs in this stack yet.\n") +
				Muted("Add commits to the stack branch to create PRs."),
		)
		output.WriteString(noChanges)
		return output.String()
	}

	table := NewTable([]Column{
		{Header: "#", Width: 3, Align: AlignRight},
		{Header: "Status", Width: 9, Align: AlignLeft},
		{Header: "PR", MinWidth: 45, MaxWidth: 70, Align: AlignLeft},
		{Header: "Title", MinWidth: 30, MaxWidth: 50, Align: AlignLeft},
		{Header: "Commit", Width: 7, Align: AlignLeft},
	})

	for _, change := range changes {
		pos := fmt.Sprintf("%d", change.Position)
		status := FormatChangeStatus(change)
		prLabel := FormatPRLabel(change.PR)
		title := Truncate(change.Title, 40)
		shortHash := change.CommitHash
		if len(shortHash) > git.ShortHashLength {
			shortHash = shortHash[:git.ShortHashLength]
		}
		shortHash = Dim(shortHash)

		// AddRow should never fail here since we're passing exactly 5 cells to a 5-column table
		// If it does fail, it's a programming bug that should be caught during development
		if err := table.AddRow(pos, status, prLabel, title, shortHash); err != nil {
			panic(fmt.Sprintf("BUG: failed to add table row: %v", err))
		}
	}

	output.WriteString(table.Render())
	output.WriteString("\n\n")

	open, draft, merged, closed, local := CountPRsByState(changes)
	totalPRs := len(changes)

	var summaryParts []string
	summaryParts = append(summaryParts, Bold(fmt.Sprintf("%d PR", totalPRs)))
	if totalPRs != 1 {
		summaryParts[0] = Bold(fmt.Sprintf("%d PRs", totalPRs))
	}
	summaryParts = append(summaryParts, Dim("total"))

	if open > 0 || draft > 0 || merged > 0 || local > 0 {
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

	legendContent := FormatStatus("open") + " - PR is open and ready for review\n" +
		FormatStatus("draft") + " - PR is in draft state\n" +
		FormatStatus("merged") + " - PR has been merged\n" +
		FormatStatus("local") + " - Not yet pushed to GitHub"

	legend := RenderPanel(legendContent)
	output.WriteString(Dim("Legend:"))
	output.WriteString("\n")
	output.WriteString(legend)

	return output.String()
}

// RenderStackSummary renders a brief summary of a stack
func RenderStackSummary(s *stack.Stack, changes []stack.Change) string {
	open, draft, merged, _, local := CountPRsByState(changes)
	totalPRs := len(changes)

	summary := Bold(s.Name) + "\n" +
		Dim("Base: ") + Muted(s.Base) + "\n" +
		Dim("PRs:  ")

	if totalPRs == 0 {
		summary += Muted("none")
	} else {
		summary += FormatPRSummary(open, draft, merged, local)
	}

	return RenderPanel(summary)
}

// RenderSwitchSuccess renders a success message after switching stacks
func RenderSwitchSuccess(stackName string) string {
	return RenderSuccessMessage(fmt.Sprintf("Switched to stack: %s", Bold(stackName)))
}

// RenderEditSuccess renders a success message after starting to edit a change
func RenderEditSuccess(position int, title string, branch string) string {
	var output strings.Builder
	output.WriteString(RenderSuccessMessage(fmt.Sprintf("Checked out change #%d: %s", position, title)))
	output.WriteString("\n")
	output.WriteString(RenderSuccessMessage(fmt.Sprintf("Branch: %s", branch)))
	output.WriteString("\n")
	output.WriteString(RenderInfoMessage("Make your changes and commit"))
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
	return RenderErrorMessage("Not on a stack branch. Use 'stack switch' to switch to a stack.")
}

// RenderError renders an error message with styling
func RenderError(err error) string {
	return RenderErrorMessage(err.Error())
}

// RenderPushProgress renders progress for pushing a PR
func RenderPushProgress(position, total int, title string, prNumber int, url string, isNew bool) string {
	var output strings.Builder

	action := "Updated"
	if isNew {
		action = "Created"
	}

	output.WriteString(SuccessStyle.Render(fmt.Sprintf("✓ %d/%d", position, total)))
	output.WriteString(" ")
	output.WriteString(Bold(title))
	output.WriteString("\n")
	output.WriteString("      ")
	output.WriteString(Dim(fmt.Sprintf("%s PR #%d:", action, prNumber)))
	output.WriteString(" ")
	output.WriteString(Muted(url))

	return output.String()
}

// RenderPushSummary renders a summary after pushing PRs
func RenderPushSummary(created, updated int) string {
	var output strings.Builder

	output.WriteString("\n")
	output.WriteString(RenderSuccessMessage("Push complete!"))
	output.WriteString("\n\n")

	var parts []string
	if created > 0 {
		parts = append(parts, fmt.Sprintf("%d created", created))
	}
	if updated > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", updated))
	}

	if len(parts) > 0 {
		output.WriteString(Dim("PRs: "))
		output.WriteString(strings.Join(parts, ", "))
	}

	return output.String()
}
