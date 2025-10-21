package stack

import (
	"time"
)

// Stack represents a PR stack
type Stack struct {
	Name          string    `json:"name"`
	Branch        string    `json:"branch"`
	Base          string    `json:"base"`
	Created       time.Time `json:"created"`
	LastSynced    time.Time `json:"last_synced"`    // When we last checked GitHub for merged PRs
	SyncHash      string    `json:"sync_hash"`      // TOP branch commit hash at last sync
	MergedChanges []Change  `json:"merged_changes"` // PRs that have been merged on GitHub
}
