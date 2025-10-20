package stack

import (
	"time"
)

// PR represents a pull request in the stack
type PR struct {
	PRNumber   int       `json:"pr_number"`
	URL        string    `json:"url"`
	Branch     string    `json:"branch"`
	CommitHash string    `json:"commit_hash"` // Current commit hash on the stack branch
	CreatedAt  time.Time `json:"created_at"`
	LastPushed time.Time `json:"last_pushed"`
	State      string    `json:"state"` // open, draft, closed, merged
}

// PRMap maps UUID to PR information
type PRMap map[string]*PR
