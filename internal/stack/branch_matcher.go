package stack

import "github.com/bjulian5/stack/internal/model"

// BranchMatcher provides logic for checking if a branch belongs to a stack
type BranchMatcher interface {
	IsStackBranch(branchName string) bool
}

// DefaultBranchMatcher implements BranchMatcher for standard stack branches
type DefaultBranchMatcher struct {
	stack    *model.Stack
	branches []string
}

// NewBranchMatcher creates a new DefaultBranchMatcher
func NewBranchMatcher(stack *model.Stack, branches []string) *DefaultBranchMatcher {
	return &DefaultBranchMatcher{
		stack:    stack,
		branches: branches,
	}
}

// IsStackBranch checks if the given branch name is part of this stack
func (m *DefaultBranchMatcher) IsStackBranch(branchName string) bool {
	// Check TOP branch
	if branchName == m.stack.Branch {
		return true
	}

	// Check UUID branches
	for _, branch := range m.branches {
		if branch == branchName {
			return true
		}
	}

	return false
}
