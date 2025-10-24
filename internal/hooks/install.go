package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var stackHooks = []string{"prepare-commit-msg", "post-commit", "commit-msg"}

func hookScript(name string) string {
	return fmt.Sprintf(`#!/bin/bash
# Git hook: %s
# Installed by stack - delegates to stack binary
exec stack hook %s "$@"
`, name, name)
}

func InstallHooks(gitRoot string) error {
	hooksDir := filepath.Join(gitRoot, ".git", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	for _, hook := range stackHooks {
		if err := installHook(hooksDir, hook, hookScript(hook)); err != nil {
			return fmt.Errorf("failed to install %s hook: %w", hook, err)
		}
	}

	return nil
}

func UninstallHooks(gitRoot string) error {
	hooksDir := filepath.Join(gitRoot, ".git", "hooks")

	for _, hook := range stackHooks {
		hookPath := filepath.Join(hooksDir, hook)

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

func installHook(hooksDir string, name string, content string) error {
	hookPath := filepath.Join(hooksDir, name)

	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}

func isStackHook(content string) bool {
	return strings.Contains(content, "Installed by stack") ||
		strings.Contains(content, "stack hook")
}
