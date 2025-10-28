package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bjulian5/stack/internal/model"
)

// Status icons
const (
	IconOpen     = "●"
	IconDraft    = "◐"
	IconMerged   = "◆"
	IconClosed   = "○"
	IconLocal    = "◯"
	IconModified = "◎"
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
			Label: "needs push",
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
func GetChangeStatus(change *model.Change) Status {
	if change == nil {
		return GetStatus("local")
	}
	if change.IsLocal() {
		if change.GetDraftStatus() {
			return GetStatus("draft")
		}
		return GetStatus("local")
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

// FormatPRSummary formats a summary of PR counts
// e.g., "● 2 open  ◐ 1 draft  ◎ 1 needs push  ◯ 1 local"
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
func CountPRsByState(changes []*model.Change) (open, draft, merged, closed, local, needsPush int) {
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

		// Check if this change needs to be synced to GitHub
		if change.NeedsSyncToGitHub().NeedsSync && !change.PR.IsMerged() {
			needsPush++
		}
	}
	return
}
