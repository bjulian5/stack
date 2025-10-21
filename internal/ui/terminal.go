package ui

import (
	"os"

	"golang.org/x/term"
)

// GetTerminalWidth returns the current terminal width in columns.
// If the terminal width cannot be determined (non-TTY or error),
// returns a sensible default of 120 columns.
func GetTerminalWidth() int {
	// Try to get the terminal file descriptor
	fd := int(os.Stdout.Fd())

	// Check if stdout is a terminal
	if !term.IsTerminal(fd) {
		return 120 // Default for non-TTY (pipes, redirects, etc.)
	}

	// Get terminal size
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 120 // Default on error
	}

	return width
}
