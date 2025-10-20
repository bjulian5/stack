package hook

import (
	"fmt"
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

	// Arguments from git

	// MessageFile is the path to the commit message file
	MessageFile string

	// Source is the source of the commit message. Has values of
	// "message", "template", "merge", "squash", or "commit"
	Source string

	// CommitSHA is the commit SHA (if applicable)
	CommitSHA string
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
	// Get current branch
	branch, err := c.Git.GetCurrentBranch()
	if err != nil {
		// Not in a git repo or error - exit silently
		return nil
	}

	// Check if on stack branch or UUID branch
	if !git.IsStackBranch(branch) && !git.IsUUIDBranch(branch) {
		// Not on a stack-related branch - exit silently
		return nil
	}

	// Read commit message file
	content, err := os.ReadFile(c.MessageFile)
	if err != nil {
		return nil // Exit silently on error
	}

	message := string(content)

	// Check if message already has PR-UUID trailer (amend case)
	if hasTrailer(message, "PR-UUID") {
		// Already has metadata, don't modify
		return nil
	}

	// Generate UUID
	uuid := common.GenerateUUID()

	// Get stack name
	var stackName string
	if git.IsStackBranch(branch) {
		stackName = git.ExtractStackName(branch)
	} else {
		stackName, _ = git.ExtractUUIDFromBranch(branch)
	}

	if stackName == "" {
		// Can't determine stack name - exit silently
		return nil
	}

	// Add trailers to message
	message = addTrailers(message, uuid, stackName)

	// Write back to file
	if err := os.WriteFile(c.MessageFile, []byte(message), 0644); err != nil {
		return nil // Exit silently
	}

	return nil
}

// hasTrailer checks if a message has a specific trailer
func hasTrailer(message string, key string) bool {
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			return true
		}
	}
	return false
}

// addTrailers adds PR-UUID and PR-Stack trailers to a commit message
func addTrailers(message string, uuid string, stackName string) string {
	// Ensure message ends with newline
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}

	// Add blank line before trailers if message is not empty and doesn't end with blank line
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		message += "\n"
	}

	// Add trailers
	message += fmt.Sprintf("PR-UUID: %s\n", uuid)
	message += fmt.Sprintf("PR-Stack: %s\n", stackName)

	return message
}
