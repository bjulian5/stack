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
	State        string    `json:"state"` // open, draft, closed, merged

	// Cached PR metadata for diff-based updates (avoids redundant API calls)
	Title string `json:"title,omitempty"` // Last pushed PR title
	Body  string `json:"body,omitempty"`  // Last pushed PR description
	Base  string `json:"base,omitempty"`  // Last pushed base branch
}

// PRCompareState represents the desired state of a PR for comparison
type PRCompareState struct {
	Title      string
	Body       string
	Base       string
	CommitHash string
	IsDraft    bool
}

// PRSyncData contains all data needed to sync a PR to local storage
type PRSyncData struct {
	StackName  string
	UUID       string
	Branch     string
	CommitHash string
	GitHubPR   *gh.PR
	Title      string
	Body       string
	Base       string
}

// NeedsUpdate checks if a PR needs to be updated on GitHub
func (p *PR) NeedsUpdate(desired PRCompareState) bool {
	if p.Title == "" || p.Body == "" || p.Base == "" {
		return true
	}

	if p.Title != desired.Title || p.Body != desired.Body || p.Base != desired.Base || p.CommitHash != desired.CommitHash {
		return true
	}

	cachedIsDraft := (p.State == "draft")
	return cachedIsDraft != desired.IsDraft
}

// WhyNeedsUpdate returns a human-readable reason why a PR needs updating (for debugging)
func (p *PR) WhyNeedsUpdate(desired PRCompareState) string {
	if p.Title == "" || p.Body == "" || p.Base == "" {
		return "metadata not cached"
	}

	if p.Title != desired.Title {
		return "title changed"
	}
	if p.Body != desired.Body {
		return "description changed"
	}
	if p.Base != desired.Base {
		return "base branch changed"
	}
	if p.CommitHash != desired.CommitHash {
		return "commit changed"
	}

	cachedIsDraft := (p.State == "draft")
	if cachedIsDraft != desired.IsDraft {
		return "draft status changed"
	}

	return ""
}

// PRData is a wrapper for PR tracking data
// This structure allows for easier evolution of the JSON format
type PRData struct {
	Version int            `json:"version"` // Format version (currently 1)
	PRs     map[string]*PR `json:"prs"`
}
