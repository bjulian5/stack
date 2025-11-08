package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RepositoryConfig represents the repository-level stack installation configuration
type RepositoryConfig struct {
	HooksInstalled bool      `json:"hooks_installed"`
	HooksVersion   string    `json:"hooks_version"`   // Version of hooks for future compatibility
	GitConfigured  bool      `json:"git_configured"`  // Whether git settings have been configured
	InstalledAt    time.Time `json:"installed_at"`    // When stack was first installed
	LastUpdatedAt  time.Time `json:"last_updated_at"` // Last time config was updated
}

// CurrentHooksVersion is the current version of the hooks system
const CurrentHooksVersion = "1.0.0"

// getRepositoryConfigPath returns the path to the repository config file
func (c *Client) getRepositoryConfigPath() string {
	return filepath.Join(c.getStacksRootDir(), "config.json")
}

// loadRepositoryConfig loads the repository stack configuration.
// Returns a default config if the file doesn't exist.
func (c *Client) loadRepositoryConfig() (*RepositoryConfig, error) {
	data, err := os.ReadFile(c.getRepositoryConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &RepositoryConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read repository config: %w", err)
	}

	var config RepositoryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repository config: %w", err)
	}

	return &config, nil
}

// saveRepositoryConfig saves the repository stack configuration.
func (c *Client) saveRepositoryConfig(config *RepositoryConfig) error {
	if err := os.MkdirAll(c.getStacksRootDir(), 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	config.LastUpdatedAt = time.Now()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.getRepositoryConfigPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// IsInstalled checks if stack is properly installed in this repository.
func (c *Client) IsInstalled() (bool, error) {
	config, err := c.loadRepositoryConfig()
	if err != nil {
		return false, err
	}

	return config.HooksInstalled && config.GitConfigured, nil
}

// MarkInstalled marks stack as installed with current timestamp.
func (c *Client) MarkInstalled() error {
	config, err := c.loadRepositoryConfig()
	if err != nil {
		return err
	}

	if config.InstalledAt.IsZero() {
		config.InstalledAt = time.Now()
	}

	config.HooksInstalled = true
	config.HooksVersion = CurrentHooksVersion
	config.GitConfigured = true

	return c.saveRepositoryConfig(config)
}
