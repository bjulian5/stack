package stack

import (
	"time"

	"github.com/bjulian5/stack/internal/model"
)

// Stack represents a PR stack
type Stack struct {
	Name          string         `json:"name"`
	Branch        string         `json:"branch"`
	Base          string         `json:"base"`
	Owner         string         `json:"owner"`     // GitHub repo owner (cached)
	RepoName      string         `json:"repo_name"` // GitHub repo name (cached)
	Created       time.Time      `json:"created"`
	LastSynced    time.Time      `json:"last_synced"`    // When we last checked GitHub for merged PRs
	SyncHash      string         `json:"sync_hash"`      // TOP branch commit hash at last sync
	MergedChanges []model.Change `json:"merged_changes"` // PRs that have been merged on GitHub
}
