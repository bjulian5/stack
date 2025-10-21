package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/cmd/down"
	"github.com/bjulian5/stack/cmd/edit"
	"github.com/bjulian5/stack/cmd/hook"
	"github.com/bjulian5/stack/cmd/list"
	"github.com/bjulian5/stack/cmd/newcmd"
	"github.com/bjulian5/stack/cmd/pr"
	"github.com/bjulian5/stack/cmd/push"
	"github.com/bjulian5/stack/cmd/show"
	switchcmd "github.com/bjulian5/stack/cmd/switch"
	"github.com/bjulian5/stack/cmd/up"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stack",
	Short: "Git-native stacked PR tool",
	Long: `Stack is a CLI tool for managing stacked pull requests on GitHub.

It allows developers to create, manage, and sync stacked PRs while using
familiar git commands for most operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(ctx context.Context) {
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	// Register all commands
	commands := []Command{
		&newcmd.Command{},
		&list.Command{},
		&show.Command{},
		&edit.Command{},
		&up.Command{},
		&down.Command{},
		&switchcmd.Command{},
		&push.Command{},
		&pr.Command{},
		&hook.Command{},
	}

	for _, cmd := range commands {
		cmd.Register(rootCmd)
	}
}
