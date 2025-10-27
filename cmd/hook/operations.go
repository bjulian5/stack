package hook

import (
	"fmt"

	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
)

// PostUpdateWorkflow updates UUID branches and checks out the return branch after modifying a stack.
func PostUpdateWorkflow(g *git.Client, s *stack.Client, ctx *stack.StackContext, returnBranch string) error {
	if _, err := s.UpdateUUIDBranches(ctx.StackName); err != nil {
		return fmt.Errorf("failed to update UUID branches: %w", err)
	}

	if err := g.CheckoutBranch(returnBranch); err != nil {
		return fmt.Errorf("failed to checkout UUID branch: %w", err)
	}

	return nil
}
