package switchcmd

import (
	"context"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command switches between stacks
type Command struct {
	// Arguments
	StackName string

	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
	GH    *gh.Client
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.GH = gh.NewClient()
	c.Stack = stack.NewClient(c.Git, c.GH)

	cmd := &cobra.Command{
		Use:   "switch [stack-name]",
		Short: "Switch to a different stack",
		Long: `Switch to a different stack. If no stack name is provided, opens an interactive fuzzy finder.

After switching, displays the full stack details.

Example:
  stack switch                  # Interactive fuzzy finder
  stack switch auth-refactor    # Direct switch`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Check for uncommitted changes before switching
	hasUncommitted, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if hasUncommitted {
		return fmt.Errorf("uncommitted changes detected: commit or stash your changes before switching stacks")
	}

	// Get all stacks
	stacks, err := c.Stack.ListStacks()
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	if len(stacks) == 0 {
		ui.Print(ui.RenderNoStacksMessage())
		return nil
	}

	// Get current stack for validation
	currentStackCtx, _ := c.Stack.GetStackContext()
	currentStackName := ""
	if currentStackCtx != nil && currentStackCtx.IsStack() {
		currentStackName = currentStackCtx.StackName
	}

	var selectedStack *model.Stack

	if c.StackName != "" {
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

	// Check if already on this stack
	if currentStackName == selectedStack.Name {
		ui.Infof("Already on stack: %s", ui.Bold(selectedStack.Name))
		ui.Print("")

		// Still show the stack details (we're already on this stack)
		stackCtx, err := c.Stack.GetStackContext()
		if err != nil {
			return fmt.Errorf("failed to get stack context: %w", err)
		}

		// Sync latest metadata from GitHub
		stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
		if err != nil {
			// Log warning but continue with cached data
			ui.Warningf("Failed to refresh stack: %v", err)
		}

		// Get current position (we're on this stack, so arrow will show)
		currentUUID := stackCtx.GetCurrentPositionUUID()

		ui.Print(ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges, currentUUID))
		return nil
	}

	// Switch to the stack
	if err := c.Stack.SwitchStack(selectedStack.Name); err != nil {
		return fmt.Errorf("failed to switch to stack: %w", err)
	}

	// Show success message
	ui.Successf("Switched to stack: %s", ui.Bold(selectedStack.Name))
	ui.Print("")

	// Load and display full stack details (we just switched, so we're on this stack)
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return fmt.Errorf("failed to get stack context: %w", err)
	}

	// Sync latest metadata from GitHub
	stackCtx, err = c.Stack.RefreshStackMetadata(stackCtx)
	if err != nil {
		// Log warning but continue with cached data
		ui.Warningf("Failed to refresh stack: %v", err)
	}

	// Get current position (we're on TOP branch, arrow will show at last change)
	currentUUID := stackCtx.GetCurrentPositionUUID()

	ui.Print(ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges, currentUUID))

	return nil
}
