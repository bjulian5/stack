package stack

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/model"
)

func TestStackContext_IsStack(t *testing.T) {
	tests := []struct {
		name      string
		stackName string
		expected  bool
	}{
		{"with stack name", "test-stack", true},
		{"empty stack name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &StackContext{
				StackName: tt.stackName,
			}
			assert.Equal(t, tt.expected, ctx.IsStack())
		})
	}
}

func TestStackContext_OnUUIDBranch(t *testing.T) {
	tests := []struct {
		name     string
		onUUID   bool
		expected bool
	}{
		{"on UUID branch", true, true},
		{"on stack branch", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &StackContext{onUUIDBranch: tt.onUUID}
			assert.Equal(t, tt.expected, ctx.OnUUIDBranch())
		})
	}
}

func TestStackContext_ChangeID(t *testing.T) {
	tests := []struct {
		name string
		uuid string
	}{
		{"with UUID", "1234567890abcdef"},
		{"empty UUID", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &StackContext{currentUUID: tt.uuid}
			assert.Equal(t, tt.uuid, ctx.ChangeID())
		})
	}
}

func TestStackContext_CurrentChange(t *testing.T) {
	change1 := &model.Change{UUID: "1111111111111111", Title: "First change"}
	change2 := &model.Change{UUID: "2222222222222222", Title: "Second change"}

	tests := []struct {
		name        string
		currentUUID string
		changes     map[string]*model.Change
		expected    *model.Change
	}{
		{
			name:        "finds current change",
			currentUUID: change1.UUID,
			changes:     map[string]*model.Change{change1.UUID: change1, change2.UUID: change2},
			expected:    change1,
		},
		{
			name:        "no current UUID",
			currentUUID: "",
			changes:     map[string]*model.Change{change1.UUID: change1},
			expected:    nil,
		},
		{
			name:        "UUID not found",
			currentUUID: "9999999999999999",
			changes:     map[string]*model.Change{change1.UUID: change1},
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &StackContext{currentUUID: tt.currentUUID, changes: tt.changes}
			assert.Equal(t, tt.expected, ctx.CurrentChange())
		})
	}
}

func TestStackContext_FindChange(t *testing.T) {
	change1 := &model.Change{UUID: "1111111111111111", Title: "First change"}
	change2 := &model.Change{UUID: "2222222222222222", Title: "Second change"}

	ctx := &StackContext{
		changes: map[string]*model.Change{
			change1.UUID: change1,
			change2.UUID: change2,
		},
	}

	t.Run("finds existing changes", func(t *testing.T) {
		assert.Equal(t, change1, ctx.FindChange(change1.UUID))
		assert.Equal(t, change2, ctx.FindChange(change2.UUID))
	})

	t.Run("non-existent UUID", func(t *testing.T) {
		assert.Nil(t, ctx.FindChange("9999999999999999"))
	})

	t.Run("empty UUID", func(t *testing.T) {
		assert.Nil(t, ctx.FindChange(""))
	})
}

func TestStackContext_FindChangeInActive(t *testing.T) {
	change1 := &model.Change{UUID: "1111111111111111", Title: "First change"}
	change2 := &model.Change{UUID: "2222222222222222", Title: "Second change"}
	change3 := &model.Change{UUID: "3333333333333333", Title: "Third change (merged)"}

	ctx := &StackContext{
		changes: map[string]*model.Change{
			change1.UUID: change1,
			change2.UUID: change2,
			change3.UUID: change3,
		},
		ActiveChanges: []*model.Change{change1, change2},
	}

	t.Run("finds changes in active set", func(t *testing.T) {
		assert.Equal(t, change1, ctx.FindChangeInActive(change1.UUID))
		assert.Equal(t, change2, ctx.FindChangeInActive(change2.UUID))
	})

	t.Run("merged change not in active set", func(t *testing.T) {
		assert.Nil(t, ctx.FindChangeInActive(change3.UUID))
	})

	t.Run("non-existent UUID", func(t *testing.T) {
		assert.Nil(t, ctx.FindChangeInActive("9999999999999999"))
	})
}

func TestStackContext_FormatUUIDBranch(t *testing.T) {
	ctx := &StackContext{username: "test-user", StackName: "auth-refactor"}
	assert.Equal(t, "test-user/stack-auth-refactor/1234567890abcdef", ctx.FormatUUIDBranch("1234567890abcdef"))
}

func TestStackContext_Save(t *testing.T) {
	t.Run("errors when no client", func(t *testing.T) {
		ctx := &StackContext{StackName: "test-stack", client: nil}
		err := ctx.Save()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot save")
	})

	t.Run("saves PRs and stack successfully", func(t *testing.T) {
		mockGithubClient := &gh.MockGithubClient{}
		mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

		stackClient := NewTestStack(t, mockGithubClient)
		stack, err := stackClient.CreateStack("test-stack", "main")
		require.NoError(t, err)

		uuid := "aaaa111111111111"
		pr := &model.PR{
			PRNumber: 101,
			URL:      "https://github.com/test-owner/test-repo/pull/101",
			State:    "open",
		}
		change := &model.Change{UUID: uuid, Title: "Test change", PR: pr}

		ctx := &StackContext{
			StackName: "test-stack",
			Stack:     stack,
			changes:   map[string]*model.Change{uuid: change},
			client:    stackClient,
		}

		err = ctx.Save()
		assert.NoError(t, err)

		loadedPRData, err := stackClient.LoadPRs("test-stack")
		require.NoError(t, err)

		expectedPRData := &model.PRData{
			Version: 1,
			PRs:     map[string]*model.PR{uuid: pr},
		}
		assert.Equal(t, expectedPRData, loadedPRData)
		mockGithubClient.AssertExpectations(t)
	})

	t.Run("saves only changh.ges with PRs", func(t *testing.T) {
		mockGithubClient := &gh.MockGithubClient{}
		mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

		stackClient := NewTestStack(t, mockGithubClient)
		stack, err := stackClient.CreateStack("test-stack", "main")
		require.NoError(t, err)

		uuid1 := "aaaa111111111111"
		uuid2 := "bbbb222222222222"
		pr := &model.PR{PRNumber: 101, URL: "https://github.com/test-owner/test-repo/pull/101"}

		changeWithPR := &model.Change{UUID: uuid1, Title: "With PR", PR: pr}
		changeWithoutPR := &model.Change{UUID: uuid2, Title: "Without PR", PR: nil}

		ctx := &StackContext{
			StackName: "test-stack",
			Stack:     stack,
			changes:   map[string]*model.Change{uuid1: changeWithPR, uuid2: changeWithoutPR},
			client:    stackClient,
		}

		err = ctx.Save()
		assert.NoError(t, err)

		loadedPRData, err := stackClient.LoadPRs("test-stack")
		require.NoError(t, err)

		expectedPRData := &model.PRData{
			Version: 1,
			PRs:     map[string]*model.PR{uuid1: pr},
		}
		assert.Equal(t, expectedPRData, loadedPRData)
		mockGithubClient.AssertExpectations(t)
	})
}

func TestFormatStackBranch(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		stackName string
		expected  string
	}{
		{"basic stack", "user", "feature", "user/stack-feature/TOP"},
		{"hyphenated stack", "user", "auth-refactor", "user/stack-auth-refactor/TOP"},
		{"underscored username", "user_name", "stack", "user_name/stack-stack/TOP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStackBranch(tt.username, tt.stackName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateBottomUpMerges(t *testing.T) {
	t.Run("no merged PRs", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: &model.PR{PRNumber: 101}},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
		}
		assert.NoError(t, validateBottomUpMerges(changes, map[int]bool{}))
	})

	t.Run("all merged from bottom up", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: &model.PR{PRNumber: 101}},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
			{UUID: "3333333333333333", PR: &model.PR{PRNumber: 103}},
		}
		merged := map[int]bool{101: true, 102: true, 103: true}
		assert.NoError(t, validateBottomUpMerges(changes, merged))
	})

	t.Run("bottom two merged", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: &model.PR{PRNumber: 101}},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
			{UUID: "3333333333333333", PR: &model.PR{PRNumber: 103}},
		}
		merged := map[int]bool{101: true, 102: true}
		assert.NoError(t, validateBottomUpMerges(changes, merged))
	})

	t.Run("out of order merge", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: &model.PR{PRNumber: 101}},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
			{UUID: "3333333333333333", PR: &model.PR{PRNumber: 103}},
		}
		merged := map[int]bool{101: true, 103: true}

		err := validateBottomUpMerges(changes, merged)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out-of-order merge detected")
		assert.Contains(t, err.Error(), "PR #103")
		assert.Contains(t, err.Error(), "change #2")
	})

	t.Run("middle merged but not bottom", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: &model.PR{PRNumber: 101}},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
			{UUID: "3333333333333333", PR: &model.PR{PRNumber: 103}},
		}
		merged := map[int]bool{102: true}

		err := validateBottomUpMerges(changes, merged)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out-of-order merge detected")
		assert.Contains(t, err.Error(), "PR #102")
	})

	t.Run("merged PR after local change", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: nil},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
		}
		merged := map[int]bool{102: true}

		err := validateBottomUpMerges(changes, merged)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out-of-order merge")
	})

	t.Run("local changes with no merges", func(t *testing.T) {
		changes := []*model.Change{
			{UUID: "1111111111111111", PR: nil},
			{UUID: "2222222222222222", PR: &model.PR{PRNumber: 102}},
			{UUID: "3333333333333333", PR: &model.PR{PRNumber: 103}},
		}
		assert.NoError(t, validateBottomUpMerges(changes, map[int]bool{}))
	})
}

func TestIsUUIDBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected bool
	}{
		{"valid UUID branch", "user/stack-feature/1234567890abcdef", true},
		{"valid UUID branch uppercase", "user/stack-feature/1234567890ABCDEF", true},
		{"TOP branch", "user/stack-feature/TOP", false},
		{"invalid UUID too short", "user/stack-feature/123456", false},
		{"invalid UUID too long", "user/stack-feature/1234567890abcdef1", false},
		{"invalid UUID non-hex", "user/stack-feature/123456789012345g", false},
		{"missing stack prefix", "user/feature/1234567890abcdef", false},
		{"wrong number of parts", "user-stack-feature/1234567890abcdef", false},
		{"empty branch", "", false},
		{"regular branch", "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUUIDBranch(tt.branch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		expected bool
	}{
		{"valid lowercase", "1234567890abcdef", true},
		{"valid uppercase", "1234567890ABCDEF", true},
		{"valid mixed case", "1234567890AbCdEf", true},
		{"valid all zeros", "0000000000000000", true},
		{"valid all fs", "ffffffffffffffff", true},
		{"too short", "123456789012345", false},
		{"too long", "1234567890abcdef1", false},
		{"invalid char g", "123456789012345g", false},
		{"invalid char space", "12345678 0abcdef", false},
		{"invalid char dash", "123456-890abcdef", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validUUID(tt.uuid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStackName(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{"TOP branch", "user/stack-feature/TOP", "feature"},
		{"UUID branch", "user/stack-feature/1234567890abcdef", "feature"},
		{"hyphenated stack", "user/stack-auth-refactor/TOP", "auth-refactor"},
		{"missing stack prefix", "user/feature/TOP", ""},
		{"wrong suffix", "user/stack-feature/BOTTOM", ""},
		{"invalid UUID", "user/stack-feature/invalid", ""},
		{"too few parts", "user/TOP", ""},
		{"too many parts", "user/stack-feature/TOP/extra", ""},
		{"empty branch", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStackName(tt.branch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractUUIDFromBranch(t *testing.T) {
	tests := []struct {
		name              string
		branch            string
		expectedStackName string
		expectedUUID      string
	}{
		{
			name:              "TOP branch",
			branch:            "user/stack-feature/TOP",
			expectedStackName: "feature",
			expectedUUID:      "TOP",
		},
		{
			name:              "UUID branch",
			branch:            "user/stack-feature/1234567890abcdef",
			expectedStackName: "feature",
			expectedUUID:      "1234567890abcdef",
		},
		{
			name:              "hyphenated stack with UUID",
			branch:            "user/stack-auth-refactor/1234567890abcdef",
			expectedStackName: "auth-refactor",
			expectedUUID:      "1234567890abcdef",
		},
		{
			name:              "missing stack prefix",
			branch:            "user/feature/TOP",
			expectedStackName: "",
			expectedUUID:      "",
		},
		{
			name:              "wrong format",
			branch:            "user-stack-feature",
			expectedStackName: "",
			expectedUUID:      "",
		},
		{
			name:              "empty branch",
			branch:            "",
			expectedStackName: "",
			expectedUUID:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stackName, uuid := extractUUIDFromBranch(tt.branch)
			assert.Equal(t, tt.expectedStackName, stackName)
			assert.Equal(t, tt.expectedUUID, uuid)
		})
	}
}

func TestStackContext_Integration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockGithubClient := &gh.MockGithubClient{}
		mockGithubClient.On("GetRepoInfo").Return("test-owner", "test-repo", nil)

		stackClient := NewTestStack(t, mockGithubClient)
		stack, err := stackClient.CreateStack("integration-test", "main")
		require.NoError(t, err)

		uuid1 := "aaaa111111111111"
		uuid2 := "bbbb222222222222"
		uuid3 := "cccc333333333333"

		pr1 := &model.PR{PRNumber: 101, State: "open"}
		pr2 := &model.PR{PRNumber: 102, State: "open"}
		pr3 := &model.PR{PRNumber: 103, State: "merged"}

		change1 := &model.Change{UUID: uuid1, Title: "First change", Position: 1, ActivePosition: 1, PR: pr1}
		change2 := &model.Change{UUID: uuid2, Title: "Second change", Position: 2, ActivePosition: 2, PR: pr2}
		change3 := &model.Change{UUID: uuid3, Title: "Third change (merged)", Position: 3, PR: pr3}

		ctx := &StackContext{
			StackName:          "integration-test",
			Stack:              stack,
			changes:            map[string]*model.Change{uuid1: change1, uuid2: change2, uuid3: change3},
			AllChanges:         []*model.Change{change1, change2, change3},
			ActiveChanges:      []*model.Change{change1, change2},
			StaleMergedChanges: []*model.Change{change3},
			currentUUID:        uuid2,
			onUUIDBranch:       true,
			username:           "test-user",
			client:             stackClient,
		}

		assert.True(t, ctx.IsStack())
		assert.True(t, ctx.OnUUIDBranch())

		currentChange := ctx.CurrentChange()
		assert.NotNil(t, currentChange)
		assert.Equal(t, "Second change", currentChange.Title)

		assert.Equal(t, uuid2, ctx.ChangeID())
		assert.Equal(t, change1, ctx.FindChange(uuid1))
		assert.Equal(t, change3, ctx.FindChange(uuid3))
		assert.Equal(t, change1, ctx.FindChangeInActive(uuid1))
		assert.Nil(t, ctx.FindChangeInActive(uuid3))

		assert.Equal(t, "test-user/stack-integration-test/"+uuid1, ctx.FormatUUIDBranch(uuid1))

		err = ctx.Save()
		assert.NoError(t, err)

		loadedPRData, err := stackClient.LoadPRs("integration-test")
		require.NoError(t, err)

		expectedPRData := &model.PRData{
			Version: 1,
			PRs:     map[string]*model.PR{uuid1: pr1, uuid2: pr2, uuid3: pr3},
		}
		assert.Equal(t, expectedPRData, loadedPRData)
		mockGithubClient.AssertExpectations(t)
	})
}
