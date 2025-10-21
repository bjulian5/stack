package switchcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
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
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.Stack = stack.NewClient(c.Git)

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
		fmt.Println(ui.RenderNoStacksMessage())
		return nil
	}

	// Get current stack for validation
	currentStackCtx, _ := c.Stack.GetStackContext()
	currentStackName := ""
	if currentStackCtx != nil && currentStackCtx.IsStack() {
		currentStackName = currentStackCtx.StackName
	}

	var selectedStack *stack.Stack

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
		stackChangesMap := make(map[string][]stack.Change)
		for _, s := range stacks {
			ctx, err := c.Stack.GetStackContextByName(s.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load stack %s: %v\n", s.Name, err)
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
		fmt.Println(ui.RenderInfoMessage(fmt.Sprintf("Already on stack: %s", ui.Bold(selectedStack.Name))))
		fmt.Println()

		// Still show the stack details
		stackCtx, err := c.Stack.GetStackContextByName(selectedStack.Name)
		if err != nil {
			return fmt.Errorf("failed to load stack details: %w", err)
		}
		fmt.Println(ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges))
		return nil
	}

	// Switch to the stack
	if err := c.Stack.SwitchStack(selectedStack.Name); err != nil {
		return fmt.Errorf("failed to switch to stack: %w", err)
	}

	// Show success message
	fmt.Println(ui.RenderSwitchSuccess(selectedStack.Name))
	fmt.Println()

	// Load and display full stack details
	stackCtx, err := c.Stack.GetStackContextByName(selectedStack.Name)
	if err != nil {
		return fmt.Errorf("failed to load stack details: %w", err)
	}

	fmt.Println(ui.RenderStackDetails(stackCtx.Stack, stackCtx.AllChanges))

	return nil
}
