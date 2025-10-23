package stack

import (
	"time"

	"github.com/bjulian5/stack/internal/gh"
)

// PR represents a pull request in the stack
type PR struct {
	PRNumber     int       `json:"pr_number"`
	URL          string    `json:"url"`
	Branch       string    `json:"branch"`
	CommitHash   string    `json:"commit_hash"`              // Latest commit hash for this PR
	VizCommentID string    `json:"viz_comment_id,omitempty"` // GitHub comment ID for stack visualization
	CreatedAt    time.Time `json:"created_at"`
	LastPushed   time.Time `json:"last_pushed"`
	State        string    `json:"state"` // open, draft, closed, merged (actual GitHub state)

	// Cached PR metadata for diff-based updates (avoids redundant API calls)
	Title string `json:"title,omitempty"` // Last pushed PR title
	Body  string `json:"body,omitempty"`  // Last pushed PR description
	Base  string `json:"base,omitempty"`  // Last pushed base branch

	// LocalDraftStatus is the user's desired draft state (true = draft, false = ready)
	// This is set by 'stack ready' and 'stack draft' commands.
	// Defaults to true for new changes.
	LocalDraftStatus bool `json:"local_draft"`

	// RemoteDraftStatus is the current draft state on GitHub (true = draft, false = ready)
	// This is synced from GitHub API during SyncPRMetadata.
	// When LocalDraftStatus differs from RemoteDraftStatus, the PR needs to be synced.
	RemoteDraftStatus bool `json:"remote_draft_status"`
}

// PRSyncData contains all data needed to sync a PR to local storage
type PRSyncData struct {
	StackName         string
	UUID              string
	Branch            string
	CommitHash        string
	GitHubPR          *gh.PR
	Title             string
	Body              string
	Base              string
	RemoteDraftStatus bool // The draft status that was pushed to GitHub
}

// PRData is a wrapper for PR tracking data
// This structure allows for easier evolution of the JSON format
type PRData struct {
	Version int            `json:"version"` // Format version (currently 1)
	PRs     map[string]*PR `json:"prs"`
}
