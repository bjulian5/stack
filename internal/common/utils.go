package common

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/google/uuid"
)

// GetUsername returns the username for branch naming
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
	hexStr := strings.ReplaceAll(u.String(), "-", "")
	return hexStr[:16]
}
