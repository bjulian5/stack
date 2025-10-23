package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// NewStackTable creates a new table with Stack-specific styling defaults
// This is a thin wrapper around lipgloss/table with opinionated defaults
func NewStackTable() *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(TableBorderStyle).
		BorderRow(true).
		BorderColumn(true).
		StyleFunc(defaultTableStyleFunc)
}

// NewSimpleTable creates a table without borders (like the old RenderSimple)
func NewSimpleTable() *table.Table {
	return table.New().
		Border(lipgloss.Border{}).
		StyleFunc(simpleTableStyleFunc)
}

// defaultTableStyleFunc provides default styling for table cells
func defaultTableStyleFunc(row, col int) lipgloss.Style {
	switch {
	case row == table.HeaderRow:
		return TableHeaderStyle
	case row%2 == 0:
		return TableCellStyle
	default:
		return TableRowAltStyle
	}
}

// simpleTableStyleFunc provides styling for simple tables (no alternating rows)
func simpleTableStyleFunc(row, col int) lipgloss.Style {
	if row == table.HeaderRow {
		return TableHeaderStyle
	}
	return TableCellStyle
}
