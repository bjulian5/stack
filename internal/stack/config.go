package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GlobalConfig represents the global stack installation configuration for a repository
type GlobalConfig struct {
	HooksInstalled bool      `json:"hooks_installed"`
	HooksVersion   string    `json:"hooks_version"`   // Version of hooks for future compatibility
	GitConfigured  bool      `json:"git_configured"`  // Whether git settings have been configured
	InstalledAt    time.Time `json:"installed_at"`    // When stack was first installed
	LastUpdatedAt  time.Time `json:"last_updated_at"` // Last time config was updated
}

// CurrentHooksVersion is the current version of the hooks system
const CurrentHooksVersion = "1.0.0"

// getGlobalConfigPath returns the path to the global config file
func (c *Client) getGlobalConfigPath() string {
	return filepath.Join(c.getStacksRootDir(), "config.json")
}

// loadGlobalConfig loads the global stack configuration
// Returns a default config if the file doesn't exist
func (c *Client) loadGlobalConfig() (*GlobalConfig, error) {
	configPath := c.getGlobalConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if not found
			return &GlobalConfig{
				HooksInstalled: false,
				HooksVersion:   "",
				GitConfigured:  false,
			}, nil
		}
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}

	var config GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse global config: %w", err)
	}

	return &config, nil
}

// saveGlobalConfig saves the global stack configuration
func (c *Client) saveGlobalConfig(config *GlobalConfig) error {
	stacksRoot := c.getStacksRootDir()

	// Ensure .git/stack directory exists
	if err := os.MkdirAll(stacksRoot, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	config.LastUpdatedAt = time.Now()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := c.getGlobalConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// IsInstalled checks if stack is properly installed in this repository
func (c *Client) IsInstalled() (bool, error) {
	config, err := c.loadGlobalConfig()
	if err != nil {
		return false, err
	}

	return config.HooksInstalled && config.GitConfigured, nil
}

// MarkInstalled marks stack as installed with current timestamp
func (c *Client) MarkInstalled() error {
	config, err := c.loadGlobalConfig()
	if err != nil {
		return err
	}

	now := time.Now()

	// Set installed timestamp if this is the first install
	if config.InstalledAt.IsZero() {
		config.InstalledAt = now
	}

	config.HooksInstalled = true
	config.HooksVersion = CurrentHooksVersion
	config.GitConfigured = true

	return c.saveGlobalConfig(config)
}
