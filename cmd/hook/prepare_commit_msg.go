package hook

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PrepareCommitMsgCommand implements the prepare-commit-msg hook
type PrepareCommitMsgCommand struct {
	Git   *git.Client
	Stack *stack.Client

	MessageFile string
	Source      string
	CommitSHA   string
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
	ctx, err := c.Stack.GetStackContext()
	if err != nil || !ctx.IsStack() {
		return nil
	}

	content, err := os.ReadFile(c.MessageFile)
	if err != nil {
		return nil
	}

	// Strip comments to avoid parsing them as trailers
	stripped, err := c.Git.StripComments(string(content))
	if err != nil {
		stripped = string(content)
	}

	commitMsg := git.ParseCommitMessage(stripped)

	if commitMsg.Trailers["PR-UUID"] != "" {
		return nil
	}

	if commitMsg.Body == "" {
		if template, _ := c.Git.FindPRTemplate(); template != "" {
			commitMsg.Body = template
		}
	}

	uuid := common.GenerateUUID()
	commitMsg.AddTrailer("PR-UUID", uuid)
	commitMsg.AddTrailer("PR-Stack", ctx.StackName)

	newContent := commitMsg.String()

	// Preserve git comments from original
	if len(content) > len(stripped) {
		commentStart := len(stripped)
		comments := string(content[commentStart:])
		if trimmed := strings.TrimSpace(comments); trimmed != "" {
			newContent = newContent + "\n" + trimmed
		}
	}

	if err := os.WriteFile(c.MessageFile, []byte(newContent), 0644); err != nil {
		return nil
	}

	return nil
}
