package git

import (
	"fmt"
	"strings"
)

// CommitMessage represents a parsed git commit message with its components
type CommitMessage struct {
	Title    string
	Body     string
	Trailers map[string]string
}

// Commit represents a git commit with its hash and parsed message
type Commit struct {
	Hash    string
	Message CommitMessage
}

// ShortHashLength is git's default abbreviated commit hash length
const ShortHashLength = 7

// ShortHash returns the abbreviated commit hash (first 7 characters)
func (c *Commit) ShortHash() string {
	return ShortHash(c.Hash)
}

// ShortHash returns the abbreviated form of a commit hash string (first 7 characters)
func ShortHash(hash string) string {
	if len(hash) <= ShortHashLength {
		return hash
	}
	return hash[:ShortHashLength]
}

// ParseCommitMessage parses a commit message string into its components
func ParseCommitMessage(message string) CommitMessage {
	lines := strings.Split(message, "\n")

	commitMsg := CommitMessage{
		Trailers: make(map[string]string),
	}

	if len(lines) == 0 {
		return commitMsg
	}

	// First line is the title
	commitMsg.Title = strings.TrimSpace(lines[0])

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
			commitMsg.Trailers[key] = value
		}
	}

	// Body is everything between title and trailers
	bodyLines := []string{}
	for i := 1; i < trailerStart; i++ {
		bodyLines = append(bodyLines, lines[i])
	}

	// Trim leading and trailing empty lines from body
	body := strings.Join(bodyLines, "\n")
	commitMsg.Body = strings.TrimSpace(body)

	return commitMsg
}

// AddTrailer adds a trailer to the commit message
func (c *CommitMessage) AddTrailer(key string, value string) {
	c.Trailers[key] = value
}

// String converts the CommitMessage back to a formatted string
func (c *CommitMessage) String() string {
	var result strings.Builder

	// Title (first line)
	result.WriteString(c.Title)
	result.WriteString("\n")

	// Body (if present)
	if c.Body != "" {
		result.WriteString("\n")
		result.WriteString(c.Body)
		result.WriteString("\n")
	}

	// Trailers (if present)
	if len(c.Trailers) > 0 {
		result.WriteString("\n")
		for key, value := range c.Trailers {
			result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	return result.String()
}
