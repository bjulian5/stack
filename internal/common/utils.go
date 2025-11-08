package common

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/bjulian5/stack/internal/gh"
	"github.com/bjulian5/stack/internal/git"
	"github.com/bjulian5/stack/internal/stack"
	"github.com/bjulian5/stack/internal/ui"
)

// GenerateUUID generates a 16-character hex UUID for PR identification
func GenerateUUID() string {
	u := uuid.New()
	hexStr := strings.ReplaceAll(u.String(), "-", "")
	return hexStr[:16]
}

// InitClients initializes git, GitHub, and stack clients
// Returns an error that is suitable for use in PreRunE hooks
func InitClients() (*git.Client, *gh.Client, *stack.Client, error) {
	gitClient, err := git.NewClient()
	if err != nil {
		ui.Error("Not in a git repository")
		return nil, nil, nil, fmt.Errorf("git client initialization failed: %w", err)
	}
	ghClient := gh.NewClient()
	stackClient := stack.NewClient(gitClient, ghClient)
	return gitClient, ghClient, stackClient, nil
}
