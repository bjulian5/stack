package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStack_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		stack Stack
	}{
		{
			name: "stack with merged changes and PR data",
			stack: Stack{
				Name:       "test-stack",
				Branch:     "user/stack-test-stack/TOP",
				Base:       "main",
				Owner:      "testuser",
				RepoName:   "testrepo",
				Created:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				LastSynced: time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC),
				SyncHash:   "abc123",
				BaseRef:    "refs/heads/main",
				MergedChanges: []Change{
					{
						Position:       1,
						ActivePosition: 0,
						UUID:           "550e8400",
						Title:          "First change",
						Description:    "First change description",
						CommitHash:     "hash1",
						MergedAt:       time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
						DesiredBase:    "main",
						PR: &PR{
							PRNumber:          101,
							URL:               "https://github.com/testuser/testrepo/pull/101",
							Branch:            "user/stack-test-stack/550e8400",
							CommitHash:        "hash1",
							State:             "merged",
							Title:             "First change",
							Body:              "First change description",
							Base:              "main",
							CreatedAt:         time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
							LastPushed:        time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC),
							LocalDraftStatus:  false,
							RemoteDraftStatus: false,
						},
					},
				},
			},
		},
		{
			name: "stack with nil merged changes",
			stack: Stack{
				Name:          "test-stack",
				Branch:        "user/stack-test-stack/TOP",
				Base:          "main",
				MergedChanges: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			jsonData, err := json.Marshal(tt.stack)
			require.NoError(t, err)

			// Unmarshal back to Stack
			var roundTripped Stack
			err = json.Unmarshal(jsonData, &roundTripped)
			require.NoError(t, err)

			// Compare original and round-tripped
			assert.Equal(t, tt.stack, roundTripped)
		})
	}
}
