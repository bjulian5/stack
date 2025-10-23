package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bjulian5/stack/internal/stack"
)

// Truncate truncates text to maxLen with an ellipsis if needed
func Truncate(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

// PadRight pads text to the right with spaces
func PadRight(text string, width int) string {
	if len(text) >= width {
		return text
	}
	return text + strings.Repeat(" ", width-len(text))
}

// PadLeft pads text to the left with spaces
func PadLeft(text string, width int) string {
	if len(text) >= width {
		return text
	}
	return strings.Repeat(" ", width-len(text)) + text
}

// Center centers text within a given width
func Center(text string, width int) string {
	if len(text) >= width {
		return text
	}
	leftPad := (width - len(text)) / 2
	rightPad := width - len(text) - leftPad
	return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
}

// RenderBox renders content in a styled box with optional title
func RenderBox(title string, content string) string {
	style := BoxStyle
	if title != "" {
		style = style.BorderForeground(ColorPrimary)
		titleStyled := lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Render(title)
		return style.Render(fmt.Sprintf("%s\n\n%s", titleStyled, content))
	}
	return style.Render(content)
}

// RenderPanel renders content in a styled panel
func RenderPanel(content string) string {
	return PanelStyle.Render(content)
}

// RenderHeader renders a section header
func RenderHeader(text string) string {
	return HeaderStyle.Render(text)
}

// RenderTitle renders a prominent title
func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

// RenderTitlef renders a formatted prominent title
func RenderTitlef(format string, args ...interface{}) string {
	return RenderTitle(fmt.Sprintf(format, args...))
}

// RenderSubtitle renders a subtitle
func RenderSubtitle(text string) string {
	return SubtitleStyle.Render(text)
}

// RenderSuccessMessage renders a success message with checkmark
func RenderSuccessMessage(message string) string {
	return SuccessStyle.Render("✓ " + message)
}

// RenderSuccessMessagef renders a formatted success message with checkmark
func RenderSuccessMessagef(format string, args ...interface{}) string {
	return RenderSuccessMessage(fmt.Sprintf(format, args...))
}

// RenderWarningMessage renders a warning message with icon
func RenderWarningMessage(message string) string {
	return WarningStyle.Render("⚠ " + message)
}

// RenderWarningMessagef renders a formatted warning message with icon
func RenderWarningMessagef(format string, args ...interface{}) string {
	return RenderWarningMessage(fmt.Sprintf(format, args...))
}

// RenderErrorMessage renders an error message with X
func RenderErrorMessage(message string) string {
	return ErrorStyle.Render("✗ " + message)
}

// RenderErrorMessagef renders a formatted error message with X
func RenderErrorMessagef(format string, args ...interface{}) string {
	return RenderErrorMessage(fmt.Sprintf(format, args...))
}

// RenderInfoMessage renders an info message with icon
func RenderInfoMessage(message string) string {
	return InfoStyle.Render("ℹ " + message)
}

// RenderInfoMessagef renders a formatted info message with icon
func RenderInfoMessagef(format string, args ...interface{}) string {
	return RenderInfoMessage(fmt.Sprintf(format, args...))
}

// RenderBulletList renders a list with bullets
func RenderBulletList(items []string) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, DimStyle.Render("  • ")+item)
	}
	return strings.Join(lines, "\n")
}

// RenderNumberedList renders a numbered list
func RenderNumberedList(items []string) string {
	var lines []string
	for i, item := range items {
		num := DimStyle.Render(fmt.Sprintf("  %d. ", i+1))
		lines = append(lines, num+item)
	}
	return strings.Join(lines, "\n")
}

// RenderSeparator renders a horizontal separator line
func RenderSeparator(width int) string {
	if width <= 0 {
		width = 80
	}
	return DimStyle.Render(strings.Repeat("─", width))
}

// RenderKeyValue renders a key-value pair
func RenderKeyValue(key string, value string) string {
	keyStyled := DimStyle.Render(key + ":")
	return fmt.Sprintf("%s %s", keyStyled, value)
}

// RenderKeyValueList renders multiple key-value pairs
func RenderKeyValueList(pairs map[string]string, keys []string) string {
	var lines []string
	maxKeyLen := 0
	for _, key := range keys {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}
	for _, key := range keys {
		paddedKey := PadRight(key, maxKeyLen)
		keyStyled := DimStyle.Render(paddedKey + ":")
		lines = append(lines, fmt.Sprintf("%s %s", keyStyled, pairs[key]))
	}
	return strings.Join(lines, "\n")
}

// Dim renders text in a dimmed style
func Dim(text string) string {
	return DimStyle.Render(text)
}

// Bold renders text in bold
func Bold(text string) string {
	return BoldStyle.Render(text)
}

// Highlight renders text with highlight style
func Highlight(text string) string {
	return HighlightStyle.Render(text)
}

// Muted renders text in muted style
func Muted(text string) string {
	return MutedStyle.Render(text)
}

// FormatStackFinderLine formats a stack for display in fuzzy finder
// Returns a formatted line showing stack name, PR summary, base branch, and optional current marker
func FormatStackFinderLine(stackName string, base string, changes []stack.Change, currentStackName string) string {
	open, draft, merged, _, local, _ := CountPRsByState(changes)
	totalPRs := len(changes)

	displayName := Truncate(stackName, 20)

	line := fmt.Sprintf("%-20s", displayName)

	if totalPRs == 0 {
		line += "  (no PRs)"
	} else {
		line += fmt.Sprintf("  (%d PR", totalPRs)
		if totalPRs != 1 {
			line += "s"
		}
		line += ": "

		var stateParts []string
		if open > 0 {
			stateParts = append(stateParts, fmt.Sprintf("%d open", open))
		}
		if draft > 0 {
			stateParts = append(stateParts, fmt.Sprintf("%d draft", draft))
		}
		if merged > 0 {
			stateParts = append(stateParts, fmt.Sprintf("%d merged", merged))
		}
		if local > 0 {
			stateParts = append(stateParts, fmt.Sprintf("%d local", local))
		}

		if len(stateParts) > 0 {
			line += stateParts[0]
			for j := 1; j < len(stateParts); j++ {
				line += ", " + stateParts[j]
			}
		}

		line += ")"
	}

	line += fmt.Sprintf("  │  base: %s", base)

	if stackName == currentStackName {
		line += "  ← current"
	}

	return line
}

// FormatStackPreview formats a stack preview for the fuzzy finder preview window
// Returns a formatted preview showing stack details and first few PRs
func FormatStackPreview(stackName string, branch string, base string, changes []stack.Change) string {
	preview := fmt.Sprintf("Stack: %s\n", stackName)
	preview += fmt.Sprintf("Branch: %s\n", branch)
	preview += fmt.Sprintf("Base: %s\n", base)

	// Handle case where changes failed to load
	if changes == nil {
		preview += "\n(Failed to load changes)\n"
		return preview
	}

	preview += fmt.Sprintf("PRs: %d\n", len(changes))

	if len(changes) > 0 {
		preview += "\nFirst PRs in stack:\n"
		maxPreview := 5
		if len(changes) < maxPreview {
			maxPreview = len(changes)
		}
		for j := 0; j < maxPreview; j++ {
			change := changes[j]
			status := "local"
			if change.PR != nil {
				status = change.PR.State
			}
			icon := GetStatusIcon(status)
			title := Truncate(change.Title, 50)
			preview += fmt.Sprintf("  %d. %s %s\n", j+1, icon, title)
		}
		if len(changes) > maxPreview {
			preview += fmt.Sprintf("  ... and %d more\n", len(changes)-maxPreview)
		}
	}

	return preview
}

// FormatChangeFinderLine formats a change for fuzzy finder display
// Used by both 'stack edit' and 'stack pr open' commands
func FormatChangeFinderLine(change stack.Change) string {
	status := "local"
	if change.PR != nil {
		status = change.PR.State
	}
	icon := GetStatusIcon(status)

	prLabel := "local"
	if change.PR != nil {
		prLabel = fmt.Sprintf("#%-4d", change.PR.PRNumber)
	}

	// Truncate title to fit nicely
	title := Truncate(change.Title, 40)

	// Import git package for ShortHash - need to handle this differently
	// Since this is in the ui package, we can't import git
	// Instead, we'll pass the short hash directly
	shortHash := change.CommitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	return fmt.Sprintf("%2d %s %-6s │ %-40s │ %s", change.Position, icon, prLabel, title, shortHash)
}

// FormatChangePreview formats a change for fuzzy finder preview window
// Used by both 'stack edit' and 'stack pr open' commands
func FormatChangePreview(change stack.Change) string {
	preview := fmt.Sprintf("Position: %d\n", change.Position)
	preview += fmt.Sprintf("Title: %s\n", change.Title)
	preview += fmt.Sprintf("Commit: %s\n", change.CommitHash)
	if change.UUID != "" {
		preview += fmt.Sprintf("UUID: %s\n", change.UUID)
	}
	if change.PR != nil {
		preview += fmt.Sprintf("PR: #%d (%s)\n", change.PR.PRNumber, change.PR.State)
		preview += fmt.Sprintf("URL: %s\n", change.PR.URL)
	}
	if change.Description != "" {
		preview += fmt.Sprintf("\nDescription:\n%s\n", change.Description)
	}
	return preview
}
