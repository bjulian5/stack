package ui

// DisplayConfig holds configuration for UI rendering
type DisplayConfig struct {
	// View settings
	DefaultView ViewMode

	// Truncation limits
	MaxStackNameLength     int
	MaxTitleLength         int
	MaxTitleLengthDetailed int
	MaxPreviewLines        int
	MaxDescriptionLines    int
	MaxStacksInPreview     int

	// Display lengths
	CommitHashDisplayLength int
	DefaultTerminalWidth    int

	// Tree settings
	UseTreeByDefault bool
	TreeIndent       string
	TreeEnumerator   TreeEnumStyle

	// Spacing
	DefaultPadding int
	DefaultMargin  int
	PanelPadding   int
	BoxPadding     int
}

// ViewMode defines how stacks are displayed
type ViewMode int

const (
	ViewTree    ViewMode = iota // Tree visualization (default)
	ViewTable                   // Traditional table view
	ViewCompact                 // One-liner per change
)

// TreeEnumStyle defines tree line styles
type TreeEnumStyle int

const (
	TreeRounded TreeEnumStyle = iota // ╰─ style
	TreeDefault                      // └─ style
)

// DefaultConfig returns the default display configuration
func DefaultConfig() DisplayConfig {
	return DisplayConfig{
		// View settings
		DefaultView: ViewTree,

		// Truncation limits
		MaxStackNameLength:     20,
		MaxTitleLength:         40,
		MaxTitleLengthDetailed: 50,
		MaxPreviewLines:        5,
		MaxDescriptionLines:    10,
		MaxStacksInPreview:     5,

		// Display lengths
		CommitHashDisplayLength: 7,
		DefaultTerminalWidth:    120,

		// Tree settings
		UseTreeByDefault: true,
		TreeIndent:       "  ",
		TreeEnumerator:   TreeRounded,

		// Spacing
		DefaultPadding: 1,
		DefaultMargin:  0,
		PanelPadding:   1,
		BoxPadding:     1,
	}
}

// Global display configuration (can be overridden)
var Display = DefaultConfig()

// SetDisplayConfig updates the global display configuration
func SetDisplayConfig(c DisplayConfig) {
	Display = c
}

// GetDisplayConfig returns the current display configuration
func GetDisplayConfig() DisplayConfig {
	return Display
}
