package cmd

import (
	"fmt"
	"strings"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show [stack-name]",
	Short: "Show details of a stack",
	Long: `Show detailed information about a stack including all PRs.

If no stack name is provided, shows the current stack.

Example:
  stack show
  stack show auth-refactor`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in a git repository
		if !git.IsGitRepo() {
			return fmt.Errorf("not in a git repository")
		}

		// Determine which stack to show
		var stackName string
		if len(args) > 0 {
			stackName = args[0]
		} else {
			// Get current stack
			currentStack, err := stack.GetCurrentStack()
			if err != nil {
				return fmt.Errorf("no current stack set. Use: stack show <name>")
			}
			stackName = currentStack
		}

		// Load stack
		s, err := stack.LoadStack(stackName)
		if err != nil {
			return fmt.Errorf("failed to load stack '%s': %w", stackName, err)
		}

		// Get commits
		commits, err := git.GetCommits(s.Branch, s.Base)
		if err != nil {
			return fmt.Errorf("failed to get commits: %w", err)
		}

		// Load PRs
		prs, err := stack.LoadPRs(stackName)
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
	},
}
