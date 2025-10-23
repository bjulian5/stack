package ui

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Confirm prompts the user to type the expected value to confirm an action.
// Returns true if the user input matches expectedValue (case-sensitive).
func Confirm(prompt string, expectedValue string) bool {
	reader := bufio.NewReader(os.Stdin)
	os.Stdout.WriteString(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input) == expectedValue
}

// PromptSelection prompts user to select items from a list.
// Supports: 'all', 'none', or comma-separated indices (1-indexed).
// Returns selected indices (0-indexed) or nil if cancelled.
func PromptSelection(prompt string, itemCount int) []int {
	reader := bufio.NewReader(os.Stdin)
	os.Stdout.WriteString(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" || strings.ToLower(input) == "none" {
		return nil
	}

	if strings.ToLower(input) == "all" {
		indices := make([]int, itemCount)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	parts := strings.Split(input, ",")
	var selected []int
	seen := make(map[int]bool)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		index, err := strconv.Atoi(part)
		if err != nil || index < 1 || index > itemCount {
			continue
		}
		zeroIdx := index - 1
		if !seen[zeroIdx] {
			selected = append(selected, zeroIdx)
			seen[zeroIdx] = true
		}
	}

	return selected
}
