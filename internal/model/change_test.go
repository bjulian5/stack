package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
)

func TestChange_IsLocal(t *testing.T) {
	tests := []struct {
		name     string
		change   *Change
		expected bool
	}{
		{
			name: "nil PR pointer",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR:         nil,
			},
			expected: true,
		},
		{
			name: "PR with zero PR number",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR: &PR{
					PRNumber: 0,
					URL:      "https://github.com/owner/repo/pull/123",
				},
			},
			expected: true,
		},
		{
			name: "PR with non-zero PR number",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR: &PR{
					PRNumber: 123,
					URL:      "https://github.com/owner/repo/pull/123",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.change.IsLocal()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChange_GetDraftStatus(t *testing.T) {
	tests := []struct {
		name     string
		change   *Change
		expected bool
	}{
		{
			name: "nil PR defaults to draft",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR:         nil,
			},
			expected: true,
		},
		{
			name: "PR with draft status true",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR: &PR{
					PRNumber:         123,
					LocalDraftStatus: true,
				},
			},
			expected: true,
		},
		{
			name: "PR with draft status false",
			change: &Change{
				UUID:       "test-uuid",
				CommitHash: "abc123",
				PR: &PR{
					PRNumber:         123,
					LocalDraftStatus: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.change.GetDraftStatus()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChange_NeedsSyncToGitHub(t *testing.T) {
	tests := []struct {
		name     string
		change   *Change
		expected ChangeSyncStatus
	}{
		{
			name: "new change with nil PR",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR:          nil,
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "new change"},
		},
		{
			name: "new change with zero PR number",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber: 0,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "new change"},
		},
		{
			name: "metadata not cached - empty title",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"},
		},
		{
			name: "metadata not cached - empty base",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Base:       "",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"},
		},
		{
			name: "metadata not cached - both empty",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "",
					Base:       "",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"},
		},
		{
			name: "commit hash changed",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "def456",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Body:       "Test description",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "commit changed"},
		},
		{
			name: "title changed",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Updated PR Title",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Body:       "Test description",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "title changed"},
		},
		{
			name: "description changed",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Updated description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Body:       "Test description",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "description changed"},
		},
		{
			name: "base changed with desired base set",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				DesiredBase: "feature-branch",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Body:       "Test description",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "base changed"},
		},
		{
			name: "base not changed when desired base is empty",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				DesiredBase: "",
				PR: &PR{
					PRNumber:          123,
					Title:             "Test PR",
					Body:              "Test description",
					Base:              "main",
					CommitHash:        "abc123",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: false},
		},
		{
			name: "draft status changed - local draft, remote ready",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					Title:             "Test PR",
					Body:              "Test description",
					Base:              "main",
					CommitHash:        "abc123",
					LocalDraftStatus:  true,
					RemoteDraftStatus: false,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "draft status changed"},
		},
		{
			name: "draft status changed - local ready, remote draft",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					Title:             "Test PR",
					Body:              "Test description",
					Base:              "main",
					CommitHash:        "abc123",
					LocalDraftStatus:  false,
					RemoteDraftStatus: true,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "draft status changed"},
		},
		{
			name: "no sync needed - all fields match",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				DesiredBase: "main",
				PR: &PR{
					PRNumber:          123,
					Title:             "Test PR",
					Body:              "Test description",
					Base:              "main",
					CommitHash:        "abc123",
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: false},
		},
		{
			name: "no sync needed - draft statuses match as false",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					Title:             "Test PR",
					Body:              "Test description",
					Base:              "main",
					CommitHash:        "abc123",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			expected: ChangeSyncStatus{NeedsSync: false},
		},
		{
			name: "multiple changes - commit and title",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Updated Title",
				Description: "Test description",
				CommitHash:  "def456",
				PR: &PR{
					PRNumber:   123,
					Title:      "Test PR",
					Body:       "Test description",
					Base:       "main",
					CommitHash: "abc123",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "commit changed"},
		},
		{
			name: "empty strings in change fields",
			change: &Change{
				UUID:        "",
				Title:       "",
				Description: "",
				CommitHash:  "",
				PR: &PR{
					PRNumber:   123,
					Title:      "",
					Body:       "",
					Base:       "",
					CommitHash: "",
				},
			},
			expected: ChangeSyncStatus{NeedsSync: true, Reason: "metadata not cached"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.change.NeedsSyncToGitHub()
			assert.Equal(t, tt.expected.NeedsSync, result.NeedsSync)
			assert.Equal(t, tt.expected.Reason, result.Reason)
		})
	}
}

func TestChange_UpdateFromPush(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedTime := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		change   *Change
		ghPR     *gh.PR
		branch   string
		expected *PR
	}{
		{
			name: "creates new PR when nil",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR:          nil,
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "open",
				IsDraft:   true,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "user/stack-test/TOP",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "open",
				Branch:            "user/stack-test/TOP",
				CommitHash:        "abc123",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				LocalDraftStatus:  true,
				RemoteDraftStatus: true,
			},
		},
		{
			name: "updates existing PR",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "def456",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "open",
					Branch:            "user/stack-test/TOP",
					CommitHash:        "abc123",
					CreatedAt:         baseTime,
					LastPushed:        baseTime,
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "open",
				IsDraft:   false,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "user/stack-test/TOP",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "open",
				Branch:            "user/stack-test/TOP",
				CommitHash:        "def456",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		},
		{
			name: "updates draft status from GitHub",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "draft",
					Branch:            "user/stack-test/TOP",
					CommitHash:        "abc123",
					CreatedAt:         baseTime,
					LastPushed:        baseTime,
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
				},
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "open",
				IsDraft:   false,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "user/stack-test/TOP",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "open",
				Branch:            "user/stack-test/TOP",
				CommitHash:        "abc123",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		},
		{
			name: "updates state to merged",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "open",
					Branch:            "user/stack-test/TOP",
					CommitHash:        "abc123",
					CreatedAt:         baseTime,
					LastPushed:        baseTime,
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "merged",
				IsDraft:   false,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "user/stack-test/TOP",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "merged",
				Branch:            "user/stack-test/TOP",
				CommitHash:        "abc123",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		},
		{
			name: "handles branch name change",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "open",
					Branch:            "old-branch",
					CommitHash:        "abc123",
					CreatedAt:         baseTime,
					LastPushed:        baseTime,
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "open",
				IsDraft:   false,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "new-branch",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "open",
				Branch:            "new-branch",
				CommitHash:        "abc123",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		},
		{
			name: "preserves metadata fields not updated by push",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Test PR",
				Description: "Test description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "open",
					Branch:            "user/stack-test/TOP",
					CommitHash:        "abc123",
					CreatedAt:         baseTime,
					LastPushed:        baseTime,
					Title:             "Cached Title",
					Body:              "Cached Body",
					Base:              "main",
					VizCommentID:      "comment-123",
					LocalDraftStatus:  false,
					RemoteDraftStatus: false,
				},
			},
			ghPR: &gh.PR{
				Number:    123,
				URL:       "https://github.com/owner/repo/pull/123",
				State:     "open",
				IsDraft:   false,
				CreatedAt: baseTime,
				UpdatedAt: updatedTime,
			},
			branch: "user/stack-test/TOP",
			expected: &PR{
				PRNumber:          123,
				URL:               "https://github.com/owner/repo/pull/123",
				State:             "open",
				Branch:            "user/stack-test/TOP",
				CommitHash:        "abc123",
				CreatedAt:         baseTime,
				LastPushed:        updatedTime,
				Title:             "Cached Title",
				Body:              "Cached Body",
				Base:              "main",
				VizCommentID:      "comment-123",
				LocalDraftStatus:  false,
				RemoteDraftStatus: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.change.UpdateFromPush(tt.ghPR, tt.branch)

			require.NotNil(t, tt.change.PR)
			assert.Equal(t, tt.expected.PRNumber, tt.change.PR.PRNumber)
			assert.Equal(t, tt.expected.URL, tt.change.PR.URL)
			assert.Equal(t, tt.expected.State, tt.change.PR.State)
			assert.Equal(t, tt.expected.Branch, tt.change.PR.Branch)
			assert.Equal(t, tt.expected.CommitHash, tt.change.PR.CommitHash)
			assert.Equal(t, tt.expected.CreatedAt, tt.change.PR.CreatedAt)
			assert.Equal(t, tt.expected.LastPushed, tt.change.PR.LastPushed)
			assert.Equal(t, tt.expected.LocalDraftStatus, tt.change.PR.LocalDraftStatus)
			assert.Equal(t, tt.expected.RemoteDraftStatus, tt.change.PR.RemoteDraftStatus)

			// If we're testing metadata preservation, check those fields too
			if tt.expected.Title != "" {
				assert.Equal(t, tt.expected.Title, tt.change.PR.Title)
			}
			if tt.expected.Body != "" {
				assert.Equal(t, tt.expected.Body, tt.change.PR.Body)
			}
			if tt.expected.Base != "" {
				assert.Equal(t, tt.expected.Base, tt.change.PR.Base)
			}
			if tt.expected.VizCommentID != "" {
				assert.Equal(t, tt.expected.VizCommentID, tt.change.PR.VizCommentID)
			}
		})
	}
}

func TestChange_UpdateTitle(t *testing.T) {
	tests := []struct {
		name        string
		change      *Change
		title       string
		description string
		base        string
		verify      func(*testing.T, *Change)
	}{
		{
			name: "updates all fields when PR exists",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Original Title",
				Description: "Original Description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "Old Title",
					Body:       "Old Body",
					Base:       "old-base",
					CommitHash: "abc123",
				},
			},
			title:       "New Title",
			description: "New Description",
			base:        "new-base",
			verify: func(t *testing.T, c *Change) {
				require.NotNil(t, c.PR)
				assert.Equal(t, "New Title", c.PR.Title)
				assert.Equal(t, "New Description", c.PR.Body)
				assert.Equal(t, "new-base", c.PR.Base)
			},
		},
		{
			name: "does nothing when PR is nil",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Original Title",
				Description: "Original Description",
				CommitHash:  "abc123",
				PR:          nil,
			},
			title:       "New Title",
			description: "New Description",
			base:        "new-base",
			verify: func(t *testing.T, c *Change) {
				assert.Nil(t, c.PR)
			},
		},
		{
			name: "handles empty strings",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Original Title",
				Description: "Original Description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:   123,
					Title:      "Old Title",
					Body:       "Old Body",
					Base:       "old-base",
					CommitHash: "abc123",
				},
			},
			title:       "",
			description: "",
			base:        "",
			verify: func(t *testing.T, c *Change) {
				require.NotNil(t, c.PR)
				assert.Equal(t, "", c.PR.Title)
				assert.Equal(t, "", c.PR.Body)
				assert.Equal(t, "", c.PR.Base)
			},
		},
		{
			name: "preserves other PR fields",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Original Title",
				Description: "Original Description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber:          123,
					URL:               "https://github.com/owner/repo/pull/123",
					State:             "open",
					Branch:            "user/stack-test/TOP",
					Title:             "Old Title",
					Body:              "Old Body",
					Base:              "old-base",
					CommitHash:        "abc123",
					VizCommentID:      "comment-123",
					LocalDraftStatus:  true,
					RemoteDraftStatus: true,
					CreatedAt:         time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					LastPushed:        time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
				},
			},
			title:       "Updated Title",
			description: "Updated Description",
			base:        "updated-base",
			verify: func(t *testing.T, c *Change) {
				require.NotNil(t, c.PR)
				// Updated fields
				assert.Equal(t, "Updated Title", c.PR.Title)
				assert.Equal(t, "Updated Description", c.PR.Body)
				assert.Equal(t, "updated-base", c.PR.Base)
				// Preserved fields
				assert.Equal(t, 123, c.PR.PRNumber)
				assert.Equal(t, "https://github.com/owner/repo/pull/123", c.PR.URL)
				assert.Equal(t, "open", c.PR.State)
				assert.Equal(t, "user/stack-test/TOP", c.PR.Branch)
				assert.Equal(t, "abc123", c.PR.CommitHash)
				assert.Equal(t, "comment-123", c.PR.VizCommentID)
				assert.Equal(t, true, c.PR.LocalDraftStatus)
				assert.Equal(t, true, c.PR.RemoteDraftStatus)
				assert.Equal(t, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), c.PR.CreatedAt)
				assert.Equal(t, time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), c.PR.LastPushed)
			},
		},
		{
			name: "handles multiline description",
			change: &Change{
				UUID:        "test-uuid",
				Title:       "Original Title",
				Description: "Original Description",
				CommitHash:  "abc123",
				PR: &PR{
					PRNumber: 123,
					Title:    "Old Title",
					Body:     "Old Body",
					Base:     "main",
				},
			},
			title:       "New Title",
			description: "Line 1\nLine 2\nLine 3",
			base:        "main",
			verify: func(t *testing.T, c *Change) {
				require.NotNil(t, c.PR)
				assert.Equal(t, "New Title", c.PR.Title)
				assert.Equal(t, "Line 1\nLine 2\nLine 3", c.PR.Body)
				assert.Equal(t, "main", c.PR.Base)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.change.UpdateTitle(tt.title, tt.description, tt.base)
			tt.verify(t, tt.change)
		})
	}
}

func TestChange_EdgeCases(t *testing.T) {
	t.Run("update title on nil PR does not panic", func(t *testing.T) {
		change := &Change{
			UUID:        "test-uuid",
			Title:       "Test",
			Description: "Test",
			CommitHash:  "abc123",
			PR:          nil,
		}

		// Should not panic
		assert.NotPanics(t, func() {
			change.UpdateTitle("New Title", "New Description", "new-base")
		})

		assert.Nil(t, change.PR)
	})

	t.Run("update from push with zero time values", func(t *testing.T) {
		change := &Change{
			UUID:        "test-uuid",
			Title:       "Test PR",
			Description: "Test description",
			CommitHash:  "abc123",
			PR:          nil,
		}

		ghPR := &gh.PR{
			Number:    123,
			URL:       "https://github.com/owner/repo/pull/123",
			State:     "open",
			IsDraft:   true,
			CreatedAt: time.Time{}, // zero value
			UpdatedAt: time.Time{}, // zero value
		}

		change.UpdateFromPush(ghPR, "test-branch")

		require.NotNil(t, change.PR)
		assert.Equal(t, 123, change.PR.PRNumber)
		assert.True(t, change.PR.CreatedAt.IsZero())
		assert.True(t, change.PR.LastPushed.IsZero())
	})
}
