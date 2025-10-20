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
	c.Stack = stack.NewClient(c.Git.GitRoot())

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
	// Determine which stack to show
	if c.StackName == "" {
		// Get current stack
		currentStack, err := c.Stack.GetCurrentStack()
		if err != nil {
			return fmt.Errorf("no current stack set. Use: stack show <name>")
		}
		c.StackName = currentStack
	}

	// Load stack
	s, err := c.Stack.LoadStack(c.StackName)
	if err != nil {
		return fmt.Errorf("failed to load stack '%s': %w", c.StackName, err)
	}

	// Get commits
	commits, err := c.Git.GetCommits(s.Branch, s.Base)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Load PRs
	prs, err := c.Stack.LoadPRs(c.StackName)
	if err != nil {
		return fmt.Errorf("failed to load PRs: %w", err)
	}

	// Print stack header
	fmt.Printf("Stack: %s (%s)\n", s.Name, s.Branch)
	fmt.Printf("Base: %s\n", s.Base)
	fmt.Println()

	// If no commits, show message
	if len(commits) == 0 {
		fmt.Println("No PRs in this stack yet.")
		fmt.Println("Add commits to the stack branch to create PRs.")
		return nil
	}

	// Print table header
	fmt.Printf(" #  Status    PR      Title%sCommit\n", strings.Repeat(" ", 30))
	fmt.Println(strings.Repeat("━", 80))

	// Print commits
	for i, commit := range commits {
		prNum := i + 1

		// Get PR status
		uuid := commit.Trailers["PR-UUID"]
		status := "⚪ Local"
		prLabel := "-"

		if uuid != "" {
			if pr, ok := prs[uuid]; ok {
				prLabel = fmt.Sprintf("#%d", pr.PRNumber)
				switch pr.State {
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
		}

		// Truncate title if too long
		title := commit.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		// Short hash
		shortHash := commit.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		fmt.Printf(" %-2d %-9s %-7s %-33s %s\n", prNum, status, prLabel, title, shortHash)
	}

	// Print summary
	fmt.Println()

	localCount := 0
	openCount := 0
	draftCount := 0

	for _, commit := range commits {
		uuid := commit.Trailers["PR-UUID"]
		if uuid == "" {
			localCount++
		} else if pr, ok := prs[uuid]; ok {
			switch pr.State {
			case "open":
				openCount++
			case "draft":
				draftCount++
			}
		} else {
			localCount++
		}
	}

	fmt.Printf("%d PR", len(commits))
	if len(commits) != 1 {
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
