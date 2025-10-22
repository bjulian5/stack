package stack

import "time"

// Change represents a change in a stack
// This is the domain-level abstraction for a commit in the context of stacks.
// Each change may eventually become a pull request.
type Change struct {
	// Position is the 1-indexed position of this change in the stack
	// For active changes, this is their position in ActiveChanges.
	// For merged changes, this preserves their position from when they were active.
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

	// IsMerged indicates if this change has been merged to the base branch
	// Merged changes are stored in Stack metadata and are read-only.
	IsMerged bool

	// MergedAt is the timestamp when this change was merged on GitHub
	// Zero value if not merged
	MergedAt time.Time `json:"merged_at"`
}

// NeedsPush returns true if this change has a PR but the local commit
// differs from what was last pushed to GitHub.
func (c *Change) NeedsPush() bool {
	// No PR means it's local-only (not yet pushed)
	if c.PR == nil {
		return false
	}

	// Compare local commit hash with the hash stored in PR metadata
	return c.CommitHash != c.PR.CommitHash
}
