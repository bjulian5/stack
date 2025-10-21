package ui

import (
	"github.com/ktr0731/go-fuzzyfinder"

	"github.com/bjulian5/stack/internal/stack"
)

// SelectChange presents a fuzzy finder to select a change from the stack.
// Returns the selected change, or nil if the user cancelled the selection.
// Returns an error only if the fuzzy finder encounters an unexpected error.
func SelectChange(changes []stack.Change) (*stack.Change, error) {
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

	return &changes[idx], nil
}
