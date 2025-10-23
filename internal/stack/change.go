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

	// DesiredBase is the base branch this change should target
	// Calculated based on stacking order: first change targets stack.Base,
	// subsequent changes target the previous change's PR branch
	DesiredBase string
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

// GetDraftStatus returns the desired draft status for this change.
// If the change has been pushed, returns the user's desired state from PR.LocalDraftStatus.
// For local-only changes, defaults to true (draft).
func (c *Change) GetDraftStatus() bool {
	if c.PR != nil {
		return c.PR.LocalDraftStatus
	}
	return true // Default new changes to draft
}

// ChangeSyncStatus contains information about whether a change needs syncing to GitHub
type ChangeSyncStatus struct {
	NeedsSync bool
	Reason    string
}

// NeedsSyncToGitHub checks if this change needs to be synced to GitHub.
// Compares local state (commit, title, description, base, draft status) with what's on GitHub.
// Uses the stored DesiredBase field to check if the base branch has changed.
// Returns false for local-only changes (no PR yet).
func (c *Change) NeedsSyncToGitHub() ChangeSyncStatus {
	// No PR means it's local-only, not a sync issue
	if c.PR == nil {
		return ChangeSyncStatus{NeedsSync: false}
	}

	// Check if metadata is cached
	if c.PR.Title == "" || c.PR.Body == "" || c.PR.Base == "" {
		return ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"}
	}

	// Check commit hash (uses existing NeedsPush logic)
	if c.NeedsPush() {
		return ChangeSyncStatus{NeedsSync: true, Reason: "commit changed"}
	}

	// Check title
	if c.PR.Title != c.Title {
		return ChangeSyncStatus{NeedsSync: true, Reason: "title changed"}
	}

	// Check description
	if c.PR.Body != c.Description {
		return ChangeSyncStatus{NeedsSync: true, Reason: "description changed"}
	}

	// Check base branch using stored DesiredBase
	if c.DesiredBase != "" && c.PR.Base != c.DesiredBase {
		return ChangeSyncStatus{NeedsSync: true, Reason: "base changed"}
	}

	// Check draft status mismatch
	if c.PR.LocalDraftStatus != c.PR.RemoteDraftStatus {
		return ChangeSyncStatus{NeedsSync: true, Reason: "draft status changed"}
	}

	return ChangeSyncStatus{NeedsSync: false}
}
