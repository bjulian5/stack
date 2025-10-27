package common

import (
	"strings"

	"github.com/google/uuid"
)

// GenerateUUID generates a 16-character hex UUID for PR identification
func GenerateUUID() string {
	u := uuid.New()
	hexStr := strings.ReplaceAll(u.String(), "-", "")
	return hexStr[:16]
}
