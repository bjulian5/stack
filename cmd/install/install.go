package install

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
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
	command := &cobra.Command{
		Use:   "install",
		Short: "Install stack hooks and configure git",
		Long: `Install stack's git hooks and configure git settings.

This command:
  1. Installs git hooks (prepare-commit-msg, post-commit, commit-msg)
  2. Configures git settings (core.commentChar=; to avoid markdown conflicts)
  3. Saves installation state

An up-to-date installation is checked without writing files, so this command
also works in sandboxes that allow reading but not modifying shared hooks.

Example:
  stack install`,
		Args: cobra.NoArgs,
		PreRunE: func(cobraCmd *cobra.Command, args []string) error {
			var err error
			c.Git, _, c.Stack, err = common.InitClients()
			if err != nil {
				ui.Println("")
				ui.Info("The 'stack install' command must be run from within a git repository.")
				ui.Info("Please navigate to your git repository and try again.")
			}
			return err
		},
		RunE: c.Run,
	}

	parent.AddCommand(command)
}

func (c *Command) Run(cmd *cobra.Command, args []string) error {
	installed, err := c.Stack.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status: %w", err)
	}

	commentChar, err := c.Git.GetConfig("core.commentChar")
	if err != nil {
		return fmt.Errorf("failed to check git configuration: %w", err)
	}
	if installed && hooks.CheckHooksCurrent(c.Git.GitCommonDir()) && commentChar == ";" {
		ui.Success("Stack is already installed and up to date.")
		return nil
	}

	if err := hooks.InstallHooks(c.Git.GitCommonDir()); err != nil {
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
