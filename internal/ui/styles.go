package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#6366F1") // Indigo

	// Status colors
	ColorSuccess = lipgloss.Color("#10B981") // Green
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorInfo    = lipgloss.Color("#3B82F6") // Blue

	// State colors
	ColorOpen   = lipgloss.Color("#10B981") // Green
	ColorDraft  = lipgloss.Color("#F59E0B") // Amber
	ColorMerged = lipgloss.Color("#8B5CF6") // Purple
	ColorClosed = lipgloss.Color("#6B7280") // Gray
	ColorLocal  = lipgloss.Color("#9CA3AF") // Light gray

	// Text colors
	ColorText       = lipgloss.Color("#F3F4F6") // Light gray
	ColorTextMuted  = lipgloss.Color("#9CA3AF") // Gray
	ColorTextBright = lipgloss.Color("#FFFFFF") // White

	// Background colors
	ColorBgSubtle = lipgloss.Color("#1F2937") // Dark gray
	ColorBgMuted  = lipgloss.Color("#111827") // Darker gray

	// Border colors
	ColorBorder       = lipgloss.Color("#374151") // Medium gray
	ColorBorderBright = lipgloss.Color("#4B5563") // Lighter gray
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

// GetStatusStyle returns the appropriate style for a PR state
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
	default:
		return StatusLocalStyle
	}
}

// GetStatusColor returns the color for a PR state
func GetStatusColor(state string) lipgloss.Color {
	switch state {
	case "open":
		return ColorOpen
	case "draft":
		return ColorDraft
	case "merged":
		return ColorMerged
	case "closed":
		return ColorClosed
	default:
		return ColorLocal
	}
}
