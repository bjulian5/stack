package cmd

import "github.com/spf13/cobra"

// Command represents a CLI command that can register itself with cobra
type Command interface {
	// Register adds the command to the parent cobra command
	Register(parent *cobra.Command)
}
