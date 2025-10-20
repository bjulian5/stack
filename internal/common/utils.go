package common

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/google/uuid"
)

// GetUsername returns the username for branch naming
// TODO: Add config support for username override
func GetUsername() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return currentUser.Username, nil
}

// GenerateUUID generates a 16-character hex UUID for PR identification
func GenerateUUID() string {
	u := uuid.New()
	// Convert to hex string and take first 16 characters
	hexStr := strings.ReplaceAll(u.String(), "-", "")
	return hexStr[:16]
}

// ShortUUID returns the first 8 characters of a UUID for display
func ShortUUID(uuid string) string {
	if len(uuid) < 8 {
		return uuid
	}
	return uuid[:8]

}
