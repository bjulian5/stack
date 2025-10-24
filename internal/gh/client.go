package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) SyncPR(spec PRSpec) (*PR, error) {
	var existingPR *PR
	var err error

	if spec.Number > 0 {
		existingPR, err = c.getPRByNumber(spec.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to query PR #%d: %w", spec.Number, err)
		}
	} else {
		existingPR, err = c.getPRByHead(spec.Head)
		if err != nil {
			return nil, fmt.Errorf("failed to query PR by head branch: %w", err)
		}
	}

	if existingPR == nil {
		return c.createPR(spec)
	}

	if strings.ToLower(existingPR.State) == "merged" {
		return nil, fmt.Errorf(
			"PR #%d is already merged. Run 'stack refresh' to sync merged PRs",
			existingPR.Number,
		)
	}

	spec.Number = existingPR.Number
	return c.updatePR(spec, existingPR)
}

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

	output, err := c.execGH(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	prNumber, err := extractPRNumber(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to extract PR number from output: %w", err)
	}

	pr, err := c.getPRByNumber(prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created PR details: %w", err)
	}

	return pr, nil
}

func extractPRNumber(output string) (int, error) {
	re := regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/(\d+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return 0, fmt.Errorf("no PR URL found in output: %s", output)
	}

	var prNumber int
	if _, err := fmt.Sscanf(matches[1], "%d", &prNumber); err != nil {
		return 0, fmt.Errorf("failed to parse PR number: %w", err)
	}

	return prNumber, nil
}

func (c *Client) updatePR(spec PRSpec, currentPR *PR) (*PR, error) {
	prNumber := fmt.Sprintf("%d", spec.Number)

	editArgs := []string{
		"pr", "edit", prNumber,
		"--title", spec.Title,
		"--body", spec.Body,
		"--base", spec.Base,
	}

	if _, err := c.execGH(editArgs...); err != nil {
		return nil, fmt.Errorf("failed to update PR: %w", err)
	}

	if spec.Draft && !currentPR.IsDraft {
		if _, err := c.execGH("pr", "ready", prNumber, "--undo"); err != nil {
			return nil, fmt.Errorf("failed to mark PR as draft: %w", err)
		}
	} else if !spec.Draft && currentPR.IsDraft {
		if _, err := c.execGH("pr", "ready", prNumber); err != nil {
			return nil, fmt.Errorf("failed to mark PR as ready: %w", err)
		}
	}

	return &PR{
		Number:    spec.Number,
		URL:       currentPR.URL,
		State:     normalizeState("OPEN", spec.Draft),
		IsDraft:   spec.Draft,
		CreatedAt: currentPR.CreatedAt,
		UpdatedAt: time.Now(),
	}, nil
}

type prJSON struct {
	Number    int       `json:"number"`
	URL       string    `json:"url"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"isDraft"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

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

func (c *Client) parsePRJSON(data []byte) (*PR, error) {
	var ghPR prJSON
	if err := json.Unmarshal(data, &ghPR); err != nil {
		return nil, fmt.Errorf("failed to parse PR JSON: %w", err)
	}
	return ghPR.toPR(), nil
}

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
		return nil, nil
	}

	return prs[0].toPR(), nil
}

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

func normalizeState(state string, isDraft bool) string {
	state = strings.ToLower(state)
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
	IsDraft  bool      // True if PR is a draft
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
			isDraft
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
			IsDraft  bool      `json:"isDraft"`
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
			IsDraft:  pr.IsDraft,
		}
	}

	return &BatchPRsResult{PRStates: prStates}, nil
}

type Comment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
	URL  string `json:"url"`
}

func (c *Client) ListPRComments(prNumber int) ([]Comment, error) {
	output, err := c.execGH(
		"pr", "view", fmt.Sprintf("%d", prNumber),
		"--comments",
		"--json", "comments",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list PR comments: %w", err)
	}

	var response struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse comments: %w", err)
	}

	return response.Comments, nil
}

func (c *Client) CreatePRComment(prNumber int, body string) (string, error) {
	_, err := c.execGH(
		"pr", "comment", fmt.Sprintf("%d", prNumber),
		"--body", body,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create PR comment: %w", err)
	}

	comments, err := c.ListPRComments(prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to fetch comment ID after creation: %w", err)
	}

	if len(comments) == 0 {
		return "", fmt.Errorf("no comments found after creating comment")
	}

	return comments[len(comments)-1].ID, nil
}

func (c *Client) UpdatePRComment(commentID string, body string) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal comment body: %w", err)
	}

	query := fmt.Sprintf(`
		mutation {
			updateIssueComment(input: {id: "%s", body: %s}) {
				issueComment {
					id
				}
			}
		}
	`, commentID, string(bodyJSON))

	_, err = c.execGH("api", "graphql", "-f", "query="+query)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	return nil
}

// MarkPRReady marks a PR as ready for review (not draft)
func (c *Client) MarkPRReady(prNumber int) error {
	_, err := c.execGH("pr", "ready", fmt.Sprintf("%d", prNumber))
	if err != nil {
		return fmt.Errorf("failed to mark PR as ready: %w", err)
	}
	return nil
}

// MarkPRDraft marks a PR as draft (not ready for review)
func (c *Client) MarkPRDraft(prNumber int) error {
	_, err := c.execGH("pr", "ready", fmt.Sprintf("%d", prNumber), "--undo")
	if err != nil {
		return fmt.Errorf("failed to mark PR as draft: %w", err)
	}
	return nil
}
