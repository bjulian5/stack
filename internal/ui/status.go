package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/stack"
)

// Status icons use Unicode geometric shapes for cross-platform compatibility.
//
// Icon choices and Unicode codepoints:
//   - IconOpen:   ● (U+25CF BLACK CIRCLE) - Solid circle indicates active/open state
//   - IconDraft:  ◐ (U+25D0 CIRCLE WITH LEFT HALF BLACK) - Half-filled shows work in progress
//   - IconMerged: ◆ (U+25C6 BLACK DIAMOND) - Diamond shape indicates completion/merged
//   - IconClosed: ○ (U+25CB WHITE CIRCLE) - Empty circle shows inactive/closed state
//   - IconLocal:  ◯ (U+25EF LARGE CIRCLE) - Larger empty circle for not-yet-pushed state
//
// Terminal compatibility:
//   - These Unicode characters are widely supported in modern terminals
//   - Requires a font with good Unicode coverage (e.g., Nerd Fonts, DejaVu, Menlo)
//   - If icons don't render correctly, check terminal font settings
//   - Some older terminals may display boxes (□) instead of shapes
//
// Design rationale:
//   - Unicode shapes chosen over emoji for better cross-platform consistency
//   - Emoji rendering varies significantly across platforms (macOS, Linux, Windows)
//   - These geometric shapes have better font support than emoji
//   - Color-blind safe: Icons have different shapes, not just different colors
//
// Future enhancement:
//   - Consider adding ASCII fallback mode for maximum compatibility
//   - Could detect terminal capabilities and switch icon styles
//   - See TODO in cmd/root.go for configuration options
const (
	IconOpen   = "●" // U+25CF - Solid circle for open
	IconDraft  = "◐" // U+25D0 - Half circle for draft
	IconMerged = "◆" // U+25C6 - Diamond for merged
	IconClosed = "○" // U+25CB - Empty circle for closed
	IconLocal  = "◯" // U+25EF - Large circle for local
)

// FormatStatus formats a PR status with icon and colored text
func FormatStatus(state string) string {
	switch state {
	case "open":
		return StatusOpenStyle.Render(IconOpen + " Open")
	case "draft":
		return StatusDraftStyle.Render(IconDraft + " Draft")
	case "merged":
		return StatusMergedStyle.Render(IconMerged + " Merged")
	case "closed":
		return StatusClosedStyle.Render(IconClosed + " Closed")
	default:
		return StatusLocalStyle.Render(IconLocal + " Local")
	}
}

// FormatStatusCompact formats a status as just the icon
func FormatStatusCompact(state string) string {
	switch state {
	case "open":
		return StatusOpenStyle.Render(IconOpen)
	case "draft":
		return StatusDraftStyle.Render(IconDraft)
	case "merged":
		return StatusMergedStyle.Render(IconMerged)
	case "closed":
		return StatusClosedStyle.Render(IconClosed)
	default:
		return StatusLocalStyle.Render(IconLocal)
	}
}

// FormatPRLabel formats a PR number with styling and full URL
func FormatPRLabel(pr *stack.PR) string {
	if pr == nil {
		return Dim("-")
	}

	// Display the full URL instead of just the PR number
	if pr.URL != "" {
		style := GetStatusStyle(pr.State)
		return style.Render(pr.URL)
	}

	// Fallback to PR number if URL is not available
	label := fmt.Sprintf("#%d", pr.PRNumber)
	style := GetStatusStyle(pr.State)
	return style.Render(label)
}

// FormatPRLabelCompact formats a PR number in compact form
func FormatPRLabelCompact(pr *stack.PR) string {
	if pr == nil {
		return Dim("-")
	}
	return fmt.Sprintf("#%d", pr.PRNumber)
}

// GetStatusIcon returns just the icon for a state
func GetStatusIcon(state string) string {
	switch state {
	case "open":
		return IconOpen
	case "draft":
		return IconDraft
	case "merged":
		return IconMerged
	case "closed":
		return IconClosed
	default:
		return IconLocal
	}
}

// FormatStatusWithCount formats a status with a count (e.g., "● 3 open")
func FormatStatusWithCount(state string, count int) string {
	if count == 0 {
		return ""
	}
	icon := GetStatusIcon(state)
	style := GetStatusStyle(state)
	text := fmt.Sprintf("%s %d %s", icon, count, state)
	return style.Render(text)
}

// FormatPRSummary formats a summary of PR counts
// e.g., "● 2 open  ◐ 1 draft  ◯ 1 local"
func FormatPRSummary(openCount, draftCount, mergedCount, localCount int) string {
	var parts []string

	if openCount > 0 {
		parts = append(parts, FormatStatusWithCount("open", openCount))
	}
	if draftCount > 0 {
		parts = append(parts, FormatStatusWithCount("draft", draftCount))
	}
	if mergedCount > 0 {
		parts = append(parts, FormatStatusWithCount("merged", mergedCount))
	}
	if localCount > 0 {
		parts = append(parts, FormatStatusWithCount("local", localCount))
	}

	if len(parts) == 0 {
		return Dim("no PRs")
	}

	var result strings.Builder
	for i, part := range parts {
		if i > 0 {
			result.WriteString("  ")
		}
		result.WriteString(part)
	}
	return result.String()
}

// FormatChangeStatus formats the status for a change in the stack
func FormatChangeStatus(change stack.Change) string {
	if change.PR == nil {
		return FormatStatus("local")
	}
	return FormatStatus(change.PR.State)
}

// FormatChangeStatusCompact formats the status for a change in compact form
func FormatChangeStatusCompact(change stack.Change) string {
	if change.PR == nil {
		return FormatStatusCompact("local")
	}
	return FormatStatusCompact(change.PR.State)
}

// CountPRsByState counts PRs by their state
func CountPRsByState(changes []stack.Change) (open, draft, merged, closed, local int) {
	for _, change := range changes {
		if change.PR == nil {
			local++
		} else {
			switch change.PR.State {
			case "open":
				open++
			case "draft":
				draft++
			case "merged":
				merged++
			case "closed":
				closed++
			}
		}
	}
	return
}
