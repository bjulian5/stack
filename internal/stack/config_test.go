package stack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/testutil"
)

func TestGetRepositoryConfigPath(t *testing.T) {
	gitClient := testutil.NewTestGitClient(t)
	client := NewClient(gitClient, &gh.MockGithubClient{})

	expected := filepath.Join(client.gitRoot, ".git", "stack", "config.json")
	actual := client.getRepositoryConfigPath()

	assert.Equal(t, expected, actual)
}

func TestLoadRepositoryConfig_NotExists(t *testing.T) {
	gitClient := testutil.NewTestGitClient(t)
	client := NewClient(gitClient, &gh.MockGithubClient{})

	config, err := client.loadRepositoryConfig()
	require.NoError(t, err)

	assert.False(t, config.HooksInstalled)
	assert.False(t, config.GitConfigured)
	assert.Empty(t, config.HooksVersion)
}

func TestSaveAndLoadRepositoryConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gitClient := testutil.NewTestGitClient(t)
		client := NewClient(gitClient, &gh.MockGithubClient{})

		now := time.Now()
		testConfig := &RepositoryConfig{
			HooksInstalled: true,
			HooksVersion:   "1.0.0",
			GitConfigured:  true,
			InstalledAt:    now,
		}

		err := client.saveRepositoryConfig(testConfig)
		require.NoError(t, err)

		loadedConfig, err := client.loadRepositoryConfig()
		require.NoError(t, err)

		expected := &RepositoryConfig{
			HooksInstalled: true,
			HooksVersion:   "1.0.0",
			GitConfigured:  true,
			InstalledAt:    now,
			LastUpdatedAt:  now,
		}

		assert.Equal(t, expected, loadedConfig)
	})
}

func TestSaveRepositoryConfig_CreatesDirectory(t *testing.T) {
	gitClient := testutil.NewTestGitClient(t)
	client := NewClient(gitClient, &gh.MockGithubClient{})

	stackDir := client.getStacksRootDir()
	_, err := os.Stat(stackDir)
	assert.True(t, os.IsNotExist(err))

	testConfig := &RepositoryConfig{
		HooksInstalled: true,
		HooksVersion:   "1.0.0",
	}
	err = client.saveRepositoryConfig(testConfig)
	require.NoError(t, err)

	_, err = os.Stat(stackDir)
	assert.False(t, os.IsNotExist(err))
}

func TestLoadRepositoryConfig_MalformedJSON(t *testing.T) {
	gitClient := testutil.NewTestGitClient(t)
	client := NewClient(gitClient, &gh.MockGithubClient{})

	err := os.MkdirAll(client.getStacksRootDir(), 0755)
	require.NoError(t, err)

	err = os.WriteFile(client.getRepositoryConfigPath(), []byte("invalid json{"), 0644)
	require.NoError(t, err)

	_, err = client.loadRepositoryConfig()
	assert.Error(t, err)
}

func TestIsInstalled(t *testing.T) {
	tests := []struct {
		name           string
		hooksInstalled bool
		gitConfigured  bool
		expected       bool
	}{
		{"Only hooks installed", true, false, false},
		{"Only git configured", false, true, false},
		{"Both installed", true, true, true},
		{"Neither installed", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := testutil.NewTestGitClient(t)
			client := NewClient(gitClient, &gh.MockGithubClient{})

			config := &RepositoryConfig{
				HooksInstalled: tt.hooksInstalled,
				GitConfigured:  tt.gitConfigured,
			}
			err := client.saveRepositoryConfig(config)
			require.NoError(t, err)

			installed, err := client.IsInstalled()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, installed)
		})
	}
}

func TestMarkInstalled_FirstTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gitClient := testutil.NewTestGitClient(t)
		client := NewClient(gitClient, &gh.MockGithubClient{})

		now := time.Now()
		err := client.MarkInstalled()
		require.NoError(t, err)

		config, err := client.loadRepositoryConfig()
		require.NoError(t, err)

		expected := &RepositoryConfig{
			HooksInstalled: true,
			HooksVersion:   CurrentHooksVersion,
			GitConfigured:  true,
			InstalledAt:    now,
			LastUpdatedAt:  now,
		}

		assert.Equal(t, expected, config)
	})
}

func TestMarkInstalled_AlreadyInstalled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gitClient := testutil.NewTestGitClient(t)
		client := NewClient(gitClient, &gh.MockGithubClient{})

		firstInstallTime := time.Now()

		err := client.MarkInstalled()
		require.NoError(t, err)

		synctest.Wait()
		secondInstallTime := time.Now()

		err = client.MarkInstalled()
		require.NoError(t, err)

		config, err := client.loadRepositoryConfig()
		require.NoError(t, err)

		expected := &RepositoryConfig{
			HooksInstalled: true,
			HooksVersion:   CurrentHooksVersion,
			GitConfigured:  true,
			InstalledAt:    firstInstallTime,
			LastUpdatedAt:  secondInstallTime,
		}

		assert.Equal(t, expected, config)
	})
}

func TestRepositoryConfig_JSONSerialization(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		original := &RepositoryConfig{
			HooksInstalled: true,
			HooksVersion:   "1.0.0",
			GitConfigured:  true,
			InstalledAt:    now,
			LastUpdatedAt:  now,
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded RepositoryConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, &decoded)
	})
}
