package stack

import (
	"time"
)

// PR represents a pull request in the stack
type PR struct {
	PRNumber   int       `json:"pr_number"`
	URL        string    `json:"url"`
	Branch     string    `json:"branch"`
	CommitHash string    `json:"commit_hash"` // Latest commit hash for this PR
	CreatedAt  time.Time `json:"created_at"`
	LastPushed time.Time `json:"last_pushed"`
	State      string    `json:"state"` // open, draft, closed, merged
}

// PRData is a wrapper for PR tracking data
// This structure allows for easier evolution of the JSON format
type PRData struct {
	Version int            `json:"version"` // Format version (currently 1)
	PRs     map[string]*PR `json:"prs"`
}
