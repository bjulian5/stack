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

const ShortHashLength = 7

func (c *Commit) ShortHash() string {
	return ShortHash(c.Hash)
}

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

	commitMsg.Title = strings.TrimSpace(lines[0])

	// Find where trailers start
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

		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && !strings.Contains(parts[0], " ") {
				inTrailers = true
				continue
			}
		}

		if inTrailers {
			trailerStart = i + 1
			break
		}
	}

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

	bodyLines := []string{}
	for i := 1; i < trailerStart; i++ {
		bodyLines = append(bodyLines, lines[i])
	}

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

	result.WriteString(c.Title)
	result.WriteString("\n")

	if c.Body != "" {
		result.WriteString("\n")
		result.WriteString(c.Body)
		result.WriteString("\n")
	}

	if len(c.Trailers) > 0 {
		result.WriteString("\n")
		for key, value := range c.Trailers {
			result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	return result.String()
}
