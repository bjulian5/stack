package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bjulian5/stack/internal/stack"
)

// Truncate truncates text to maxLen with an ellipsis if needed
// Uses lipgloss for proper ANSI-aware width handling
func Truncate(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	// Use lipgloss width to handle ANSI codes properly
	width := lipgloss.Width(text)
	if width <= maxLen {
		return text
	}

	if maxLen <= 3 {
		// Use lipgloss MaxWidth for proper truncation
		return lipgloss.NewStyle().MaxWidth(maxLen).Render(text)
	}

	// Use lipgloss MaxWidth and add ellipsis
	return lipgloss.NewStyle().MaxWidth(maxLen-3).Render(text) + "..."
}

func Pad(text string, width int, align lipgloss.Position) string {
	return lipgloss.PlaceHorizontal(width, align, text)
}

func RenderBox(title string, content string) string {
	style := BoxStyle
	if title != "" {
		style = style.BorderForeground(ColorPrimary)
		titleStyled := lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Render(title)

		combined := lipgloss.JoinVertical(lipgloss.Left, titleStyled, "", content)
		return style.Render(combined)
	}
	return style.Render(content)
}

func RenderPanel(content string) string {
	return PanelStyle.Render(content)
}

func RenderHeader(text string) string {
	return HeaderStyle.Render(text)
}

func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

func RenderTitlef(format string, args ...interface{}) string {
	return RenderTitle(fmt.Sprintf(format, args...))
}

func RenderSubtitle(text string) string {
	return SubtitleStyle.Render(text)
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
		width = GetTerminalWidth()
		if width <= 0 {
			width = Display.DefaultTerminalWidth
		}
	}
	return DimStyle.Render(strings.Repeat("─", width))
}

func RenderKeyValue(key string, value string) string {
	keyStyled := DimStyle.Render(key + ":")
	return fmt.Sprintf("%s %s", keyStyled, value)
}

func RenderKeyValueList(pairs map[string]string, keys []string) string {
	var lines []string

	maxKeyLen := 0
	for _, key := range keys {
		keyLen := lipgloss.Width(key)
		if keyLen > maxKeyLen {
			maxKeyLen = keyLen
		}
	}

	for _, key := range keys {
		// Pad key to max width
		paddedKey := Pad(key, maxKeyLen, lipgloss.Left)
		keyStyled := DimStyle.Render(paddedKey + ":")
		lines = append(lines, fmt.Sprintf("%s %s", keyStyled, pairs[key]))
	}

	return strings.Join(lines, "\n")
}

// Rows joins multiple strings vertically with newlines
// Uses lipgloss.JoinVertical for consistent layout
func Rows(items ...string) string {
	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// Columns joins multiple strings horizontally
// Uses lipgloss.JoinHorizontal for consistent layout
func Columns(items ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, items...)
}

// FormatStackFinderLine formats a stack for display in fuzzy finder
// Returns a formatted line showing stack name, PR summary, base branch, and optional current marker
// Note: Fuzzy finder doesn't support ANSI codes, so we use plain text
func FormatStackFinderLine(stackName string, base string, changes []stack.Change, currentStackName string) string {
	open, draft, merged, _, local, _ := CountPRsByState(changes)
	totalPRs := len(changes)

	// Simple truncation for stack name
	displayName := stackName
	if len(displayName) > Display.MaxStackNameLength {
		if Display.MaxStackNameLength > 3 {
			displayName = displayName[:Display.MaxStackNameLength-3] + "..."
		} else {
			displayName = displayName[:Display.MaxStackNameLength]
		}
	}

	// Pad to fixed width for alignment using simple string padding
	line := fmt.Sprintf("%-*s", Display.MaxStackNameLength, displayName)

	// Add PR summary
	if totalPRs == 0 {
		line += "  (no PRs)"
	} else {
		summary := fmt.Sprintf("(%d PR", totalPRs)
		if totalPRs != 1 {
			summary += "s"
		}
		summary += ": "

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
			summary += strings.Join(stateParts, ", ")
		}

		summary += ")"
		line += "  " + summary
	}

	line += fmt.Sprintf("  │  base: %s", base)

	if stackName == currentStackName {
		line += "  ← current"
	}

	return line
}

// FormatStackPreview formats a stack preview for the fuzzy finder preview window
// Returns a formatted preview showing stack details and first few PRs
// Note: Preview pane supports ANSI codes, so we can use styling
func FormatStackPreview(stackName string, branch string, base string, changes []stack.Change) string {
	lines := []string{
		RenderKeyValue("Stack", Bold(stackName)),
		RenderKeyValue("Branch", Muted(branch)),
		RenderKeyValue("Base", Muted(base)),
		RenderKeyValue("PRs", fmt.Sprintf("%d", len(changes))),
	}

	// Handle case where changes failed to load
	if changes == nil {
		lines = append(lines, "", Dim("(Failed to load changes)"))
		return strings.Join(lines, "\n")
	}

	// Add preview of first few PRs
	if len(changes) > 0 {
		lines = append(lines, "", Bold("First PRs in stack:"))

		maxPreview := Display.MaxPreviewLines
		if len(changes) < maxPreview {
			maxPreview = len(changes)
		}

		for j := 0; j < maxPreview; j++ {
			change := changes[j]
			status := GetChangeStatus(change)
			icon := status.RenderCompact()
			// Don't truncate titles in preview - let them wrap
			lines = append(lines, fmt.Sprintf("  %d. %s %s", j+1, icon, change.Title))
		}

		if len(changes) > maxPreview {
			lines = append(lines, Dim(fmt.Sprintf("  ... and %d more", len(changes)-maxPreview)))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatChangeFinderLine formats a change for fuzzy finder display.
// Fuzzy finder doesn't support ANSI codes, so we use plain text.
func FormatChangeFinderLine(change stack.Change) string {
	status := GetChangeStatus(change)

	prLabel := "local"
	if !change.IsLocal() {
		prLabel = fmt.Sprintf("#%d", change.PR.PRNumber)
	}

	shortHash := change.CommitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	return fmt.Sprintf("%d %s %s %s %s",
		change.Position,
		status.Icon,
		prLabel,
		change.Title,
		shortHash)
}

// FormatChangePreview formats a change for fuzzy finder preview window.
// Preview pane supports ANSI codes, so we can use styling.
func FormatChangePreview(change stack.Change) string {
	lines := []string{
		RenderKeyValue("Position", fmt.Sprintf("%d", change.Position)),
		RenderKeyValue("Title", Bold(change.Title)),
		RenderKeyValue("Commit", Muted(change.CommitHash)),
	}

	if change.UUID != "" {
		lines = append(lines, RenderKeyValue("UUID", Muted(change.UUID)))
	}

	if !change.IsLocal() {
		status := GetStatus(change.PR.State)
		prInfo := fmt.Sprintf("#%d (%s)", change.PR.PRNumber, status.Render())
		lines = append(lines, RenderKeyValue("PR", prInfo))
		lines = append(lines, RenderKeyValue("URL", Highlight(change.PR.URL)))
	}

	if change.Description != "" {
		lines = append(lines, "", Bold("Description:"), change.Description)
	}

	return strings.Join(lines, "\n")
}
