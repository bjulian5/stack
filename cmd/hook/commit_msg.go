package hook

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// CommitMsgCommand implements the commit-msg hook
type CommitMsgCommand struct {
	Git   *git.Client
	Stack *stack.Client

	MessageFile string
}

// Register registers the commit-msg command
func (c *CommitMsgCommand) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "commit-msg <file>",
		Short: "commit-msg git hook",
		Long:  `Called by git after the commit message is written, validates the message.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.MessageFile = args[0]
			return c.Run()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	parent.AddCommand(cmd)
}

// Run executes the commit-msg hook
func (c *CommitMsgCommand) Run() error {
	ctx, err := c.Stack.GetStackContext()
	if err != nil || !ctx.IsStack() {
		return nil
	}

	content, err := os.ReadFile(c.MessageFile)
	if err != nil {
		return nil
	}

	// Strip comments to avoid trailer parsing issues
	message := string(content)
	stripped, err := c.Git.StripComments(message)
	if err != nil {
		stripped = message
	}

	commit := git.ParseCommitMessage(stripped)

	if commit.Trailers["PR-UUID"] == "" {
		fmt.Fprintln(os.Stderr, "Error: Commit message missing PR-UUID trailer")
		fmt.Fprintln(os.Stderr, "This should have been added by prepare-commit-msg hook")
		return fmt.Errorf("missing PR-UUID trailer")
	}

	if commit.Trailers["PR-Stack"] == "" {
		fmt.Fprintln(os.Stderr, "Error: Commit message missing PR-Stack trailer")
		fmt.Fprintln(os.Stderr, "This should have been added by prepare-commit-msg hook")
		return fmt.Errorf("missing PR-Stack trailer")
	}

	if strings.TrimSpace(commit.Title) == "" {
		fmt.Fprintln(os.Stderr, "Error: Commit message title cannot be empty")
		fmt.Fprintln(os.Stderr, "The first line of your commit message will be the PR title")
		return fmt.Errorf("empty commit title")
	}

	return nil
}
