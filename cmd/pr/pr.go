package pr

import (
	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/cmd/pr/draft"
	"github.com/bjulian5/stack/cmd/pr/open"
	"github.com/bjulian5/stack/cmd/pr/ready"
)

type Command struct{}

func (c *Command) Register(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "PR operations",
		Long:  `Commands for working with pull requests in the current stack.`,
	}

	// Subcommands will initialize their own clients in PreRunE
	openCmd := &open.Command{}
	openCmd.Register(cmd)

	readyCmd := &ready.Command{}
	readyCmd.Register(cmd)

	draftCmd := &draft.Command{}
	draftCmd.Register(cmd)

	parent.AddCommand(cmd)
}
