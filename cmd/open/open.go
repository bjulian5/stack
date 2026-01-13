package open

import (
	"context"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command opens PRs for a stack in the browser
type Command struct {
	// Arguments
	StackName string

	// Flags
	All bool

	// Clients
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use:   "open [stack-name]",
		Short: "Open PRs for a stack in the browser",
		Long: `Open PRs for a stack in the browser.

If no stack name is provided, opens an interactive fuzzy finder to select a stack.
By default, opens the top PR of the stack. Use --all to open all PRs.

Examples:
  stack open                  # Interactive fuzzy finder, opens top PR
  stack open auth-refactor    # Direct open top PR of auth-refactor
  stack open --all            # Interactive select, open all PRs`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cobraCmd.Context())
		},
	}

	command.Flags().BoolVarP(&c.All, "all", "a", false, "Open all PRs in the stack")

	parent.AddCommand(command)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Get all stacks
	stacks, err := c.Stack.ListStacks()
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	if len(stacks) == 0 {
		ui.Print(ui.RenderNoStacksMessage())
		return nil
	}

	// Get current stack for highlighting
	currentStackCtx, _ := c.Stack.GetStackContext()
	currentStackName := ""
	if currentStackCtx != nil && currentStackCtx.IsStack() {
		currentStackName = currentStackCtx.StackName
	}

	var selectedStack *model.Stack

	if c.StackName != "" {
		// Direct stack name provided
		found := false
		for _, s := range stacks {
			if s.Name == c.StackName {
				selectedStack = s
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("stack '%s' not found", c.StackName)
		}
	} else {
		// Interactive fuzzy finder
		stackChangesMap := make(map[string][]*model.Change)
		for _, s := range stacks {
			ctx, err := c.Stack.GetStackContextByName(s.Name)
			if err != nil {
				ui.Warningf("Failed to load stack %s: %v", s.Name, err)
				continue
			}
			stackChangesMap[s.Name] = ctx.AllChanges
		}

		idx, err := fuzzyfinder.Find(
			stacks,
			func(i int) string {
				s := stacks[i]
				changes := stackChangesMap[s.Name]
				return ui.FormatStackFinderLine(s.Name, s.Base, changes, currentStackName)
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				s := stacks[i]
				changes := stackChangesMap[s.Name]
				return ui.FormatStackPreview(s.Name, s.Branch, s.Base, changes)
			}),
		)

		if err != nil {
			// User cancelled
			return nil
		}

		selectedStack = stacks[idx]
	}

	// Load the stack's context to get PRs
	stackCtx, err := c.Stack.GetStackContextByName(selectedStack.Name)
	if err != nil {
		return fmt.Errorf("failed to load stack: %w", err)
	}

	// Find PRs to open
	var prsToOpen []*model.Change
	for _, change := range stackCtx.AllChanges {
		if !change.IsLocal() && change.PR != nil && change.PR.PRNumber > 0 {
			prsToOpen = append(prsToOpen, change)
		}
	}

	if len(prsToOpen) == 0 {
		return fmt.Errorf("no PRs in stack '%s': use 'stack push' to create PRs", selectedStack.Name)
	}

	if c.All {
		// Open all PRs
		for _, change := range prsToOpen {
			if err := c.GH.OpenPR(change.PR.PRNumber); err != nil {
				ui.Warningf("Failed to open PR #%d: %v", change.PR.PRNumber, err)
			} else {
				ui.Successf("Opening PR #%d: %s", change.PR.PRNumber, change.Title)
			}
		}
		ui.Successf("Opened %d PR(s) for stack '%s'", len(prsToOpen), selectedStack.Name)
	} else {
		// Open just the top PR (last in list = top of stack)
		topPR := prsToOpen[len(prsToOpen)-1]
		if err := c.GH.OpenPR(topPR.PR.PRNumber); err != nil {
			return fmt.Errorf("failed to open PR in browser: %w", err)
		}
		ui.Successf("Opening PR #%d: %s", topPR.PR.PRNumber, topPR.Title)
	}

	return nil
}
