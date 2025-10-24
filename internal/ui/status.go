package ui

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// Status icons use Unicode geometric shapes for cross-platform compatibility.
//
// Icon choices and Unicode codepoints:
//   - IconOpen:   ● (U+25CF BLACK CIRCLE) - Solid circle indicates active/open state
//   - IconDraft:  ◐ (U+25D0 CIRCLE WITH LEFT HALF BLACK) - Half-filled shows work in progress
//   - IconMerged: ◆ (U+25C6 BLACK DIAMOND) - Diamond shape indicates completion/merged
//   - IconClosed: ○ (U+25CB WHITE CIRCLE) - Empty circle shows inactive/closed state
//   - IconLocal:  ◯ (U+25EF LARGE CIRCLE) - Larger empty circle for not-yet-pushed state
//   - IconModified: ⟳ (U+27F3) - Clockwise gapped circle arrow for modified
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
const (
	IconOpen     = "●" // U+25CF - Solid circle for open
	IconDraft    = "◐" // U+25D0 - Half circle for draft
	IconMerged   = "◆" // U+25C6 - Diamond for merged
	IconClosed   = "○" // U+25CB - Empty circle for closed
	IconLocal    = "◯" // U+25EF - Large circle for local
	IconModified = "⟳" // U+27F3 - Clockwise gapped circle arrow for modified
)

// Status represents a PR or change status with rendering capabilities
type Status struct {
	Icon  string
	Label string
	State string // "open", "draft", "merged", "closed", "local", "needs-push"
	Style lipgloss.Style
}

// GetStatus returns a Status object for the given state
func GetStatus(state string) Status {
	switch state {
	case "open":
		return Status{
			Icon:  IconOpen,
			Label: "Open",
			State: state,
			Style: StatusOpenStyle,
		}
	case "draft":
		return Status{
			Icon:  IconDraft,
			Label: "Draft",
			State: state,
			Style: StatusDraftStyle,
		}
	case "merged":
		return Status{
			Icon:  IconMerged,
			Label: "Merged",
			State: state,
			Style: StatusMergedStyle,
		}
	case "closed":
		return Status{
			Icon:  IconClosed,
			Label: "Closed",
			State: state,
			Style: StatusClosedStyle,
		}
	case "needs-push":
		return Status{
			Icon:  IconModified,
			Label: "modified", // Display as "modified" not "needs-push"
			State: state,
			Style: StatusModifiedStyle,
		}
	default: // "local" or unknown
		return Status{
			Icon:  IconLocal,
			Label: "Local",
			State: "local",
			Style: StatusLocalStyle,
		}
	}
}

// GetChangeStatus returns a Status for a stack change, using LocalDraftStatus as source of truth.
func GetChangeStatus(change model.Change) Status {
	if change.IsLocal() {
		if change.GetDraftStatus() {
			return GetStatus("draft")
		}
		return GetStatus("local")
	}

	if change.NeedsPush() {
		return GetStatus("needs-push")
	}

	if change.GetDraftStatus() {
		return GetStatus("draft")
	}
	return GetStatus(change.PR.State)
}

// Render returns the full status with icon and label (e.g., "● Open")
func (s Status) Render() string {
	return s.Style.Render(s.Icon + " " + s.Label)
}

// RenderCompact returns just the styled icon
func (s Status) RenderCompact() string {
	return s.Style.Render(s.Icon)
}

// RenderIcon returns the icon without styling
func (s Status) RenderIcon() string {
	return s.Icon
}

// RenderWithCount returns status with count (e.g., "● 3 open")
func (s Status) RenderWithCount(count int) string {
	if count == 0 {
		return ""
	}
	text := fmt.Sprintf("%s %d %s", s.Icon, count, s.Label)
	return s.Style.Render(text)
}

// FormatPRLabel formats a PR number with styling and full URL
func FormatPRLabel(pr *model.PR) string {
	if pr == nil {
		return Dim("-")
	}

	status := GetStatus(pr.State)

	// Display the full URL instead of just the PR number
	if pr.URL != "" {
		return status.Style.Render(pr.URL)
	}

	// Fallback to PR number if URL is not available
	label := fmt.Sprintf("#%d", pr.PRNumber)
	return status.Style.Render(label)
}

// FormatPRLabelCompact formats a PR number in compact form
func FormatPRLabelCompact(pr *model.PR) string {
	if pr == nil {
		return Dim("-")
	}
	return fmt.Sprintf("#%d", pr.PRNumber)
}

// FormatChangeStatus formats the status for a change in the stack.
// Shows local draft/ready preference with sync indicator if GitHub state differs.
func FormatChangeStatus(change model.Change) string {
	if change.IsLocal() {
		if change.GetDraftStatus() {
			return GetStatus("draft").Render()
		}
		return GetStatus("local").Render()
	}

	var localStatus Status
	if change.GetDraftStatus() {
		localStatus = GetStatus("draft")
	} else {
		localStatus = GetStatus("open")
	}

	syncStatus := change.NeedsSyncToGitHub()
	if syncStatus.NeedsSync {
		modifiedIcon := GetStatus("needs-push").RenderCompact()
		return localStatus.Render() + " " + modifiedIcon
	}

	return localStatus.Render()
}

// FormatChangeStatusCompact formats the status for a change in compact form.
func FormatChangeStatusCompact(change model.Change) string{
	if change.IsLocal() {
		if change.GetDraftStatus() {
			return GetStatus("draft").RenderCompact()
		}
		return GetStatus("local").RenderCompact()
	}

	var localStatus Status
	if change.GetDraftStatus() {
		localStatus = GetStatus("draft")
	} else {
		localStatus = GetStatus("open")
	}

	syncStatus := change.NeedsSyncToGitHub()
	if syncStatus.NeedsSync {
		modifiedStatus := GetStatus("needs-push")
		return localStatus.RenderCompact() + modifiedStatus.RenderCompact()
	}

	return localStatus.RenderCompact()
}

// FormatPRSummary formats a summary of PR counts
// e.g., "● 2 open  ◐ 1 draft  ⟳ 1 modified  ◯ 1 local"
func FormatPRSummary(openCount, draftCount, mergedCount, localCount, needsPushCount int) string {
	var parts []string

	if openCount > 0 {
		parts = append(parts, GetStatus("open").RenderWithCount(openCount))
	}
	if draftCount > 0 {
		parts = append(parts, GetStatus("draft").RenderWithCount(draftCount))
	}
	if mergedCount > 0 {
		parts = append(parts, GetStatus("merged").RenderWithCount(mergedCount))
	}
	if needsPushCount > 0 {
		parts = append(parts, GetStatus("needs-push").RenderWithCount(needsPushCount))
	}
	if localCount > 0 {
		parts = append(parts, GetStatus("local").RenderWithCount(localCount))
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

// CountPRsByState counts PRs by their state
// Returns counts for: open, draft, merged, closed, local, and needsPush
func CountPRsByState(changes []model.Change) (open, draft, merged, closed, local, needsPush int) {
	for _, change := range changes {
		if change.IsLocal() {
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

		// Check if this change needs to be pushed
		if change.NeedsPush() {
			needsPush++
		}
	}
	return
}

// Deprecated: Use GetStatus().Render() instead
func FormatStatus(state string) string {
	return GetStatus(state).Render()
}

// Deprecated: Use GetStatus().RenderCompact() instead
func FormatStatusCompact(state string) string {
	return GetStatus(state).RenderCompact()
}

// Deprecated: Use GetStatus().RenderIcon() instead
func GetStatusIcon(state string) string {
	return GetStatus(state).RenderIcon()
}
