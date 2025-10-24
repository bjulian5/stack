package hook

import (
	"fmt"
	"os"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PostUpdateWorkflow updates UUID branches and checks out the return branch after modifying a stack.
func PostUpdateWorkflow(g *git.Client, s *stack.Client, ctx *stack.StackContext, returnBranch string) error {
	if err := updateAllUUIDBranches(g, ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update UUID branches: %v\n", err)
	}

	if err := g.CheckoutBranch(returnBranch); err != nil {
		return fmt.Errorf("failed to checkout UUID branch: %w", err)
	}

	return nil
}

func updateAllUUIDBranches(g *git.Client, ctx *stack.StackContext) error {
	username, err := common.GetUsername()
	if err != nil {
		return fmt.Errorf("failed to get username: %w", err)
	}

	for i := range ctx.ActiveChanges {
		change := &ctx.ActiveChanges[i]

		if change.UUID == "" {
			continue
		}

		branchName := ctx.FormatUUIDBranch(username, change.UUID)

		if !g.BranchExists(branchName) {
			continue
		}

		if err := g.UpdateRef(branchName, change.CommitHash); err != nil {
			return fmt.Errorf("failed to update branch %s: %w", branchName, err)
		}
	}

	return nil
}
