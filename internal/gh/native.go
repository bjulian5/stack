package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const apiVersion = "2026-03-10"

type apiResponse struct {
	Status int
	Body   []byte
	Stderr string
}

func (c *Client) api(ctx context.Context, method, path string, body any) (apiResponse, error) {
	args := []string{"api", "--include", "--method", method,
		"--header", "Accept: application/vnd.github+json",
		"--header", "X-GitHub-Api-Version: " + apiVersion, path}
	var input []byte
	if body != nil {
		var err error
		input, err = json.Marshal(body)
		if err != nil {
			return apiResponse{}, fmt.Errorf("marshal GitHub API request: %w", err)
		}
		args = append(args, "--input", "-")
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	response, parseErr := parseAPIResponse(stdout.Bytes(), stderr.String())
	if parseErr != nil {
		if runErr != nil {
			return apiResponse{}, fmt.Errorf("gh api: %w: %s", runErr, strings.TrimSpace(stderr.String()))
		}
		return apiResponse{}, parseErr
	}
	if runErr != nil || response.Status >= 400 {
		return response, &APIError{Method: method, Path: path, StatusCode: response.Status, Response: response}
	}
	return response, nil
}

func parseAPIResponse(output []byte, stderr string) (apiResponse, error) {
	text := strings.ReplaceAll(string(output), "\r\n", "\n")
	headerEnd := strings.Index(text, "\n\n")
	if headerEnd < 0 {
		return apiResponse{}, fmt.Errorf("GitHub API response did not include HTTP headers: %s", strings.TrimSpace(text))
	}
	statusLine := strings.SplitN(text[:headerEnd], "\n", 2)[0]
	fields := strings.Fields(statusLine)
	if len(fields) < 2 {
		return apiResponse{}, fmt.Errorf("invalid GitHub API status line %q", statusLine)
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		return apiResponse{}, fmt.Errorf("parse GitHub API status %q: %w", fields[1], err)
	}
	return apiResponse{Status: status, Body: []byte(text[headerEnd+2:]), Stderr: strings.TrimSpace(stderr)}, nil
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Response   apiResponse
}

func (e *APIError) Error() string {
	detail := strings.TrimSpace(string(e.Response.Body))
	if e.Response.Stderr != "" {
		detail = strings.TrimSpace(detail + " " + e.Response.Stderr)
	}
	return fmt.Sprintf("GitHub API %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, detail)
}

func APIStatus(err error) int {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode
	}
	return 0
}

type NativeStack struct {
	Number       int             `json:"number"`
	Base         NativeStackBase `json:"base"`
	PullRequests []NativeStackPR `json:"pull_requests"`
}

type NativeStackBase struct {
	Ref string `json:"ref"`
}
type NativeStackPR struct {
	Number int `json:"number"`
}

func repoPath(owner, repo string) string { return "/repos/" + owner + "/" + repo }

func (c *Client) FindStacks(ctx context.Context, owner, repo string, prNumber int) ([]NativeStack, error) {
	path := fmt.Sprintf("%s/stacks?pull_request=%d", repoPath(owner, repo), prNumber)
	response, err := c.api(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var stacks []NativeStack
	if err := json.Unmarshal(response.Body, &stacks); err != nil {
		return nil, fmt.Errorf("parse GitHub stacks: %w", err)
	}
	return stacks, nil
}

func (c *Client) CreateStack(ctx context.Context, owner, repo string, prs []int) error {
	_, err := c.api(ctx, "POST", repoPath(owner, repo)+"/stacks", map[string]any{"pull_requests": prs})
	return err
}

func (c *Client) AddToStack(ctx context.Context, owner, repo string, stackNumber int, prs []int) error {
	path := fmt.Sprintf("%s/stacks/%d/add", repoPath(owner, repo), stackNumber)
	_, err := c.api(ctx, "POST", path, map[string]any{"pull_requests": prs})
	return err
}

func (c *Client) Unstack(ctx context.Context, owner, repo string, stackNumber int) (int, []byte, error) {
	path := fmt.Sprintf("%s/stacks/%d/unstack", repoPath(owner, repo), stackNumber)
	response, err := c.api(ctx, "POST", path, nil)
	return response.Status, response.Body, err
}

func (c *Client) UpdatePRBase(ctx context.Context, owner, repo string, number int, base string) error {
	path := fmt.Sprintf("%s/pulls/%d", repoPath(owner, repo), number)
	_, err := c.api(ctx, "PATCH", path, map[string]string{"base": base})
	return err
}

type PRDetails struct {
	Number        int
	State         string
	Draft         bool
	Merged        bool
	HeadRef       string
	HeadSHA       string
	BaseRef       string
	Queued        bool
	AutoMerge     bool
	HasMergeQueue bool
}

type RepoMergeConfig struct {
	ViewerDefaultMergeMethod string
	MergeCommitAllowed       bool
	SquashMergeAllowed       bool
	RebaseMergeAllowed       bool
}

func (c *Client) GetPRDetails(ctx context.Context, owner, repo string, numbers []int) (map[int]PRDetails, RepoMergeConfig, error) {
	var fields strings.Builder
	for _, number := range numbers {
		fmt.Fprintf(&fields, "pr%d: pullRequest(number: %d) { number state isDraft merged headRefName headRefOid baseRefName mergeQueueEntry { id } autoMergeRequest { enabledAt } mergeQueue { id } }\n", number, number)
	}
	query := fmt.Sprintf("query($owner:String!,$repo:String!){repository(owner:$owner,name:$repo){viewerDefaultMergeMethod mergeCommitAllowed squashMergeAllowed rebaseMergeAllowed %s}}", fields.String())
	output, err := c.execGHContext(ctx, "api", "graphql", "-f", "query="+query, "-f", "owner="+owner, "-f", "repo="+repo)
	if err != nil {
		return nil, RepoMergeConfig{}, err
	}
	var raw struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, RepoMergeConfig{}, fmt.Errorf("parse pull request details: %w", err)
	}
	details := make(map[int]PRDetails, len(numbers))
	config := RepoMergeConfig{}
	for key, value := range raw.Data.Repository {
		switch key {
		case "viewerDefaultMergeMethod":
			_ = json.Unmarshal(value, &config.ViewerDefaultMergeMethod)
		case "mergeCommitAllowed":
			_ = json.Unmarshal(value, &config.MergeCommitAllowed)
		case "squashMergeAllowed":
			_ = json.Unmarshal(value, &config.SquashMergeAllowed)
		case "rebaseMergeAllowed":
			_ = json.Unmarshal(value, &config.RebaseMergeAllowed)
		default:
			var pr struct {
				Number           int             `json:"number"`
				State            string          `json:"state"`
				IsDraft          bool            `json:"isDraft"`
				Merged           bool            `json:"merged"`
				HeadRef          string          `json:"headRefName"`
				HeadSHA          string          `json:"headRefOid"`
				BaseRef          string          `json:"baseRefName"`
				MergeQueueEntry  json.RawMessage `json:"mergeQueueEntry"`
				AutoMergeRequest json.RawMessage `json:"autoMergeRequest"`
				MergeQueue       json.RawMessage `json:"mergeQueue"`
			}
			if err := json.Unmarshal(value, &pr); err != nil || pr.Number == 0 {
				continue
			}
			details[pr.Number] = PRDetails{Number: pr.Number, State: pr.State, Draft: pr.IsDraft, Merged: pr.Merged,
				HeadRef: pr.HeadRef, HeadSHA: pr.HeadSHA, BaseRef: pr.BaseRef,
				Queued: string(pr.MergeQueueEntry) != "null", AutoMerge: string(pr.AutoMergeRequest) != "null", HasMergeQueue: string(pr.MergeQueue) != "null"}
		}
	}
	return details, config, nil
}

type MergeRequest struct{ SHA, Method, Action string }
type MergeResult struct {
	Status  string `json:"status"`
	Details struct {
		Message string `json:"message"`
		UUID    string `json:"uuid"`
		SHA     string `json:"sha"`
	} `json:"details"`
}

func (c *Client) StartMerge(ctx context.Context, owner, repo string, pr int, request MergeRequest) (MergeResult, int, error) {
	body := map[string]string{"sha": request.SHA, "merge_action": request.Action}
	if request.Method != "" {
		body["merge_method"] = request.Method
	}
	path := fmt.Sprintf("%s/pulls/%d/merge-async", repoPath(owner, repo), pr)
	response, err := c.api(ctx, "PUT", path, body)
	var result MergeResult
	if unmarshalErr := json.Unmarshal(response.Body, &result); unmarshalErr != nil && len(response.Body) > 0 {
		return result, response.Status, fmt.Errorf("parse asynchronous merge response: %w", unmarshalErr)
	}
	return result, response.Status, err
}

func (c *Client) PollMerge(ctx context.Context, owner, repo string, pr int, uuid string) (MergeResult, error) {
	path := fmt.Sprintf("%s/pulls/%d/merge-async/%s", repoPath(owner, repo), pr, uuid)
	response, err := c.api(ctx, "GET", path, nil)
	if err != nil {
		return MergeResult{}, err
	}
	var result MergeResult
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return result, fmt.Errorf("parse asynchronous merge result: %w", err)
	}
	return result, nil
}
