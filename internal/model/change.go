package model

import (
	"time"

	"github.com/bjulian5/stack/internal/gh"
)

// Change represents a commit in the context of a stack that may become a pull request.
type Change struct {
	Position       int
	ActivePosition int
	Title          string
	Description    string
	CommitHash     string
	UUID           string
	PR             *PR
	MergedAt       time.Time `json:"merged_at"`
	DesiredBase    string
}

func (c *Change) IsLocal() bool {
	return c.PR == nil || c.PR.PRNumber == 0
}

func (c *Change) GetDraftStatus() bool {
	if c.PR != nil {
		return c.PR.LocalDraftStatus
	}
	return true
}

type ChangeSyncStatus struct {
	NeedsSync bool
	Reason    string
}

func (c *Change) NeedsSyncToGitHub() ChangeSyncStatus {
	if c.IsLocal() {
		return ChangeSyncStatus{NeedsSync: true, Reason: "new change"}
	}

	if c.PR.Title == "" || c.PR.Base == "" {
		return ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"}
	}

	if c.CommitHash != c.PR.CommitHash {
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

// UpdateFromPush updates the PR metadata after a successful push to GitHub
func (c *Change) UpdateFromPush(ghPR *gh.PR, branch string) {
	if c.PR == nil {
		c.PR = &PR{
			CreatedAt:        ghPR.CreatedAt,
			LocalDraftStatus: true, // Will be updated below
		}
	}

	// Update PR fields from GitHub response
	c.PR.PRNumber = ghPR.Number
	c.PR.URL = ghPR.URL
	c.PR.State = ghPR.State
	c.PR.Branch = branch
	c.PR.CommitHash = c.CommitHash
	c.PR.LastPushed = ghPR.UpdatedAt
	c.PR.RemoteDraftStatus = ghPR.IsDraft

	// Also update the local draft status to match what we just pushed
	// (the caller should have already set LocalDraftStatus before calling SyncPR)
	c.PR.LocalDraftStatus = ghPR.IsDraft
}

// UpdateTitle updates the title and description after syncing
func (c *Change) UpdateTitle(title, description, base string) {
	if c.PR != nil {
		c.PR.Title = title
		c.PR.Body = description
		c.PR.Base = base
	}
}

// StackChanges contains the various categories of changes in a stack.
type StackChanges struct {
	// All includes merged + active changes (deduplicated by UUID).
	All []Change
	// Active includes only unmerged changes currently on the stack branch.
	Active []Change
	// StaleMerged includes active changes that are merged on GitHub but still on the TOP branch (need refresh).
	StaleMerged []Change
}
