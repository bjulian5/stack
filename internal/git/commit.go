package git

import (
	"fmt"
	"strings"
)

// Commit represents a git commit
type Commit struct {
	Hash     string
	Title    string
	Body     string
	Message  string
	Trailers map[string]string
}

// ParseCommitMessage parses a commit message into title, body, and trailers
func ParseCommitMessage(hash string, message string) Commit {
	lines := strings.Split(message, "\n")

	commit := Commit{
		Hash:     hash,
		Message:  message,
		Trailers: make(map[string]string),
	}

	if len(lines) == 0 {
		return commit
	}

	// First line is the title
	commit.Title = strings.TrimSpace(lines[0])

	// Find where trailers start (last non-empty block with Key: Value format)
	trailerStart := len(lines)
	inTrailers := false

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if inTrailers {
				trailerStart = i + 1
				break
			}
			continue
		}

		// Check if this line looks like a trailer
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && !strings.Contains(parts[0], " ") {
				inTrailers = true
				continue
			}
		}

		// If we hit a non-trailer line while in trailers, we're done
		if inTrailers {
			trailerStart = i + 1
			break
		}
	}

	// Parse trailers
	for i := trailerStart; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			commit.Trailers[key] = value
		}
	}

	// Body is everything between title and trailers
	bodyLines := []string{}
	for i := 1; i < trailerStart; i++ {
		bodyLines = append(bodyLines, lines[i])
	}

	// Trim leading and trailing empty lines from body
	body := strings.Join(bodyLines, "\n")
	commit.Body = strings.TrimSpace(body)

	return commit
}

// GetTrailer extracts a specific trailer from a commit message
func GetTrailer(message string, key string) string {
	commit := ParseCommitMessage("", message)
	return commit.Trailers[key]
}

// AddTrailer adds a trailer to a commit message
func AddTrailer(message string, key string, value string) string {
	// Ensure message ends with newline
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}

	return message + fmt.Sprintf("%s: %s\n", key, value)
}
