package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestClassifyNative(t *testing.T) {
	desired := []desiredNativePR{{Number: 1, Base: "main"}, {Number: 2, Base: "one"}, {Number: 3, Base: "two"}}
	details := map[int]gh.PRDetails{1: {BaseRef: "main"}, 2: {BaseRef: "one"}, 3: {BaseRef: "two"}}
	stack := func(base string, numbers ...int) *gh.NativeStack {
		prs := make([]gh.NativeStackPR, len(numbers))
		for i, number := range numbers {
			prs[i].Number = number
		}
		return &gh.NativeStack{Number: 9, Base: gh.NativeStackBase{Ref: base}, PullRequests: prs}
	}
	tests := []struct {
		name    string
		desired []desiredNativePR
		remote  *gh.NativeStack
		want    NativeAction
	}{
		{"create", desired, nil, NativeCreate},
		{"single stays unstacked", desired[:1], nil, NativeNoop},
		{"exact", desired, stack("main", 1, 2, 3), NativeNoop},
		{"append", desired, stack("main", 1, 2), NativeAppend},
		{"reorder", desired, stack("main", 2, 1, 3), NativeRecreate},
		{"removal", desired[:2], stack("main", 1, 2, 3), NativeRecreate},
		{"middle insertion", desired, stack("main", 1, 3), NativeRecreate},
		{"trunk change", desired, stack("develop", 1, 2, 3), NativeRecreate},
		{"dissolve", desired[:1], stack("main", 1, 2), NativeDissolve},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, classifyNative(tt.desired, tt.remote, details).Action) })
	}
}

func TestReconcileNative(t *testing.T) {
	tests := []struct {
		name    string
		changes []*model.Change
		setup   func(*gh.MockGithubClient)
		want    NativeAction
		wantErr string
	}{
		{
			name:    "registers unchanged local stack",
			changes: nativeChanges(1, 2),
			setup: func(m *gh.MockGithubClient) {
				m.On("FindStacks", mock.Anything, "owner", "repo", 1).Return([]gh.NativeStack{}, nil)
				m.On("FindStacks", mock.Anything, "owner", "repo", 2).Return([]gh.NativeStack{}, nil)
				m.On("GetPRDetails", mock.Anything, "owner", "repo", []int{1, 2}).Return(map[int]gh.PRDetails{1: {Number: 1, BaseRef: "main"}, 2: {Number: 2, BaseRef: "branch-1"}}, gh.RepoMergeConfig{}, nil)
				m.On("CreateStack", mock.Anything, "owner", "repo", []int{1, 2}).Return(nil)
			},
			want: NativeCreate,
		},
		{
			name:    "rejects locked recreation",
			changes: nativeChanges(1, 2),
			setup: func(m *gh.MockGithubClient) {
				remote := gh.NativeStack{Number: 9, Base: gh.NativeStackBase{Ref: "main"}, PullRequests: []gh.NativeStackPR{{Number: 2}, {Number: 1}}}
				m.On("FindStacks", mock.Anything, "owner", "repo", 1).Return([]gh.NativeStack{remote}, nil)
				m.On("FindStacks", mock.Anything, "owner", "repo", 2).Return([]gh.NativeStack{remote}, nil)
				m.On("GetPRDetails", mock.Anything, "owner", "repo", []int{1, 2}).Return(map[int]gh.PRDetails{1: {Number: 1, State: "OPEN", BaseRef: "main"}, 2: {Number: 2, State: "OPEN", BaseRef: "branch-1", Queued: true}}, gh.RepoMergeConfig{}, nil)
			},
			want: NativeRecreate, wantErr: "cannot be safely recreated",
		},
		{
			name:    "stops after partial unstack",
			changes: nativeChanges(1),
			setup: func(m *gh.MockGithubClient) {
				remote := gh.NativeStack{Number: 9, Base: gh.NativeStackBase{Ref: "main"}, PullRequests: []gh.NativeStackPR{{Number: 1}}}
				m.On("FindStacks", mock.Anything, "owner", "repo", 1).Return([]gh.NativeStack{remote}, nil)
				m.On("GetPRDetails", mock.Anything, "owner", "repo", []int{1}).Return(map[int]gh.PRDetails{1: {Number: 1, State: "OPEN", BaseRef: "main"}}, gh.RepoMergeConfig{}, nil)
				m.On("Unstack", mock.Anything, "owner", "repo", 9).Return(200, []byte(`{"number":9}`), nil)
			},
			want: NativeDissolve, wantErr: "partially unstacked",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, ctx, github := newNativeContext(t, tt.changes)
			tt.setup(github)
			plan, err := client.ReconcileNative(t.Context(), ctx, false)
			assert.Equal(t, tt.want, plan.Action)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			github.AssertExpectations(t)
		})
	}
}

func TestLegacyMetadataDropsVisualizationFieldOnSave(t *testing.T) {
	client, _, _ := newNativeContext(t, nil)
	dir := client.getStackDir("test-stack")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "prs.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"prs":{"id":{"pr_number":7,"viz_comment_id":"legacy"}}}`), 0o644))
	data, err := client.LoadPRs("test-stack")
	require.NoError(t, err)
	require.NoError(t, client.savePRs("test-stack", data))
	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(saved), `"version": 1`)
	assert.NotContains(t, string(saved), "viz_comment_id")
}

func nativeChanges(numbers ...int) []*model.Change {
	changes := make([]*model.Change, len(numbers))
	for i, number := range numbers {
		base := "main"
		if i > 0 {
			base = "branch-" + string(rune('0'+i))
		}
		changes[i] = &model.Change{UUID: fmt.Sprintf("uuid-%d", number), Title: fmt.Sprintf("PR %d", number), DesiredBase: base, PR: &model.PR{PRNumber: number, Base: base, State: "open"}}
	}
	return changes
}

func newNativeContext(t *testing.T, changes []*model.Change) (*Client, *StackContext, *gh.MockGithubClient) {
	github := &gh.MockGithubClient{}
	client := NewClient(testutil.NewTestGitClient(t), github)
	client.SetUsernameForTesting("test")
	stackModel := &model.Stack{Name: "test-stack", Owner: "owner", RepoName: "repo", Base: "main", Branch: "test/stack-test-stack/TOP"}
	require.NoError(t, client.SaveStack(stackModel))
	ctx := &StackContext{StackName: "test-stack", Stack: stackModel, AllChanges: changes, ActiveChanges: changes, client: client, username: "test"}
	return client, ctx, github
}
