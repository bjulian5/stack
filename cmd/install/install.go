package install

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/hooks"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
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
		Use:   "install",
		Short: "Install stack hooks and configure git",
		Long: `Install stack's git hooks and configure git settings.

This command:
  1. Installs git hooks (prepare-commit-msg, post-commit, commit-msg)
  2. Configures git settings (core.commentChar=; to avoid markdown conflicts)
  3. Saves installation state

This command is idempotent and can be run multiple times safely.

Example:
  stack install`,
		Args: cobra.NoArgs,
		RunE: c.Run,
	}

	parent.AddCommand(cmd)
}

func (c *Command) Run(cmd *cobra.Command, args []string) error {
	installed, err := c.Stack.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status: %w", err)
	}

	if installed {
		ui.Info("Stack is already installed in this repository.")
		ui.Info("Reinstalling...")
	}

	if err := hooks.InstallHooks(c.Git.GitRoot()); err != nil {
		return fmt.Errorf("failed to install git hooks: %w", err)
	}
	ui.Success("Git hooks installed")

	if err := c.Git.SetConfig("core.commentChar", ";"); err != nil {
		return fmt.Errorf("failed to configure git: %w", err)
	}
	ui.Success("Git configured (core.commentChar=';')")

	if err := c.Stack.MarkInstalled(); err != nil {
		return fmt.Errorf("failed to save installation state: %w", err)
	}

	ui.Print("")
	ui.Success("Installation complete!")
	ui.Print("")
	ui.Print("Get started by creating your first stack:")
	ui.Print("  " + ui.Highlight("stack new <stack-name>"))

	return nil
}
