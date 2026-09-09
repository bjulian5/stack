package push

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
	"github.com/bjulian5/stack/internal/stack"
)

func TestValidatePRHeads(t *testing.T) {
	const branch = "creator/stack-example/1111111111111111"
	for _, tt := range []struct {
		name    string
		head    string
		wantErr string
	}{
		{name: "original PR branch", head: branch},
		{name: "different owner", head: "collaborator/stack-example/1111111111111111", wantErr: "repair the local stack branch"},
		{name: "missing head", wantErr: "PR #42 uses head"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &stack.StackContext{
				StackName: "example",
				Stack:     &model.Stack{Name: "example", Branch: "creator/stack-example/TOP"},
				ActiveChanges: []*model.Change{
					{UUID: "1111111111111111", PR: &model.PR{PRNumber: 42}},
					{UUID: "2222222222222222"},
				},
			}
			err := validatePRHeads(ctx, map[int]gh.PRDetails{42: {Number: 42, HeadRef: tt.head}})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
