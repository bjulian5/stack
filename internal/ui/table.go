package ui

import (
	"fmt"

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

// RenderStackDetailsTable renders a table for stack details (status command)
// This is a convenience function that sets up the table structure for stack changes
func RenderStackDetailsTable(headers []string, rows [][]string) string {
	t := NewStackTable().
		Headers(headers...).
		Rows(rows...)

	return t.String()
}

// RenderSimpleTable renders a simple table without heavy borders
func RenderSimpleTable(headers []string, rows [][]string) string {
	t := NewSimpleTable().
		Headers(headers...).
		Rows(rows...)

	return t.String()
}

// Example usage for backward compatibility with old Table API:
//
// Old code:
//   table := ui.NewTable([]ui.Column{
//       {Header: "#", Width: 3, Align: ui.AlignRight},
//       {Header: "Status", MinWidth: 18, MaxWidth: 20},
//   })
//   table.AddRow("1", "Open")
//   output := table.Render()
//
// New code:
//   t := ui.NewStackTable().
//       Headers("#", "Status", "PR", "Title", "Commit")
//   t.Row("1", "● Open", "#123", "Add auth", "a1b2c3d")
//   output := t.String()
//
// Or using the convenience function:
//   output := ui.RenderStackDetailsTable(
//       []string{"#", "Status", "PR", "Title", "Commit"},
//       [][]string{
//           {"1", "● Open", "#123", "Add auth", "a1b2c3d"},
//           {"2", "◐ Draft", "#124", "Fix bug", "b2c3d4e"},
//       },
//   )

// Deprecated types and functions for backward compatibility
// These will be removed in a future version

// Column defines a table column (deprecated - use lipgloss/table directly)
type Column struct {
	Header   string
	Width    int
	MinWidth int
	MaxWidth int
	Align    ColumnAlign
}

// ColumnAlign defines column alignment (deprecated)
type ColumnAlign int

const (
	AlignLeft ColumnAlign = iota
	AlignRight
	AlignCenter
)

// Table represents a styled table (deprecated - use lipgloss/table directly)
type Table struct {
	inner   *table.Table
	columns []Column
	rows    [][]string
}

// NewTable creates a new table (deprecated)
func NewTable(columns []Column) *Table {
	return &Table{
		inner:   NewStackTable(),
		columns: columns,
		rows:    [][]string{},
	}
}

// AddRow adds a row to the table (deprecated)
func (t *Table) AddRow(cells ...string) error {
	if len(cells) != len(t.columns) {
		return fmt.Errorf("row has %d cells but table has %d columns", len(cells), len(t.columns))
	}
	t.rows = append(t.rows, cells)
	return nil
}

// Render renders the table (deprecated)
func (t *Table) Render() string {
	// Extract headers from columns
	headers := make([]string, len(t.columns))
	for i, col := range t.columns {
		headers[i] = col.Header
	}

	return RenderStackDetailsTable(headers, t.rows)
}

// RenderSimple renders a simple table (deprecated)
func (t *Table) RenderSimple() string {
	// Extract headers from columns
	headers := make([]string, len(t.columns))
	for i, col := range t.columns {
		headers[i] = col.Header
	}

	return RenderSimpleTable(headers, t.rows)
}

// RenderCompact renders a compact table (deprecated - use RenderSimple)
func (t *Table) RenderCompact() string {
	return t.RenderSimple()
}
