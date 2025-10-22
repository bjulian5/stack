package restack

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// Command rebases the stack on top of the base branch
type Command struct {
	Git   *git.Client
	Stack *stack.Client

	// Flags
	Fetch bool
	Onto  string
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
		Use:   "restack",
		Short: "Rebase the stack on top of the latest base branch changes",
		Long: `Rebase the stack on top of the latest base branch changes.

By default, this command fetches from the remote and rebases your stack on top
of the updated base branch. This is the typical workflow for pulling in upstream
changes before pushing your stack.

Use --onto to move your stack to a different base branch (e.g., from main to develop).
When using --onto, fetching is NOT automatic - add --fetch if needed.

Examples:
  # Fetch and rebase on latest origin/main (most common)
  stack restack

  # Move stack to a different base branch
  stack restack --onto develop

  # Fetch first, then move to different base
  stack restack --onto develop --fetch`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&c.Fetch, "fetch", false, "Fetch from remote before rebasing")
	cmd.Flags().StringVar(&c.Onto, "onto", "", "Rebase stack onto a different base branch")

	parent.AddCommand(cmd)
}

func (c *Command) Run(ctx context.Context) error {
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}

	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack.")
	}

	if stackCtx.IsEditing() {
		return fmt.Errorf("cannot restack while editing a change. Commit or abort your changes first.")
	}

	hasChanges, err := c.Git.HasUncommittedChanges()
	if err != nil {
		return err
	}
	if hasChanges {
		return fmt.Errorf("you have uncommitted changes. Commit or stash them before restacking.")
	}

	// Determine target base: use --onto if specified, otherwise current base
	targetBase := c.Onto
	fetch := c.Fetch

	if targetBase == "" {
		targetBase = stackCtx.Stack.Base
		fetch = true
	}

	opts := stack.RestackOptions{
		Onto:  targetBase,
		Fetch: fetch,
	}
	if err := c.Stack.Restack(stackCtx, opts); err != nil {
		return err
	}

	fmt.Println(ui.RenderSuccessMessage(fmt.Sprintf("Restacked on %s", targetBase)))
	return nil
}
