package stack

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/ui"
)

type NativeAction string

const (
	NativeNoop     NativeAction = "noop"
	NativeCreate   NativeAction = "create"
	NativeAppend   NativeAction = "append"
	NativeRecreate NativeAction = "recreate"
	NativeDissolve NativeAction = "dissolve"
)

type desiredNativePR struct {
	Number int
	Base   string
}

type NativePlan struct {
	Action NativeAction
	Stack  *gh.NativeStack
	Append []int
}

func classifyNative(desired []desiredNativePR, remote *gh.NativeStack, details map[int]gh.PRDetails) NativePlan {
	if remote == nil {
		if len(desired) >= 2 {
			return NativePlan{Action: NativeCreate}
		}
		return NativePlan{Action: NativeNoop}
	}
	if len(desired) == 1 {
		return NativePlan{Action: NativeDissolve, Stack: remote}
	}

	remoteNumbers := make([]int, len(remote.PullRequests))
	for i, pr := range remote.PullRequests {
		remoteNumbers[i] = pr.Number
	}
	desiredNumbers := make([]int, len(desired))
	prefixBasesMatch := remote.Base.Ref == desired[0].Base
	for i, pr := range desired {
		desiredNumbers[i] = pr.Number
		if i < len(remoteNumbers) {
			if detail, ok := details[pr.Number]; ok && detail.BaseRef != pr.Base {
				prefixBasesMatch = false
			}
		}
	}
	if slices.Equal(remoteNumbers, desiredNumbers) && prefixBasesMatch {
		return NativePlan{Action: NativeNoop, Stack: remote}
	}
	if len(remoteNumbers) < len(desiredNumbers) && slices.Equal(remoteNumbers, desiredNumbers[:len(remoteNumbers)]) && prefixBasesMatch {
		return NativePlan{Action: NativeAppend, Stack: remote, Append: desiredNumbers[len(remoteNumbers):]}
	}
	return NativePlan{Action: NativeRecreate, Stack: remote}
}

func (c *Client) ReconcileNative(ctx context.Context, stackCtx *StackContext, dryRun bool) (NativePlan, error) {
	desired := make([]desiredNativePR, 0, len(stackCtx.AllChanges))
	known := make(map[int]bool)
	prData, err := c.LoadPRs(stackCtx.StackName)
	if err != nil {
		return NativePlan{}, fmt.Errorf("load local PR metadata: %w", err)
	}
	for _, pr := range prData.PRs {
		if pr != nil && pr.PRNumber > 0 {
			known[pr.PRNumber] = true
		}
	}
	for _, change := range stackCtx.AllChanges {
		if change.PR == nil || change.PR.PRNumber == 0 {
			continue
		}
		desired = append(desired, desiredNativePR{Number: change.PR.PRNumber, Base: change.DesiredBase})
		known[change.PR.PRNumber] = true
	}
	if len(desired) == 0 {
		return NativePlan{Action: NativeNoop}, nil
	}

	stackByNumber := make(map[int]gh.NativeStack)
	for _, pr := range desired {
		stacks, err := c.gh.FindStacks(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, pr.Number)
		if err != nil {
			return NativePlan{}, fmt.Errorf("discover GitHub-native stack for PR #%d: %w", pr.Number, err)
		}
		for _, native := range stacks {
			stackByNumber[native.Number] = native
		}
	}
	if len(stackByNumber) > 1 {
		return NativePlan{}, fmt.Errorf("local PRs belong to multiple GitHub-native stacks; unstack them on GitHub before retrying")
	}
	var remote *gh.NativeStack
	for _, native := range stackByNumber {
		native := native
		remote = &native
	}
	if remote != nil {
		for _, pr := range remote.PullRequests {
			if !known[pr.Number] {
				return NativePlan{}, fmt.Errorf("GitHub-native stack #%d contains unknown PR #%d; refusing to modify it", remote.Number, pr.Number)
			}
		}
	}

	needed := make(map[int]bool)
	for _, pr := range desired {
		needed[pr.Number] = true
	}
	if remote != nil {
		for _, pr := range remote.PullRequests {
			needed[pr.Number] = true
		}
	}
	numbers := make([]int, 0, len(needed))
	for number := range needed {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	details, _, err := c.gh.GetPRDetails(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, numbers)
	if err != nil {
		return NativePlan{}, fmt.Errorf("read GitHub pull request structure: %w", err)
	}
	for _, number := range numbers {
		if details[number].Number == 0 {
			return NativePlan{}, fmt.Errorf("PR #%d is missing on GitHub", number)
		}
	}
	plan := classifyNative(desired, remote, details)
	if dryRun {
		ui.Infof("GitHub-native stack: would %s", plan.Action)
		return plan, nil
	}

	owner, repo := stackCtx.Stack.Owner, stackCtx.Stack.RepoName
	if plan.Action == NativeRecreate || plan.Action == NativeDissolve {
		for _, member := range remote.PullRequests {
			detail := details[member.Number]
			if detail.Merged || !strings.EqualFold(detail.State, "OPEN") || detail.Queued || detail.AutoMerge {
				return plan, fmt.Errorf("PR #%d is merged, closed, queued, or has auto-merge enabled; GitHub-native stack #%d cannot be safely recreated", member.Number, remote.Number)
			}
		}
		status, body, err := c.gh.Unstack(ctx, owner, repo, remote.Number)
		if err != nil {
			return plan, fmt.Errorf("unstack GitHub-native stack #%d: %w", remote.Number, err)
		}
		if status != 204 {
			return plan, fmt.Errorf("GitHub only partially unstacked stack #%d (HTTP %d: %s). Resolve its locked PRs on GitHub, then rerun 'stack push'", remote.Number, status, strings.TrimSpace(string(body)))
		}
	}

	if plan.Action == NativeCreate || plan.Action == NativeRecreate || plan.Action == NativeDissolve || plan.Action == NativeAppend {
		start := 0
		if plan.Action == NativeAppend {
			start = len(desired) - len(plan.Append)
		}
		for _, pr := range desired[start:] {
			if details[pr.Number].BaseRef == pr.Base {
				continue
			}
			if err := c.gh.UpdatePRBase(ctx, owner, repo, pr.Number, pr.Base); err != nil {
				return plan, fmt.Errorf("set base of PR #%d to %s: %w; rerun 'stack push' to resume", pr.Number, pr.Base, err)
			}
			if change := findChangeByPR(stackCtx, pr.Number); change != nil {
				change.PR.Base = pr.Base
			}
			if err := stackCtx.Save(); err != nil {
				return plan, fmt.Errorf("persist base update for PR #%d: %w", pr.Number, err)
			}
		}
	}

	switch plan.Action {
	case NativeCreate, NativeRecreate:
		prs := make([]int, len(desired))
		for i := range desired {
			prs[i] = desired[i].Number
		}
		if err := c.gh.CreateStack(ctx, owner, repo, prs); err != nil {
			return plan, fmt.Errorf("create GitHub-native stack: %w", err)
		}
	case NativeAppend:
		if err := c.gh.AddToStack(ctx, owner, repo, remote.Number, plan.Append); err != nil {
			return plan, fmt.Errorf("append GitHub-native stack #%d: %w", remote.Number, err)
		}
	}
	return plan, nil
}

func findChangeByPR(stackCtx *StackContext, number int) *model.Change {
	for _, change := range stackCtx.AllChanges {
		if change.PR != nil && change.PR.PRNumber == number {
			return change
		}
	}
	return nil
}
