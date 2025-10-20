package stack

// StackContext represents the current stack context based on the git branch.
// This is the single source of truth for what stack you're working on - it's
// derived from the branch you're currently on.
type StackContext struct {
	// StackName is the name of the stack this branch belongs to.
	// Empty if the current branch is not a stack-related branch.
	StackName string

	// Stack is the loaded stack metadata.
	// Nil if not on a stack-related branch.
	Stack *Stack

	// Change is the specific change being edited on a UUID branch.
	// Nil if on the TOP branch or not in a stack.
	Change *Change
}

// InStack returns true if the current branch is part of a stack.
func (ctx *StackContext) InStack() bool {
	return ctx.StackName != ""
}

// IsEditing returns true if currently on a UUID branch (editing a specific change).
// Returns false if on TOP branch or not in a stack.
func (ctx *StackContext) IsEditing() bool {
	return ctx.Change != nil
}
