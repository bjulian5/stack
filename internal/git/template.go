package git

import (
	"os"
	"path/filepath"
)

// prTemplateLocations lists standard GitHub PR template locations in order of precedence
var prTemplateLocations = []string{
	".github/PULL_REQUEST_TEMPLATE.md",
	".github/pull_request_template.md",
	"docs/pull_request_template.md",
	"PULL_REQUEST_TEMPLATE.md",
	"pull_request_template.md",
}

// FindPRTemplate searches for a GitHub pull request template in standard locations.
func (c *Client) FindPRTemplate() (string, error) {
	for _, location := range prTemplateLocations {
		templatePath := filepath.Join(c.gitRoot, location)
		content, err := os.ReadFile(templatePath)
		if err == nil {
			return string(content), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", nil
}
