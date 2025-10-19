package common

import (
	"fmt"
	"os/user"
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
