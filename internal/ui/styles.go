package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette with adaptive colors for light/dark backgrounds
var (
	// Primary colors with adaptive support
	ColorPrimary = lipgloss.AdaptiveColor{
		Light: "#5B21B6", // Darker purple for light backgrounds
		Dark:  "#7C3AED", // Current purple for dark backgrounds
	}
	ColorSecondary = lipgloss.AdaptiveColor{
		Light: "#4338CA", // Darker indigo
		Dark:  "#6366F1", // Current indigo
	}

	// Status colors (semantic, context-aware)
	ColorSuccess = lipgloss.AdaptiveColor{
		Light: "#059669", // Darker green
		Dark:  "#10B981", // Current green
	}
	ColorWarning = lipgloss.AdaptiveColor{
		Light: "#D97706", // Darker amber
		Dark:  "#F59E0B", // Current amber
	}
	ColorError = lipgloss.AdaptiveColor{
		Light: "#DC2626", // Darker red
		Dark:  "#EF4444", // Current red
	}
	ColorInfo = lipgloss.AdaptiveColor{
		Light: "#2563EB", // Darker blue
		Dark:  "#3B82F6", // Current blue
	}

	// State colors (more distinct palette)
	ColorOpen = lipgloss.AdaptiveColor{
		Light: "#16A34A", // Brighter green for light bg
		Dark:  "#22C55E", // Brighter green (was #10B981)
	}
	ColorDraft = lipgloss.AdaptiveColor{
		Light: "#EA580C", // Darker orange
		Dark:  "#F97316", // Bright orange (was amber #F59E0B)
	}
	ColorMerged = lipgloss.AdaptiveColor{
		Light: "#7C3AED", // Purple
		Dark:  "#A78BFA", // Light purple (better contrast)
	}
	ColorClosed = lipgloss.AdaptiveColor{
		Light: "#475569", // Darker slate
		Dark:  "#64748B", // Slate (was gray #6B7280)
	}
	ColorLocal = lipgloss.AdaptiveColor{
		Light: "#64748B", // Slate
		Dark:  "#94A3B8", // Light slate (was #9CA3AF)
	}
	ColorModified = lipgloss.AdaptiveColor{
		Light: "#CA8A04", // Darker gold
		Dark:  "#FBBF24", // Gold (was amber, now distinct from draft)
	}

	// Text colors with adaptive support
	ColorText = lipgloss.AdaptiveColor{
		Light: "#1F2937", // Dark gray for light backgrounds
		Dark:  "#F3F4F6", // Light gray for dark backgrounds
	}
	ColorTextMuted = lipgloss.AdaptiveColor{
		Light: "#6B7280", // Medium gray
		Dark:  "#9CA3AF", // Light gray
	}
	ColorTextBright = lipgloss.AdaptiveColor{
		Light: "#000000", // Black for light backgrounds
		Dark:  "#FFFFFF", // White for dark backgrounds
	}

	// Tree colors (for hierarchical visualization)
	ColorTreeBranch = lipgloss.AdaptiveColor{
		Light: "#9CA3AF", // Light gray
		Dark:  "#6B7280", // Gray
	}
	ColorTreeCurrent = lipgloss.AdaptiveColor{
		Light: "#2563EB", // Blue
		Dark:  "#3B82F6", // Lighter blue
	}
	ColorTreeEnumerator = lipgloss.AdaptiveColor{
		Light: "#6B7280", // Gray
		Dark:  "#9CA3AF", // Light gray
	}

	// Background colors
	ColorBgSubtle = lipgloss.AdaptiveColor{
		Light: "#F3F4F6", // Light gray
		Dark:  "#1F2937", // Dark gray
	}
	ColorBgMuted = lipgloss.AdaptiveColor{
		Light: "#E5E7EB", // Lighter gray
		Dark:  "#111827", // Darker gray
	}

	// Border colors
	ColorBorder = lipgloss.AdaptiveColor{
		Light: "#D1D5DB", // Light gray border
		Dark:  "#374151", // Medium gray border
	}
	ColorBorderBright = lipgloss.AdaptiveColor{
		Light: "#9CA3AF", // Medium gray
		Dark:  "#4B5563", // Lighter gray
	}
)

// Border styles
var (
	BorderRounded = lipgloss.RoundedBorder()
	BorderNormal  = lipgloss.NormalBorder()
	BorderThick   = lipgloss.ThickBorder()
	BorderDouble  = lipgloss.DoubleBorder()
)

// Base styles
var (
	// Box style with rounded border
	BoxStyle = lipgloss.NewStyle().
			Border(BorderRounded).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	// Panel style for content sections
	PanelStyle = lipgloss.NewStyle().
			Border(BorderRounded).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTextBright).
			Background(ColorPrimary).
			Padding(0, 2).
			MarginBottom(1)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)
)

// Text styles
var (
	BoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTextBright)

	DimStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)
)

// Status styles for PR states
var (
	StatusOpenStyle = lipgloss.NewStyle().
			Foreground(ColorOpen).
			Bold(true)

	StatusDraftStyle = lipgloss.NewStyle().
				Foreground(ColorDraft).
				Bold(true)

	StatusMergedStyle = lipgloss.NewStyle().
				Foreground(ColorMerged).
				Bold(true)

	StatusClosedStyle = lipgloss.NewStyle().
				Foreground(ColorClosed)

	StatusLocalStyle = lipgloss.NewStyle().
				Foreground(ColorLocal)

	StatusModifiedStyle = lipgloss.NewStyle().
				Foreground(ColorModified).
				Bold(true)
)

// Message styles
var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)
)

// Table styles
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorTextBright)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableRowAltStyle = lipgloss.NewStyle().
				Background(ColorBgMuted).
				Padding(0, 1)

	TableBorderStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)
)

// Tree styles (for hierarchical stack visualization)
var (
	TreeRootStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	TreeItemStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TreeEnumeratorStyle = lipgloss.NewStyle().
				Foreground(ColorTreeEnumerator)

	TreeCurrentMarkerStyle = lipgloss.NewStyle().
				Foreground(ColorTreeCurrent).
				Bold(true)

	TreeBranchStyle = lipgloss.NewStyle().
			Foreground(ColorTreeBranch)
)

// GetStatusStyle returns the appropriate style for a PR state
// Deprecated: Use GetStatus(state).Style instead
func GetStatusStyle(state string) lipgloss.Style {
	switch state {
	case "open":
		return StatusOpenStyle
	case "draft":
		return StatusDraftStyle
	case "merged":
		return StatusMergedStyle
	case "closed":
		return StatusClosedStyle
	case "needs-push":
		return StatusModifiedStyle
	default:
		return StatusLocalStyle
	}
}
