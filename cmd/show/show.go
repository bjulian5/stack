package show

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// Command shows details of a stack
type Command struct {
	// Arguments
	StackName string

	// Clients (can be mocked in tests)
	Git   *git.Client
	Stack *stack.Client
}

// Register registers the command with cobra
func (c *Command) Register(parent *cobra.Command) {
	var err error
	c.Git, err = git.NewClient()
	if err != nil {
		panic(err)
	}
	c.Stack = stack.NewClient(c.Git)

	cmd := &cobra.Command{
		Use:   "show [stack-name]",
		Short: "Show details of a stack",
		Long: `Show detailed information about a stack including all PRs.

If no stack name is provided, shows the current stack.

Example:
  stack show
  stack show auth-refactor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.StackName = args[0]
			}
			return c.Run(cmd.Context())
		},
	}

	parent.AddCommand(cmd)
}

// Run executes the command
func (c *Command) Run(ctx context.Context) error {
	// Resolve stack name - use context if not specified
	stackName := c.StackName
	if stackName == "" {
		stackCtx, err := c.Stack.GetStackContext()
		if err != nil || !stackCtx.IsStack() {
			return fmt.Errorf("not on a stack branch. Use: stack show <name>")
		}
		stackName = stackCtx.StackName
	}

	// Get stack context
	stackCtx, err := c.Stack.GetStackContextByName(stackName)
	if err != nil {
		return err
	}

	if stackCtx.Stack == nil {
		return fmt.Errorf("stack '%s' does not exist", stackName)
	}

	s := stackCtx.Stack
	changes := stackCtx.Changes

	// Print stack header
	fmt.Printf("Stack: %s (%s)\n", s.Name, s.Branch)
	fmt.Printf("Base: %s\n", s.Base)
	fmt.Println()

	// If no changes, show message
	if len(changes) == 0 {
		fmt.Println("No PRs in this stack yet.")
		fmt.Println("Add commits to the stack branch to create PRs.")
		return nil
	}

	// Print table header
	fmt.Printf(" #  Status    PR      Title%sCommit\n", strings.Repeat(" ", 30))
	fmt.Println(strings.Repeat("━", 80))

	// Print changes
	for _, change := range changes {
		// Get PR status
		status := "⚪ Local"
		prLabel := "-"

		if change.PR != nil {
			prLabel = fmt.Sprintf("#%d", change.PR.PRNumber)
			switch change.PR.State {
			case "open":
				status = "🟢 Open"
			case "draft":
				status = "🟡 Draft"
			case "merged":
				status = "🟣 Merged"
			case "closed":
				status = "⚫ Closed"
			}
		}

		// Truncate title if too long
		title := change.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		// Short hash
		shortHash := change.CommitHash
		if len(shortHash) > git.ShortHashLength {
			shortHash = shortHash[:git.ShortHashLength]
		}

		fmt.Printf(" %-2d %-9s %-7s %-33s %s\n", change.Position, status, prLabel, title, shortHash)
	}

	// Print summary
	fmt.Println()

	localCount := 0
	openCount := 0
	draftCount := 0

	for _, change := range changes {
		if change.PR == nil {
			localCount++
		} else {
			switch change.PR.State {
			case "open":
				openCount++
			case "draft":
				draftCount++
			}
		}
	}

	fmt.Printf("%d PR", len(changes))
	if len(changes) != 1 {
		fmt.Print("s")
	}
	fmt.Print(" total (")

	parts := []string{}
	if openCount > 0 {
		parts = append(parts, fmt.Sprintf("%d open", openCount))
	}
	if draftCount > 0 {
		parts = append(parts, fmt.Sprintf("%d draft", draftCount))
	}
	if localCount > 0 {
		parts = append(parts, fmt.Sprintf("%d local", localCount))
	}

	fmt.Printf("%s)\n", strings.Join(parts, ", "))

	// Legend
	fmt.Println()
	fmt.Println("Legend:")
	fmt.Println("🟢 Open   - PR is open and ready for review")
	fmt.Println("🟡 Draft  - PR is in draft state")
	fmt.Println("🟣 Merged - PR has been merged")
	fmt.Println("⚪ Local  - Not yet pushed to GitHub")

	return nil
}
