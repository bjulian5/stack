package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ColumnAlign defines column alignment
type ColumnAlign int

const (
	AlignLeft ColumnAlign = iota
	AlignRight
	AlignCenter
)

// Table rendering constants
const (
	tableCellPadding   = 3
	tableBorderPadding = 4
)

// Column defines a table column
type Column struct {
	Header   string
	Width    int // If 0, column is flexible and auto-sizes
	MinWidth int // Minimum width for flexible columns (ignored if Width > 0)
	MaxWidth int // Maximum width for flexible columns (ignored if Width > 0)
	Align    ColumnAlign
}

// Table represents a styled table
type Table struct {
	Columns    []Column
	Rows       [][]string
	ShowHeader bool
}

// NewTable creates a new table
func NewTable(columns []Column) *Table {
	return &Table{
		Columns:    columns,
		Rows:       [][]string{},
		ShowHeader: true,
	}
}

// AddRow adds a row to the table. Returns an error if the number of cells
// doesn't match the number of columns. This helps catch bugs early rather than
// silently padding or truncating rows.
func (t *Table) AddRow(cells ...string) error {
	if len(cells) != len(t.Columns) {
		return fmt.Errorf("row has %d cells but table has %d columns", len(cells), len(t.Columns))
	}
	t.Rows = append(t.Rows, cells)
	return nil
}

// calculateColumnWidths calculates actual column widths based on content and terminal size
func (t *Table) calculateColumnWidths() []int {
	widths := make([]int, len(t.Columns))
	terminalWidth := GetTerminalWidth()

	// Step 1: Calculate fixed column widths and identify flexible columns
	var fixedWidth int
	var flexibleColumns []int

	for i, col := range t.Columns {
		if col.Width > 0 {
			// Fixed width column
			widths[i] = col.Width
			fixedWidth += col.Width
		} else {
			// Flexible column - start with MinWidth
			widths[i] = col.MinWidth
			if widths[i] == 0 {
				widths[i] = 10 // Minimum fallback
			}
			flexibleColumns = append(flexibleColumns, i)
		}
	}

	// Account for borders and padding: "│ col │ col │ col │"
	// Each column has: " col " (2 spaces) + "│" separator
	// Total overhead: (numColumns + 1) separators + (numColumns * 2) padding
	overhead := (len(t.Columns) + 1) + (len(t.Columns) * 2)

	// Step 2: Calculate available space for flexible columns
	availableSpace := terminalWidth - fixedWidth - overhead
	if availableSpace < 0 {
		availableSpace = 0
	}

	// Step 3: Distribute space among flexible columns
	if len(flexibleColumns) > 0 && availableSpace > 0 {
		// Calculate current flexible width total
		currentFlexWidth := 0
		for _, idx := range flexibleColumns {
			currentFlexWidth += widths[idx]
		}

		// Calculate how much extra space we have
		extraSpace := availableSpace - currentFlexWidth
		if extraSpace > 0 {
			// Distribute extra space proportionally, respecting MaxWidth
			spacePerColumn := extraSpace / len(flexibleColumns)

			for _, idx := range flexibleColumns {
				col := t.Columns[idx]
				newWidth := widths[idx] + spacePerColumn

				// Apply MaxWidth constraint
				maxWidth := col.MaxWidth
				if maxWidth == 0 {
					maxWidth = 200 // Reasonable default maximum
				}
				if newWidth > maxWidth {
					newWidth = maxWidth
				}

				widths[idx] = newWidth
			}
		}
	}

	return widths
}

// visibleWidth returns the visible width of a string, accounting for ANSI codes
func visibleWidth(s string) int {
	return lipgloss.Width(s)
}

// alignCell aligns a cell's content based on column alignment
func (t *Table) alignCell(content string, width int, align ColumnAlign) string {
	plainLen := visibleWidth(content)
	if plainLen >= width {
		return content
	}

	padding := width - plainLen
	switch align {
	case AlignLeft:
		return content + strings.Repeat(" ", padding)
	case AlignRight:
		return strings.Repeat(" ", padding) + content
	case AlignCenter:
		leftPad := padding / 2
		rightPad := padding - leftPad
		return strings.Repeat(" ", leftPad) + content + strings.Repeat(" ", rightPad)
	default:
		return content
	}
}

// Render renders the table with styled borders
func (t *Table) Render() string {
	if len(t.Columns) == 0 {
		return ""
	}

	widths := t.calculateColumnWidths()
	var lines []string

	totalWidth := 0
	for i, w := range widths {
		totalWidth += w
		if i < len(widths)-1 {
			totalWidth += tableCellPadding
		}
	}
	totalWidth += tableBorderPadding

	topBorder := TableBorderStyle.Render("╭" + strings.Repeat("─", totalWidth-2) + "╮")
	lines = append(lines, topBorder)

	if t.ShowHeader {
		headerParts := []string{}
		for i, col := range t.Columns {
			aligned := t.alignCell(col.Header, widths[i], col.Align)
			styled := TableHeaderStyle.Render(aligned)
			headerParts = append(headerParts, styled)
		}
		sep := TableBorderStyle.Render("│")
		headerLine := sep + " " + strings.Join(headerParts, " "+sep+" ") + " " + sep
		lines = append(lines, headerLine)

		// Format matches header: "│ col1 │ col2 │ col3 │"
		// Separator should be:    "├──────┼──────┼──────┤"
		// Each section is: space + column_width + space = width + 2
		sepParts := []string{}
		for _, w := range widths {
			sepParts = append(sepParts, strings.Repeat("─", w+2))
		}
		headerSep := TableBorderStyle.Render("├" + strings.Join(sepParts, "┼") + "┤")
		lines = append(lines, headerSep)
	}

	sep := TableBorderStyle.Render("│")
	for _, row := range t.Rows {
		rowParts := []string{}
		for colIdx, cell := range row {
			if colIdx < len(t.Columns) {
				aligned := t.alignCell(cell, widths[colIdx], t.Columns[colIdx].Align)
				rowParts = append(rowParts, aligned)
			}
		}
		rowLine := sep + " " + strings.Join(rowParts, " "+sep+" ") + " " + sep
		lines = append(lines, rowLine)
	}

	bottomBorder := TableBorderStyle.Render("╰" + strings.Repeat("─", totalWidth-2) + "╯")
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// RenderSimple renders a simple table without heavy borders
func (t *Table) RenderSimple() string {
	if len(t.Columns) == 0 {
		return ""
	}

	widths := t.calculateColumnWidths()
	var lines []string

	if t.ShowHeader {
		headerParts := []string{}
		for i, col := range t.Columns {
			aligned := t.alignCell(col.Header, widths[i], col.Align)
			styled := BoldStyle.Render(aligned)
			headerParts = append(headerParts, styled)
		}
		headerLine := strings.Join(headerParts, "  ")
		lines = append(lines, headerLine)

		sepParts := []string{}
		for _, w := range widths {
			sepParts = append(sepParts, strings.Repeat("─", w))
		}
		separator := DimStyle.Render(strings.Join(sepParts, "  "))
		lines = append(lines, separator)
	}

	for _, row := range t.Rows {
		rowParts := []string{}
		for colIdx, cell := range row {
			if colIdx < len(t.Columns) {
				aligned := t.alignCell(cell, widths[colIdx], t.Columns[colIdx].Align)
				rowParts = append(rowParts, aligned)
			}
		}
		rowLine := strings.Join(rowParts, "  ")
		lines = append(lines, rowLine)
	}

	return strings.Join(lines, "\n")
}

// RenderCompact renders a compact table with minimal spacing
func (t *Table) RenderCompact() string {
	if len(t.Columns) == 0 {
		return ""
	}

	widths := t.calculateColumnWidths()
	var lines []string

	if t.ShowHeader {
		headerParts := []string{}
		for i, col := range t.Columns {
			aligned := t.alignCell(col.Header, widths[i], col.Align)
			styled := BoldStyle.Render(aligned)
			headerParts = append(headerParts, styled)
		}
		headerLine := strings.Join(headerParts, " ")
		lines = append(lines, headerLine)
	}

	for _, row := range t.Rows {
		rowParts := []string{}
		for colIdx, cell := range row {
			if colIdx < len(t.Columns) {
				aligned := t.alignCell(cell, widths[colIdx], t.Columns[colIdx].Align)
				rowParts = append(rowParts, aligned)
			}
		}
		rowLine := strings.Join(rowParts, " ")
		lines = append(lines, rowLine)
	}

	return strings.Join(lines, "\n")
}

// RenderBorderedContent renders content with a border
func RenderBorderedContent(content string, title string) string {
	style := lipgloss.NewStyle().
		Border(BorderRounded).
		BorderForeground(ColorBorder).
		Padding(1, 2)

	if title != "" {
		titleBar := lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorTextBright).
			Bold(true).
			Padding(0, 2).
			Render(title)

		return titleBar + "\n" + style.Render(content)
	}

	return style.Render(content)
}
