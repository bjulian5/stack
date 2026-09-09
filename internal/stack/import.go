package stack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bjulian5/stack/internal/model"
)

type ImportOptions struct {
	Branch string
	Base   string
}

// ImportStack reconstructs local stack state from any branch belonging to a pushed stack.
func (c *Client) ImportStack(ctx context.Context, opts ImportOptions) (*StackContext, error) {
	remote, err := c.git.GetRemoteName()
	if err != nil {
		return nil, err
	}
	if err := c.git.Fetch(remote); err != nil {
		return nil, err
	}

	branch, ref, err := c.resolveImportBranch(opts.Branch, remote)
	if err != nil {
		return nil, err
	}
	stackName := extractStackName(branch)
	if c.StackExists(stackName) {
		return nil, fmt.Errorf("stack '%s' already exists locally", stackName)
	}

	selectedCommit, err := c.git.GetCommit(ref)
	if err != nil {
		return nil, fmt.Errorf("read branch %s: %w", opts.Branch, err)
	}
	selectedUUID := selectedCommit.Message.Trailers["PR-UUID"]
	if selectedUUID == "" || selectedCommit.Message.Trailers["PR-Stack"] != stackName {
		return nil, fmt.Errorf("branch %s does not point to a commit in stack '%s'", opts.Branch, stackName)
	}
	owner, _, _ := strings.Cut(branch, "/")
	selectedHead := fmt.Sprintf("%s/stack-%s/%s", owner, stackName, selectedUUID)
	selectedPR, err := c.gh.FindPRByHead(selectedHead)
	if err != nil {
		return nil, fmt.Errorf("find pull request for branch %s: %w", selectedHead, err)
	}
	if selectedPR == nil {
		return nil, fmt.Errorf("no pull request found for branch %s", selectedHead)
	}

	repoOwner, repoName, err := c.gh.GetRepoInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info: %w", err)
	}
	nativeStacks, err := c.gh.FindStacks(ctx, repoOwner, repoName, selectedPR.Number)
	if err != nil {
		return nil, fmt.Errorf("discover GitHub-native stack for PR #%d: %w", selectedPR.Number, err)
	}
	if len(nativeStacks) > 1 {
		return nil, fmt.Errorf("PR #%d belongs to multiple GitHub-native stacks", selectedPR.Number)
	}

	numbers := []int{selectedPR.Number}
	base := opts.Base
	if len(nativeStacks) == 1 {
		if len(nativeStacks[0].PullRequests) == 0 {
			return nil, fmt.Errorf("GitHub-native stack for PR #%d has no members", selectedPR.Number)
		}
		numbers = make([]int, len(nativeStacks[0].PullRequests))
		for i, pr := range nativeStacks[0].PullRequests {
			numbers[i] = pr.Number
		}
		if base == "" {
			base = nativeStacks[0].Base.Ref
		}
	}
	details, _, err := c.gh.GetPRDetails(ctx, repoOwner, repoName, numbers)
	if err != nil {
		return nil, fmt.Errorf("read GitHub pull request structure: %w", err)
	}
	if base == "" {
		base = details[numbers[0]].BaseRef
	}
	if base == "" {
		return nil, fmt.Errorf("could not determine the stack base; pass --base explicitly")
	}

	prData := &model.PRData{Version: 1, PRs: make(map[string]*model.PR, len(numbers))}
	var baseRef, previousHash, topHash string
	created := time.Now()
	for i, number := range numbers {
		detail := details[number]
		if detail.Number == 0 {
			return nil, fmt.Errorf("PR #%d is missing on GitHub", number)
		}
		if extractStackName(detail.HeadRef) != stackName || !strings.HasPrefix(detail.HeadRef, owner+"/") {
			return nil, fmt.Errorf("PR #%d head %q does not belong to stack '%s'", number, detail.HeadRef, stackName)
		}
		_, uuid := extractUUIDFromBranch(detail.HeadRef)
		commit, err := c.git.GetCommit(detail.HeadSHA)
		if err != nil {
			return nil, fmt.Errorf("read commit for PR #%d: %w", number, err)
		}
		if commit.Message.Trailers["PR-UUID"] != uuid || commit.Message.Trailers["PR-Stack"] != stackName {
			return nil, fmt.Errorf("PR #%d head commit trailers do not match branch %s", number, detail.HeadRef)
		}
		parent, err := c.git.GetParentCommit(detail.HeadSHA)
		if err != nil {
			return nil, fmt.Errorf("read parent for PR #%d: %w", number, err)
		}
		if i == 0 {
			baseRef = parent
		} else if parent != previousHash {
			return nil, fmt.Errorf("PR #%d does not directly follow PR #%d in local git history", number, numbers[i-1])
		}
		state := strings.ToLower(detail.State)
		if detail.Draft && state == "open" {
			state = "draft"
		}
		prData.PRs[uuid] = &model.PR{
			PRNumber: number, URL: detail.URL, Branch: detail.HeadRef, CommitHash: detail.HeadSHA,
			CreatedAt: detail.CreatedAt, LastPushed: detail.UpdatedAt, State: state,
			Title: detail.Title, Body: detail.Body, Base: detail.BaseRef,
			LocalDraftStatus: detail.Draft, RemoteDraftStatus: detail.Draft,
		}
		if !detail.CreatedAt.IsZero() && detail.CreatedAt.Before(created) {
			created = detail.CreatedAt
		}
		previousHash = detail.HeadSHA
		topHash = detail.HeadSHA
	}

	topBranch := formatStackBranch(owner, stackName)
	if c.git.BranchExists(topBranch) {
		existingHash, err := c.git.GetCommitHash(topBranch)
		if err != nil || existingHash != topHash {
			return nil, fmt.Errorf("local branch %s already exists at a different commit", topBranch)
		}
		if err := c.git.CheckoutBranch(topBranch); err != nil {
			return nil, fmt.Errorf("checkout imported stack: %w", err)
		}
	} else if err := c.git.CreateAndCheckoutBranchAt(topBranch, topHash); err != nil {
		return nil, fmt.Errorf("create imported stack branch: %w", err)
	}

	stack := &model.Stack{
		Name: stackName, Branch: topBranch, Base: base, Owner: repoOwner, RepoName: repoName,
		Created: created, LastSynced: time.Now(), SyncHash: topHash, BaseRef: baseRef,
		MergedChanges: []model.Change{},
	}
	if err := c.SaveStack(stack); err != nil {
		return nil, fmt.Errorf("save imported stack: %w", err)
	}
	if err := c.savePRs(stackName, prData); err != nil {
		return nil, fmt.Errorf("save imported PR metadata: %w", err)
	}
	return c.GetStackContextByName(stackName)
}

func (c *Client) resolveImportBranch(input, remote string) (branch, ref string, err error) {
	input = strings.TrimPrefix(input, "refs/heads/")
	if strings.HasPrefix(input, "refs/remotes/") {
		input = strings.TrimPrefix(input, "refs/remotes/")
	}
	remotePrefix := remote + "/"
	branch = strings.TrimPrefix(input, remotePrefix)
	if !IsStackBranch(branch) {
		return "", "", fmt.Errorf("%q is not a stack branch", input)
	}
	ref = input
	if _, err := c.git.ResolveCommitHash(ref); err == nil {
		return branch, ref, nil
	}
	ref = remotePrefix + branch
	if _, err := c.git.ResolveCommitHash(ref); err != nil {
		return "", "", fmt.Errorf("stack branch %s was not found locally or on %s", branch, remote)
	}
	return branch, ref, nil
}
