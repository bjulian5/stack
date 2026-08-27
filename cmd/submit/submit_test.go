package submit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
)

type fakeGithub struct {
	results []gh.MergeResult
	err     error
	calls   int
}

func (f *fakeGithub) GetPRDetails(context.Context, string, string, []int) (map[int]gh.PRDetails, gh.RepoMergeConfig, error) {
	return nil, gh.RepoMergeConfig{}, nil
}
func (f *fakeGithub) StartMerge(context.Context, string, string, int, gh.MergeRequest) (gh.MergeResult, int, error) {
	return gh.MergeResult{}, 0, nil
}
func (f *fakeGithub) PollMerge(context.Context, string, string, int, string) (gh.MergeResult, error) {
	if f.err != nil {
		return gh.MergeResult{}, f.err
	}
	result := f.results[f.calls]
	if f.calls < len(f.results)-1 {
		f.calls++
	}
	return result, nil
}

func mergeResult(status, message string) gh.MergeResult {
	var result gh.MergeResult
	result.Status, result.Details.Message = status, message
	return result
}

func TestMergeMethod(t *testing.T) {
	config := gh.RepoMergeConfig{ViewerDefaultMergeMethod: "SQUASH", MergeCommitAllowed: true, SquashMergeAllowed: true}
	tests := []struct {
		name    string
		command Command
		want    string
		wantErr string
	}{
		{"repository default", Command{}, "squash", ""},
		{"short override", Command{Merge: true}, "merge", ""},
		{"long override", Command{MergeMethod: "merge"}, "merge", ""},
		{"disabled", Command{Rebase: true}, "", "disabled"},
		{"invalid", Command{MergeMethod: "octopus"}, "", "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.command.mergeMethod(config)
			assert.Equal(t, tt.want, got)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChangesThrough(t *testing.T) {
	changes := []*model.Change{
		{CommitHash: "aaa", PR: &model.PR{PRNumber: 10}},
		{CommitHash: "bbb", PR: &model.PR{PRNumber: 11}},
		{CommitHash: "ccc", PR: &model.PR{PRNumber: 12}},
	}
	resolve := func(ref string) (string, error) {
		if ref == "HEAD~1" {
			return "bbb", nil
		}
		if ref == "main" {
			return "base", nil
		}
		return "", errors.New("unknown commit")
	}
	tests := []struct {
		name    string
		target  string
		want    []*model.Change
		wantErr error
	}{
		{name: "whole stack", want: changes},
		{name: "through PR", target: "11", want: changes[:2]},
		{name: "through commit", target: "HEAD~1", want: changes[:2]},
		{name: "PR not active", target: "99", wantErr: errors.New("PR #99 is not an active change")},
		{name: "commit not active", target: "main", wantErr: errors.New("commit main is not an active change")},
		{name: "invalid commit", target: "missing", wantErr: errors.New("unknown commit")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changesThrough(changes, tt.target, resolve)
			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMergeStart(t *testing.T) {
	pending := mergeResult("pending", "")
	pending.Details.UUID = "request-1"
	tests := []struct {
		name     string
		result   gh.MergeResult
		status   int
		err      error
		wantUUID string
		terminal bool
		wantErr  bool
	}{
		{"accepted", pending, 202, nil, "request-1", false, false},
		{"existing request", pending, 409, &gh.APIError{StatusCode: 409}, "request-1", false, false},
		{"already merged", mergeResult("merged", ""), 200, nil, "", true, false},
		{"missing uuid", mergeResult("pending", ""), 202, nil, "", false, true},
		{"request failure", gh.MergeResult{}, 422, errors.New("sha race"), "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, terminal, err := mergeStart(tt.result, tt.status, tt.err)
			assert.Equal(t, tt.wantUUID, uuid)
			assert.Equal(t, tt.terminal, terminal)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestPoll(t *testing.T) {
	t.Run("terminal merged", func(t *testing.T) {
		client := &fakeGithub{results: []gh.MergeResult{mergeResult("pending", ""), mergeResult("merged", "")}}
		command := &Command{GH: client, PollInterval: time.Millisecond, PollTimeout: time.Second}
		require.NoError(t, command.poll(t.Context(), "owner", "repo", 7, "request-1", "direct_merge"))
	})
	t.Run("terminal failure", func(t *testing.T) {
		client := &fakeGithub{results: []gh.MergeResult{mergeResult("failed", "checks failed")}}
		command := &Command{GH: client, PollInterval: time.Millisecond, PollTimeout: time.Second}
		assert.ErrorContains(t, command.poll(t.Context(), "owner", "repo", 7, "request-1", "direct_merge"), "checks failed")
	})
	t.Run("timeout recovery", func(t *testing.T) {
		client := &fakeGithub{results: []gh.MergeResult{mergeResult("pending", "")}}
		command := &Command{GH: client, PollInterval: time.Millisecond, PollTimeout: 3 * time.Millisecond}
		err := command.poll(t.Context(), "owner", "repo", 7, "request-1", "direct_merge")
		assert.ErrorContains(t, err, "Request UUID: request-1")
		assert.ErrorContains(t, err, "/repos/owner/repo/pulls/7/merge-async/request-1")
	})
	t.Run("cancellation recovery", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		command := &Command{GH: &fakeGithub{}, PollInterval: time.Millisecond, PollTimeout: time.Second}
		assert.ErrorContains(t, command.poll(ctx, "owner", "repo", 7, "request-1", "direct_merge"), "Request UUID: request-1")
	})
}
