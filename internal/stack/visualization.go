package stack

import (
	"fmt"
	"strings"
)

func GenerateStackVisualization(stackCtx *StackContext, currentPRNumber int) string {
	var sb strings.Builder

	totalPRs := len(stackCtx.ActiveChanges)
	sb.WriteString(fmt.Sprintf("## 📚 Stack: %s (%d PRs)\n\n", stackCtx.StackName, totalPRs))

	currentPosition := 0
	for _, change := range stackCtx.ActiveChanges {
		if !change.IsLocal() && change.PR.PRNumber == currentPRNumber {
			currentPosition = change.Position
			break
		}
	}

	sb.WriteString("| # | PR | Status | Title |\n")
	sb.WriteString("|---|-----|---------|---------------------------------------|\n")

	for _, change := range stackCtx.ActiveChanges {
		prLabel := "-"
		if !change.IsLocal() {
			prLabel = change.PR.URL
		}

		status := "local"
		if !change.IsLocal() {
			status = change.PR.State
		}
		statusEmoji, statusText := getStatusDisplay(status)

		row := fmt.Sprintf("| %d | %s | %s %s | %s",
			change.Position, prLabel, statusEmoji, statusText, change.Title)

		if change.Position == currentPosition {
			row += " ← **YOU ARE HERE**"
		}

		sb.WriteString(row + " |\n")
	}

	sb.WriteString("\n**Merge order:** `" + stackCtx.Stack.Base)
	for _, change := range stackCtx.ActiveChanges {
		if !change.IsLocal() {
			sb.WriteString(fmt.Sprintf(" → #%d", change.PR.PRNumber))
		}
	}
	sb.WriteString("`\n\n---\n\n")

	sb.WriteString("💡 **Review tip:** Start from the bottom (")
	if len(stackCtx.ActiveChanges) > 0 {
		firstChange := stackCtx.ActiveChanges[0]
		if !firstChange.IsLocal() {
			sb.WriteString(fmt.Sprintf("[#%d](%s)", firstChange.PR.PRNumber, firstChange.PR.URL))
		}
	}
	sb.WriteString(") for full context\n\n")

	sb.WriteString(fmt.Sprintf("<!-- stack-visualization: %s -->\n", stackCtx.StackName))

	return sb.String()
}

func getStatusDisplay(status string) (emoji, text string) {
	switch status {
	case "open":
		return "✅", "Open  "
	case "draft":
		return "📝", "Draft "
	case "merged":
		return "🟣", "Merged"
	case "closed":
		return "❌", "Closed"
	default:
		return "⚪", "Local "
	}
}

func (c *Client) SyncVisualizationComments(stackCtx *StackContext) error {
	for _, change := range stackCtx.ActiveChanges {
		if change.IsLocal() {
			continue
		}

		vizContent := GenerateStackVisualization(stackCtx, change.PR.PRNumber)

		if err := c.syncCommentForPR(stackCtx.StackName, change.UUID, change.PR, vizContent); err != nil {
			return fmt.Errorf("failed to sync comment for PR #%d: %w", change.PR.PRNumber, err)
		}
	}

	return nil
}

func (c *Client) syncCommentForPR(stackName, uuid string, pr *PR, vizContent string) error {
	marker := fmt.Sprintf("<!-- stack-visualization: %s -->", stackName)

	if pr.VizCommentID != "" {
		err := c.gh.UpdatePRComment(pr.VizCommentID, vizContent)
		if err == nil {
			return nil
		}
		fmt.Printf("Warning: Failed to update cached comment for PR #%d, will search for it\n", pr.PRNumber)
	}

	comments, err := c.gh.ListPRComments(pr.PRNumber)
	if err != nil {
		return fmt.Errorf("failed to list comments: %w", err)
	}

	var existingCommentID string
	for _, comment := range comments {
		if strings.Contains(comment.Body, marker) {
			existingCommentID = comment.ID
			break
		}
	}

	if existingCommentID != "" {
		if err := c.gh.UpdatePRComment(existingCommentID, vizContent); err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		c.cacheCommentID(stackName, uuid, existingCommentID, pr)
	} else {
		commentID, err := c.gh.CreatePRComment(pr.PRNumber, vizContent)
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
		c.cacheCommentID(stackName, uuid, commentID, pr)
	}

	return nil
}

func (c *Client) cacheCommentID(stackName, uuid, commentID string, pr *PR) {
	if pr.VizCommentID != commentID {
		pr.VizCommentID = commentID
		if err := c.SetPR(stackName, uuid, pr); err != nil {
			fmt.Printf("Warning: Failed to cache comment ID for PR #%d: %v\n", pr.PRNumber, err)
		}
	}
}
