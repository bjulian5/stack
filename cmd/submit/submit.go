package submit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bjulian5/stack/internal/common"
	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

type Command struct {
	DryRun, Merge, Squash, Rebase bool
	MergeMethod                   string
	Git                           *git.Client
	Stack                         *stack.Client
	GH                            GithubClient
	PollInterval, PollTimeout     time.Duration
}

type GithubClient interface {
	GetPRDetails(ctx context.Context, owner, repo string, numbers []int) (map[int]gh.PRDetails, gh.RepoMergeConfig, error)
	StartMerge(ctx context.Context, owner, repo string, pr int, request gh.MergeRequest) (gh.MergeResult, int, error)
	PollMerge(ctx context.Context, owner, repo string, pr int, uuid string) (gh.MergeResult, error)
}

func (c *Command) Register(parent *cobra.Command) {
	command := &cobra.Command{
		Use: "submit", Short: "Merge or enqueue the entire stack",
		Long: "Validate the local stack and submit its top pull request to GitHub's asynchronous stack merge endpoint.",
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			c.Git, c.GH, c.Stack, err = common.InitClients()
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return c.Run(cmd.Context()) },
	}
	command.Flags().BoolVar(&c.DryRun, "dry-run", false, "Validate without submitting")
	command.Flags().BoolP("yes", "y", false, "Accepted for compatibility; submission does not prompt")
	command.Flags().BoolVar(&c.Merge, "merge", false, "Use a merge commit")
	command.Flags().BoolVar(&c.Squash, "squash", false, "Squash and merge")
	command.Flags().BoolVar(&c.Rebase, "rebase", false, "Rebase and merge")
	command.Flags().StringVar(&c.MergeMethod, "merge-method", "", "Merge method: merge, squash, or rebase")
	command.MarkFlagsMutuallyExclusive("merge", "squash", "rebase", "merge-method")
	parent.AddCommand(command)
}

func (c *Command) Run(ctx context.Context) error {
	stackCtx, err := c.Stack.GetStackContext()
	if err != nil {
		return err
	}
	if !stackCtx.IsStack() {
		return fmt.Errorf("not on a stack branch. Use 'stack switch' to switch to a stack")
	}
	if dirty, err := c.Git.HasUncommittedChanges(); err != nil {
		return err
	} else if dirty {
		return fmt.Errorf("working tree has uncommitted changes; commit them before submitting")
	}
	if len(stackCtx.ActiveChanges) == 0 {
		return fmt.Errorf("stack has no unmerged pull requests")
	}

	numbers := make([]int, 0, len(stackCtx.AllChanges))
	for _, change := range stackCtx.AllChanges {
		if change.PR == nil || change.PR.PRNumber == 0 {
			return fmt.Errorf("%q has no pull request; run 'stack push'", change.Title)
		}
		numbers = append(numbers, change.PR.PRNumber)
	}
	details, config, err := c.GH.GetPRDetails(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, numbers)
	if err != nil {
		return fmt.Errorf("read GitHub pull requests: %w", err)
	}
	for _, change := range stackCtx.AllChanges {
		pr := details[change.PR.PRNumber]
		if pr.Number == 0 {
			return fmt.Errorf("PR #%d is missing on GitHub", change.PR.PRNumber)
		}
		if pr.HeadRef != stackCtx.FormatUUIDBranch(change.UUID) || pr.HeadSHA != change.CommitHash || pr.BaseRef != change.DesiredBase {
			return fmt.Errorf("PR #%d does not match local branch, SHA, or base; run 'stack push'", pr.Number)
		}
		if !change.PR.IsMerged() && change.NeedsSyncToGitHub().NeedsSync {
			return fmt.Errorf("PR #%d metadata is stale; run 'stack push'", pr.Number)
		}
	}
	for _, change := range stackCtx.ActiveChanges {
		pr := details[change.PR.PRNumber]
		if !strings.EqualFold(pr.State, "OPEN") || pr.Draft {
			return fmt.Errorf("PR #%d must be open and ready for review", pr.Number)
		}
	}
	plan, err := c.Stack.ReconcileNative(ctx, stackCtx, true)
	if err != nil {
		return err
	}
	if plan.Action != stack.NativeNoop {
		return fmt.Errorf("GitHub-native membership is stale (%s); run 'stack push'", plan.Action)
	}

	top := stackCtx.ActiveChanges[len(stackCtx.ActiveChanges)-1]
	topDetails := details[top.PR.PRNumber]
	action := "direct_merge"
	method := ""
	if topDetails.HasMergeQueue {
		action = "merge_queue"
		if c.requestedMethod() != "" {
			ui.Warning("The base branch uses a merge queue; GitHub chooses its merge method and the override is ignored.")
		}
	} else if method, err = c.mergeMethod(config); err != nil {
		return err
	}

	ui.Info("Stack submission order:")
	for _, change := range stackCtx.ActiveChanges {
		ui.Printf("  PR #%d  %s\n", change.PR.PRNumber, change.Title)
	}
	ui.Println("")
	ui.Info("GitHub evaluates checks, approvals, rules, and the aggregate stack verdict only after submission.")
	if c.DryRun {
		ui.Success("Stack is structurally ready to submit")
		return nil
	}

	result, status, startErr := c.GH.StartMerge(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, top.PR.PRNumber,
		gh.MergeRequest{SHA: topDetails.HeadSHA, Method: method, Action: action})
	uuid, terminal, err := mergeStart(result, status, startErr)
	if err != nil {
		return err
	}
	if terminal {
		return reportResult(result, action)
	}
	return c.poll(ctx, stackCtx.Stack.Owner, stackCtx.Stack.RepoName, top.PR.PRNumber, uuid, action)
}
func mergeStart(result gh.MergeResult, status int, err error) (string, bool, error) {
	if err != nil && gh.APIStatus(err) != 409 {
		return "", false, err
	}
	if status == 200 && result.Status != "pending" {
		return "", true, nil
	}
	if result.Details.UUID == "" {
		return "", false, fmt.Errorf("GitHub returned HTTP %d without an asynchronous merge UUID", status)
	}
	return result.Details.UUID, false, nil
}

func (c *Command) requestedMethod() string {
	if c.Merge {
		return "merge"
	}
	if c.Squash {
		return "squash"
	}
	if c.Rebase {
		return "rebase"
	}
	return strings.ToLower(c.MergeMethod)
}

func (c *Command) mergeMethod(config gh.RepoMergeConfig) (string, error) {
	method := c.requestedMethod()
	if method == "" {
		method = strings.ToLower(config.ViewerDefaultMergeMethod)
	}
	allowed := map[string]bool{"merge": config.MergeCommitAllowed, "squash": config.SquashMergeAllowed, "rebase": config.RebaseMergeAllowed}
	if _, ok := allowed[method]; !ok {
		return "", fmt.Errorf("invalid merge method %q; use merge, squash, or rebase", method)
	}
	if !allowed[method] {
		return "", fmt.Errorf("merge method %q is disabled for this repository", method)
	}
	return method, nil
}

func (c *Command) poll(ctx context.Context, owner, repo string, pr int, uuid, action string) error {
	interval := c.PollInterval
	if interval == 0 {
		interval = time.Second
	}
	timeout := c.PollTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	timer, ticker := time.NewTimer(timeout), time.NewTicker(interval)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return c.recoveryError(owner, repo, pr, uuid, "submission interrupted")
		case <-timer.C:
			return c.recoveryError(owner, repo, pr, uuid, "submission is still running after 10 minutes")
		case <-ticker.C:
			result, err := c.GH.PollMerge(ctx, owner, repo, pr, uuid)
			if err != nil {
				if ctx.Err() != nil {
					return c.recoveryError(owner, repo, pr, uuid, "submission interrupted")
				}
				return err
			}
			if result.Status != "pending" {
				return reportResult(result, action)
			}
		}
	}
}

func (c *Command) recoveryError(owner, repo string, pr int, uuid, message string) error {
	return fmt.Errorf("%s. Request UUID: %s\nPoll with: gh api -H 'X-GitHub-Api-Version: 2026-03-10' /repos/%s/%s/pulls/%d/merge-async/%s", message, uuid, owner, repo, pr, uuid)
}

func reportResult(result gh.MergeResult, action string) error {
	switch result.Status {
	case "merged":
		ui.Success("Stack merged")
		return nil
	case "enqueued":
		ui.Success("Stack enqueued in the merge queue")
		return nil
	case "failed":
		return fmt.Errorf("GitHub stack submission failed: %s", result.Details.Message)
	default:
		if action == "merge_queue" && result.Status == "queued" {
			ui.Success("Stack enqueued in the merge queue")
			return nil
		}
		return fmt.Errorf("GitHub returned unknown merge status %q: %s", result.Status, result.Details.Message)
	}
}
