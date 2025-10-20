package stack

// Change represents a change in a stack
// This is the domain-level abstraction for a commit in the context of stacks.
// Each change may eventually become a pull request.
type Change struct {
	// Position is the 1-indexed position of this change in the stack
	Position int

	// Title is the first line of the commit message
	Title string

	// Description is the body of the commit message
	Description string

	// CommitHash is the git commit hash
	CommitHash string

	// UUID is the PR identifier (empty if not yet assigned)
	UUID string

	// PR contains GitHub PR metadata if this change has been pushed
	// Will be nil if the change is only local
	PR *PR
}
