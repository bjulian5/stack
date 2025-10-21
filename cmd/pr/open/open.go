package open

import (
	"context"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command opens a PR in the browser
type Command struct {
	// Arguments
	Position string // Optional: "top" to open top PR

	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	c.GH = gh.NewClient()

	cmd := &cobra.Command{
		Use:   "open [top]",
		Short: "Open a PR in the browser",
		Long: `Open a pull request in the browser using a fuzzy finder.

If "top" is provided, opens the top PR in the stack.
Otherwise, displays a fuzzy finder to select which PR to open.

Example:
  stack pr open       # Select PR interactively
  stack pr open top   # Open the top PR`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if args[0] != "top" {
					return fmt.Errorf("invalid argument %q: use 'top' or no argument", args[0])
				}
				c.Position = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get current stack context
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Validate we're in a stack
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch: switch to a stack first or use 'stack switch'")
	}

	// Filter changes to only those with PRs
	var prsOnly []stack.Change
	for _, change := range stackCtx.Changes {
		if change.PR != nil {
			prsOnly = append(prsOnly, change)
		}
	}

	// Validate stack has PRs
	if len(prsOnly) == 0 {
		return fmt.Errorf("no PRs in this stack: use 'stack push' to create PRs")
	}

	// Determine which PR to open
	var selectedChange *stack.Change

	if c.Position == "top" {
		// Open the top PR (last in the list)
		selectedChange = &prsOnly[len(prsOnly)-1]
	} else {
		// Use fuzzy finder to select a PR
		idx, err := fuzzyfinder.Find(
			prsOnly,
			func(i int) string {
				return ui.FormatChangeFinderLine(prsOnly[i])
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return ui.FormatChangePreview(prsOnly[i])
			}),
		)

		if err != nil {
			// User cancelled
			return nil
		}

		selectedChange = &prsOnly[idx]
	}

	// Open PR in browser using gh
	if err := c.GH.OpenPR(selectedChange.PR.PRNumber); err != nil {
		return fmt.Errorf("failed to open PR in browser: %w (ensure 'gh' CLI is installed)", err)
	}

	// Print success message
	fmt.Println(ui.RenderSuccessMessage(fmt.Sprintf("Opening PR #%d: %s", selectedChange.PR.PRNumber, selectedChange.Title)))

	return nil
}
