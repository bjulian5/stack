package pr

import (
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/cmd/pr/open"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// Command is the parent command for all pr subcommands
type Command struct {
	// Clients (shared by subcommands)
	Git   *git.Client
	Stack *stack.Client
}

// Register registers the pr command and all subcommands
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:   "pr",
		Short: "PR operations",
		Long:  `Commands for working with pull requests in the current stack.`,
	}

	// Register subcommands
	openCmd := &open.Command{Git: c.Git, Stack: c.Stack}
	openCmd.Register(cmd)

	parent.AddCommand(cmd)
}
