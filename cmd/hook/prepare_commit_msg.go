package hook

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PrepareCommitMsgCommand implements the prepare-commit-msg hook
type PrepareCommitMsgCommand struct {
	Git   *git.Client
	Stack *stack.Client

	// Arguments from git

	// MessageFile is the path to the commit message file
	MessageFile string

	// Source is the source of the commit message. Has values of
	// "message", "template", "merge", "squash", or "commit"
	Source string

	// CommitSHA is the commit SHA (if applicable)
	CommitSHA string
}

// Register registers the prepare-commit-msg command
func (c *PrepareCommitMsgCommand) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "prepare-commit-msg <file> <source> [<sha>]",
		Short: "prepare-commit-msg git hook",
		Long:  `Called by git before opening the commit message editor.`,
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.MessageFile = args[0]

			if len(args) > 1 {
				c.Source = args[1]
			}

			if len(args) > 2 {
				c.CommitSHA = args[2]
			}

			return c.Run()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	parent.AddCommand(cmd)
}

// Run executes the prepare-commit-msg hook
func (c *PrepareCommitMsgCommand) Run() error {
	// Get stack context
	ctx, err := c.Stack.GetStackContext()
	if err != nil || !ctx.InStack() {
		// Not in a stack or error - exit silently
		return nil
	}

	// Read commit message file
	content, err := os.ReadFile(c.MessageFile)
	if err != nil {
		return nil // Exit silently on error
	}

	message := string(content)

	// Check if message already has PR-UUID trailer (amend case)
	if git.GetTrailer(message, "PR-UUID") != "" {
		// Already has metadata, don't modify
		return nil
	}

	// Generate UUID
	uuid := common.GenerateUUID()

	// Add trailers to message
	message = git.AddTrailer(message, "PR-UUID", uuid)
	message = git.AddTrailer(message, "PR-Stack", ctx.StackName)

	// Write back to file
	if err := os.WriteFile(c.MessageFile, []byte(message), 0644); err != nil {
		return nil // Exit silently
	}

	return nil
}
