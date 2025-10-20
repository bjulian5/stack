package hook

import (
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// Command is the parent command for all hook subcommands
type Command struct {
	// Clients (shared by subcommands)
	Git   *git.Client
	Stack *stack.Client
}

// Register registers the hook command and all subcommands
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		// Hooks should fail silently if not in a git repo
		return
	}
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:    "hook",
		Short:  "Git hook commands (internal use)",
		Long:   `Hook commands are called by git hooks and should not be run directly by users.`,
		Hidden: true, // Hide from normal help output
	}

	// Register subcommands
	prepareCommitMsg := &PrepareCommitMsgCommand{Git: c.Git, Stack: c.Stack}
	prepareCommitMsg.Register(cmd)

	commitMsg := &CommitMsgCommand{Git: c.Git, Stack: c.Stack}
	commitMsg.Register(cmd)

	postCommit := &PostCommitCommand{Git: c.Git, Stack: c.Stack}
	postCommit.Register(cmd)

	parent.AddCommand(cmd)
}
