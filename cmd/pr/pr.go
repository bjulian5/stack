package pr

import (
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/cmd/pr/draft"
	"github.com/bjulian5/stack/cmd/pr/open"
	"github.com/bjulian5/stack/cmd/pr/ready"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

type Command struct {
	Git   *git.Client
	Stack *stack.Client
}

func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	ghClient := gh.NewClient()
	c.Stack = stack.NewClient(c.Git, ghClient)

	cmd := &cobra.Command{
		Use:   "pr",
		Short: "PR operations",
		Long:  `Commands for working with pull requests in the current stack.`,
	}

	openCmd := &open.Command{Git: c.Git, Stack: c.Stack, GH: ghClient}
	openCmd.Register(cmd)

	readyCmd := &ready.Command{Git: c.Git, Stack: c.Stack}
	readyCmd.Register(cmd)

	draftCmd := &draft.Command{Git: c.Git, Stack: c.Stack}
	draftCmd.Register(cmd)

	parent.AddCommand(cmd)
}
