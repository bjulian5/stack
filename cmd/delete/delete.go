package delete

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
	StackName string
	Force     bool
	Git       *git.Client
	Stack     *stack.Client
	GH        *gh.Client
}

func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.GH = gh.NewClient()
	c.Stack = stack.NewClient(c.Git, c.GH)

	cmd := &cobra.Command{
		Use:   "delete [stack-name]",
		Short: "Delete a stack and its branches",
		Long: `Delete a stack by archiving its metadata and deleting all associated branches.

The stack metadata is moved to .git/stack/.archived/<name>-<timestamp> for potential recovery.
All local and remote branches associated with the stack will be deleted.

If no stack name is provided, deletes the current stack (if on a stack branch).

Example:
  stack delete                  # Delete current stack
  stack delete auth-refactor    # Delete specific stack
  stack delete --force          # Skip confirmation prompt`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&c.Force, "force", "f", false, "Skip confirmation prompt")
	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	stackName, err := c.resolveStackName()
	if err != nil {
		return err
	}

	stackCtx, err := c.Stack.GetStackContextByName(stackName)
	if err != nil {
		return fmt.Errorf("failed to load stack: %w", err)
	}

	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	branches, err := c.Stack.GetStackBranches(username, stackName)
	if err != nil {
		return fmt.Errorf("failed to get stack branches: %w", err)
	}

	c.showDeletionSummary(stackCtx, branches)

	if !c.Force {
		prompt := fmt.Sprintf("Type the stack name '%s' to confirm deletion: ", ui.Bold(stackName))
		if !ui.Confirm(prompt, stackName) {
			fmt.Println("Deletion cancelled.")
			return nil
		}
		fmt.Println()
	}

	fmt.Println(ui.RenderInfoMessage("Deleting stack..."))
	fmt.Println()

	if err := c.Stack.DeleteStack(stackName, c.Force); err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.RenderSuccessMessagef("Successfully deleted stack: %s", stackName))
	return nil
}

func (c *Command) resolveStackName() (string, error) {
	if c.StackName != "" {
		if !c.Stack.StackExists(c.StackName) {
			return "", fmt.Errorf("stack '%s' not found", c.StackName)
		}
		return c.StackName, nil
	}

	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return "", fmt.Errorf("failed to get stack context: %w", err)
	}
	if !stackCtx.IsStack() {
		return "", fmt.Errorf("not on a stack branch. Specify stack name: stack delete <name>")
	}
	return stackCtx.StackName, nil
}

func (c *Command) showDeletionSummary(stackCtx *stack.StackContext, branches []string) {
	openCount, mergedCount := 0, 0
	for _, change := range stackCtx.AllChanges {
		if change.IsMerged {
			mergedCount++
		} else {
			openCount++
		}
	}

	fmt.Println(ui.RenderWarningMessagef("About to delete stack: %s", ui.Bold(stackCtx.StackName)))
	fmt.Println()
	fmt.Printf("  Stack details:\n")
	fmt.Printf("    Base branch: %s\n", stackCtx.Stack.Base)
	fmt.Printf("    Changes: %d total (%d open, %d merged)\n", len(stackCtx.AllChanges), openCount, mergedCount)
	fmt.Printf("    Branches: %d\n", len(branches))
	if len(branches) > 0 {
		fmt.Printf("\n  Branches to be deleted:\n")
		for _, branch := range branches {
			fmt.Printf("    - %s\n", branch)
		}
	}
	fmt.Printf("\n  Metadata will be archived to:\n")
	fmt.Printf("    .git/stack/.archived/%s-<timestamp>\n", stackCtx.StackName)
	fmt.Println()
}
