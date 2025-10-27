package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/ktr0731/go-fuzzyfinder"

	"github.com/bjulian5/stack/internal/model"
)

func init() {
	// Force lipgloss to initialize and detect terminal before fuzzy finder starts
	// This prevents ANSI escape sequences from leaking into the finder input
	_ = lipgloss.NewStyle().Render("")
	// Ensure color profile is detected early
	_ = lipgloss.HasDarkBackground()
}

// SelectChange presents a fuzzy finder to select a change from the stack.
// Returns the selected change, or nil if the user cancelled the selection.
// Returns an error only if the fuzzy finder encounters an unexpected error.
func SelectChange(changes []*model.Change) (*model.Change, error) {
	// Flush stdout/stderr before starting fuzzy finder to clear any ANSI sequences
	os.Stdout.Sync()
	os.Stderr.Sync()

	idx, err := fuzzyfinder.Find(
		changes,
		func(i int) string {
			return FormatChangeFinderLine(changes[i])
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return FormatChangePreview(changes[i])
		}),
	)

	if err != nil {
		// User cancelled (Ctrl+C or ESC)
		return nil, nil
	}

	return changes[idx], nil
}
