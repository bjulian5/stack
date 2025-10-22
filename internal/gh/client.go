package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Client provides GitHub operations via gh CLI
type Client struct{}

// NewClient creates a new GitHub client
func NewClient() *Client {
	return &Client{}
}

// SyncPR creates or updates a PR on GitHub idempotently
// If spec.Number is 0, creates a new PR
// If spec.Number > 0, updates the existing PR
// Handles edge cases:
// - PR already exists on GitHub but not tracked locally (auto-recovers)
// - Closed PRs (reopens and updates)
// - Merged PRs (returns error - user should run stack refresh)
func (c *Client) SyncPR(spec PRSpec) (*PR, error) {
	if spec.Number == 0 {
		// Attempt to create new PR
		pr, err := c.createPR(spec)
		if err != nil {
			// Auto-recover: check if PR already exists on GitHub
			if isPRAlreadyExistsError(err) {
				// Query GitHub for existing PR by head branch
				existingPR, findErr := c.getPRByHead(spec.Head)
				if findErr == nil && existingPR != nil {
					// Found it! Update instead
					spec.Number = existingPR.Number
					return c.updatePR(spec)
				}
			}
			return nil, err // Other errors are fatal
		}
		return pr, nil
	}

	// Number provided - update existing PR
	return c.updatePR(spec)
}

// createPR creates a new PR on GitHub
func (c *Client) createPR(spec PRSpec) (*PR, error) {
	args := []string{
		"pr", "create",
		"--title", spec.Title,
		"--body", spec.Body,
		"--base", spec.Base,
		"--head", spec.Head,
	}

	if spec.Draft {
		args = append(args, "--draft")
	}

	// Create the PR (outputs URL to stdout)
	_, err := c.execGH(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	// Query GitHub for the newly created PR details
	pr, err := c.getPRByHead(spec.Head)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created PR details: %w", err)
	}
	if pr == nil {
		return nil, fmt.Errorf("PR was created but not found")
	}

	return pr, nil
}

// updatePR updates an existing PR on GitHub
func (c *Client) updatePR(spec PRSpec) (*PR, error) {
	prNumber := fmt.Sprintf("%d", spec.Number)

	// First check current PR state
	currentPR, err := c.getPRByNumber(spec.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR #%d: %w", spec.Number, err)
	}

	// Block updates to merged PRs (permanent state)
	if currentPR.State == "MERGED" {
		return nil, fmt.Errorf(
			"PR #%d is already merged. Run 'stack refresh' to sync merged PRs",
			spec.Number,
		)
	}

	// Allow updates to closed PRs (gh pr edit will reopen them)

	// Update PR title, body, and base
	editArgs := []string{
		"pr", "edit", prNumber,
		"--title", spec.Title,
		"--body", spec.Body,
		"--base", spec.Base,
	}

	if _, err := c.execGH(editArgs...); err != nil {
		return nil, fmt.Errorf("failed to update PR: %w", err)
	}

	// Handle draft/ready state separately (only if state needs to change)
	if spec.Draft && !currentPR.IsDraft {
		// Convert to draft: use "gh pr ready --undo"
		if _, err := c.execGH("pr", "ready", prNumber, "--undo"); err != nil {
			return nil, fmt.Errorf("failed to mark PR as draft: %w", err)
		}
	} else if !spec.Draft && currentPR.IsDraft {
		// Mark as ready for review
		if _, err := c.execGH("pr", "ready", prNumber); err != nil {
			return nil, fmt.Errorf("failed to mark PR as ready: %w", err)
		}
	}

	// Fetch and return updated PR data
	return c.getPRByNumber(spec.Number)
}

// prJSON is the common structure for PR data from gh CLI
type prJSON struct {
	Number    int       `json:"number"`
	URL       string    `json:"url"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"isDraft"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// toPR converts a prJSON to a PR
func (p *prJSON) toPR() *PR {
	return &PR{
		Number:    p.Number,
		URL:       p.URL,
		State:     normalizeState(p.State, p.IsDraft),
		IsDraft:   p.IsDraft,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// parsePRJSON parses PR data from gh CLI JSON output (single PR)
func (c *Client) parsePRJSON(data []byte) (*PR, error) {
	var ghPR prJSON
	if err := json.Unmarshal(data, &ghPR); err != nil {
		return nil, fmt.Errorf("failed to parse PR JSON: %w", err)
	}
	return ghPR.toPR(), nil
}

// execGH executes a gh CLI command and returns the output
func (c *Client) execGH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh CLI error: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute gh: %w", err)
	}
	return output, nil
}

// getPRByHead finds a PR by head branch name (private helper)
func (c *Client) getPRByHead(head string) (*PR, error) {
	output, err := c.execGH(
		"pr", "list",
		"--head", head,
		"--json", "number,url,state,isDraft,createdAt,updatedAt",
		"--limit", "1",
	)
	if err != nil {
		return nil, err
	}

	var prs []prJSON
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR list: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil // No PR found
	}

	return prs[0].toPR(), nil
}

// getPRByNumber fetches PR details by number (private helper)
func (c *Client) getPRByNumber(number int) (*PR, error) {
	output, err := c.execGH(
		"pr", "view", fmt.Sprintf("%d", number),
		"--json", "number,url,state,isDraft,createdAt,updatedAt",
	)
	if err != nil {
		return nil, err
	}

	return c.parsePRJSON(output)
}

// isPRAlreadyExistsError checks if error indicates PR already exists (private helper)
func isPRAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}

// normalizeState converts GitHub API state to our internal format
// GitHub returns: OPEN, CLOSED, MERGED (uppercase)
// We need: open, draft, closed, merged (lowercase, with draft derived from isDraft)
func normalizeState(state string, isDraft bool) string {
	// Convert to lowercase first
	state = strings.ToLower(state)

	// If PR is open and marked as draft, return "draft" instead of "open"
	if state == "open" && isDraft {
		return "draft"
	}

	return state
}

// OpenPR opens a pull request in the browser using gh CLI
func (c *Client) OpenPR(prNumber int) error {
	_, err := c.execGH("pr", "view", fmt.Sprintf("%d", prNumber), "--web")
	return err
}

// PRState contains the merge state of a pull request
type PRState struct {
	Number   int       // PR number
	State    string    // "OPEN", "CLOSED", "MERGED"
	IsMerged bool      // True if PR is merged
	MergedAt time.Time // When PR was merged (zero if not merged)
}

// GetPRState queries the merge state of a pull request from GitHub
func (c *Client) GetPRState(prNumber int) (*PRState, error) {
	output, err := c.execGH(
		"pr", "view", fmt.Sprintf("%d", prNumber),
		"--json", "number,state,mergedAt",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR state: %w", err)
	}

	// Parse the JSON response
	var response struct {
		Number   int       `json:"number"`
		State    string    `json:"state"` // "OPEN", "CLOSED", "MERGED"
		MergedAt time.Time `json:"mergedAt"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse PR state: %w", err)
	}

	state := &PRState{
		Number:   response.Number,
		State:    response.State,
		IsMerged: response.State == "MERGED",
		MergedAt: response.MergedAt,
	}

	return state, nil
}

// GetRepoInfo fetches the repository owner and name from GitHub
func (c *Client) GetRepoInfo() (owner, repoName string, err error) {
	output, err := c.execGH("repo", "view", "--json", "owner,name")
	if err != nil {
		return "", "", fmt.Errorf("failed to get repo info: %w", err)
	}

	var repo struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &repo); err != nil {
		return "", "", fmt.Errorf("failed to parse repo info: %w", err)
	}

	return repo.Owner.Login, repo.Name, nil
}

// BatchPRsResult contains results from bulk PR query
type BatchPRsResult struct {
	PRStates map[int]*PRState // Map of PR number to state
}

// BatchGetPRs fetches states for multiple PRs in a single GraphQL query.
// This is much more efficient than querying each PR individually.
func (c *Client) BatchGetPRs(owner, repoName string, prNumbers []int) (*BatchPRsResult, error) {
	if len(prNumbers) == 0 {
		return &BatchPRsResult{PRStates: make(map[int]*PRState)}, nil
	}

	// Build dynamic GraphQL query
	query := c.buildBatchPRQuery(prNumbers)

	// Execute GraphQL query
	output, err := c.execGH(
		"api", "graphql",
		"-f", "query="+query,
		"-f", "owner="+owner,
		"-f", "repo="+repoName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	// Parse response
	result, err := c.parseBatchPRResponse(output, prNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse batch PR response: %w", err)
	}

	return result, nil
}

// buildBatchPRQuery builds a GraphQL query to fetch multiple PRs
func (c *Client) buildBatchPRQuery(prNumbers []int) string {
	var sb strings.Builder
	sb.WriteString(`query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
`)

	fragment := `    pr%d: pullRequest(number: %d) {
      number
      state
      merged
      mergedAt
    }
`

	for _, num := range prNumbers {
		sb.WriteString(fmt.Sprintf(fragment, num, num))
	}

	sb.WriteString(`  }
}`)

	return sb.String()
}

// parseBatchPRResponse parses the GraphQL response into BatchPRsResult
func (c *Client) parseBatchPRResponse(data []byte, prNumbers []int) (*BatchPRsResult, error) {
	// Parse JSON response
	var response struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract PR data for each PR
	prStates := make(map[int]*PRState)

	for _, prNum := range prNumbers {
		key := fmt.Sprintf("pr%d", prNum)
		prData, exists := response.Data.Repository[key]
		if !exists {
			// PR doesn't exist or was deleted
			continue
		}

		var pr struct {
			Number   int       `json:"number"`
			State    string    `json:"state"` // "OPEN", "CLOSED", "MERGED"
			Merged   bool      `json:"merged"`
			MergedAt time.Time `json:"mergedAt"`
		}

		if err := json.Unmarshal(prData, &pr); err != nil {
			// Skip PRs that fail to parse
			continue
		}

		prStates[prNum] = &PRState{
			Number:   pr.Number,
			State:    pr.State,
			IsMerged: pr.Merged,
			MergedAt: pr.MergedAt,
		}
	}

	return &BatchPRsResult{PRStates: prStates}, nil
}
