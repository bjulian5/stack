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

// IsLocal returns true if this change hasn't been pushed to GitHub yet.
// A change is local if it has no PR or if the PR entry only exists to store draft preferences (PRNumber == 0).
func (c *Change) IsLocal() bool {
	return c.PR == nil || c.PR.PRNumber == 0
}

// NeedsPush returns true if this change has a PR but the local commit
// differs from what was last pushed to GitHub.
func (c *Change) NeedsPush() bool {
	if c.IsLocal() {
		return false
	}
	return c.CommitHash != c.PR.CommitHash
}

// GetDraftStatus returns the desired draft status for this change.
// For local-only changes, defaults to true (draft).
func (c *Change) GetDraftStatus() bool {
	if c.PR != nil {
		return c.PR.LocalDraftStatus
	}
	return true
}

// ChangeSyncStatus contains information about whether a change needs syncing to GitHub
type ChangeSyncStatus struct {
	NeedsSync bool
	Reason    string
}

// NeedsSyncToGitHub checks if this change needs to be synced to GitHub.
// Compares local state (commit, title, description, base, draft status) with what's on GitHub.
func (c *Change) NeedsSyncToGitHub() ChangeSyncStatus {
	if c.IsLocal() {
		return ChangeSyncStatus{NeedsSync: false}
	}

	if c.PR.Title == "" || c.PR.Body == "" || c.PR.Base == "" {
		return ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"}
	}

	if c.NeedsPush() {
		return ChangeSyncStatus{NeedsSync: true, Reason: "commit changed"}
	}

	if c.PR.Title != c.Title {
		return ChangeSyncStatus{NeedsSync: true, Reason: "title changed"}
	}

	if c.PR.Body != c.Description {
		return ChangeSyncStatus{NeedsSync: true, Reason: "description changed"}
	}

	if c.DesiredBase != "" && c.PR.Base != c.DesiredBase {
		return ChangeSyncStatus{NeedsSync: true, Reason: "base changed"}
	}

	if c.PR.LocalDraftStatus != c.PR.RemoteDraftStatus {
		return ChangeSyncStatus{NeedsSync: true, Reason: "draft status changed"}
	}

	return ChangeSyncStatus{NeedsSync: false}
}
