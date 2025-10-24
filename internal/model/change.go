package model

import "time"

// Change represents a commit in the context of a stack that may become a pull request.
type Change struct {
	Position    int
	Title       string
	Description string
	CommitHash  string
	UUID        string
	PR          *PR
	IsMerged    bool
	MergedAt    time.Time `json:"merged_at"`
	DesiredBase string
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
		return ChangeSyncStatus{NeedsSync: false}
	}

	if c.PR.Title == "" || c.PR.Body == "" || c.PR.Base == "" {
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
