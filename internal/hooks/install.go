package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var stackHooks = []string{"prepare-commit-msg", "post-commit", "commit-msg"}

// hookScript generates a git hook script for the given hook name
func hookScript(name string) string {
	return fmt.Sprintf(`#!/bin/bash
# Git hook: %s
# Installed by stack - delegates to stack binary
exec stack hook %s "$@"
`, name, name)
}

// InstallHooks installs git hooks for stack
func InstallHooks(gitRoot string) error {
	hooksDir := filepath.Join(gitRoot, ".git", "hooks")

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Install all hooks
	for _, hook := range stackHooks {
		if err := installHook(hooksDir, hook, hookScript(hook)); err != nil {
			return fmt.Errorf("failed to install %s hook: %w", hook, err)
		}
	}

	return nil
}

// UninstallHooks removes git hooks installed by stack
func UninstallHooks(gitRoot string) error {
	hooksDir := filepath.Join(gitRoot, ".git", "hooks")

	for _, hook := range stackHooks {
		hookPath := filepath.Join(hooksDir, hook)

		// Only remove if it's our hook (contains "stack hook")
		content, err := os.ReadFile(hookPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to read hook %s: %w", hook, err)
		}

		if !isStackHook(string(content)) {
			continue
		}

		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("failed to remove hook %s: %w", hook, err)
		}
	}

	return nil
}

// CheckHooksInstalled verifies that stack hooks are installed
func CheckHooksInstalled(gitRoot string) bool {
	hooksDir := filepath.Join(gitRoot, ".git", "hooks")

	for _, hook := range stackHooks {
		hookPath := filepath.Join(hooksDir, hook)

		content, err := os.ReadFile(hookPath)
		if err != nil {
			return false
		}

		if !isStackHook(string(content)) {
			return false
		}
	}

	return true
}

// installHook writes a hook script to the hooks directory
func installHook(hooksDir string, name string, content string) error {
	hookPath := filepath.Join(hooksDir, name)

	// Write hook file
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}

// isStackHook checks if a hook file was created by stack
func isStackHook(content string) bool {
	return strings.Contains(content, "Installed by stack") ||
		strings.Contains(content, "stack hook")
}
