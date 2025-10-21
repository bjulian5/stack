package gh

import "time"

// PRSpec defines all parameters for creating/updating a PR
type PRSpec struct {
	Number int    // 0 for new PR, >0 to update existing
	Title  string // PR title
	Body   string // PR description
	Base   string // base branch name
	Head   string // head branch name
	Draft  bool   // whether PR should be a draft
}

// PR contains GitHub PR information returned from gh CLI
type PR struct {
	Number    int       // PR number
	URL       string    // PR URL
	State     string    // "open", "closed", "merged"
	IsDraft   bool      // draft status
	CreatedAt time.Time // when PR was created
	UpdatedAt time.Time // when PR was last updated
}
