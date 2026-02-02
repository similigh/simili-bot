// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package gemini

import (
	"fmt"
	"strings"
)

// buildTriagePromptJSON creates a prompt for issue triage analysis with JSON output.
func buildTriagePromptJSON(issue *IssueInput) string {
	return fmt.Sprintf(`You are an AI assistant helping with GitHub issue triage. Analyze the following issue and provide your assessment in JSON format.

Issue Details:
- Title: %s
- Body: %s
- Author: %s
- Current Labels: %s

Analyze:
- Is the issue well-described with clear steps to reproduce (for bugs) or clear requirements (for features)?
- What type of issue is this?
- Are there any red flags (spam, duplicate, off-topic)?

Respond with valid JSON in this exact format:
{
  "quality": "good|needs-improvement|poor",
  "suggested_labels": ["bug", "enhancement", "documentation", "question"],
  "reasoning": "Your brief analysis here",
  "is_duplicate": false,
  "duplicate_reason": ""
}

Note: Only set is_duplicate to true if this appears to be a duplicate of an existing issue.`,
		issue.Title,
		truncate(issue.Body, 1000), // Limit body length
		issue.Author,
		strings.Join(issue.Labels, ", "),
	)
}

// buildTriagePrompt creates a prompt for issue triage analysis (legacy format).
// Deprecated: Use buildTriagePromptJSON for structured output.
func buildTriagePrompt(issue *IssueInput) string {
	return fmt.Sprintf(`You are an AI assistant helping with GitHub issue triage. Analyze the following issue and provide:

1. Quality assessment (good, needs-improvement, or poor)
2. Suggested labels (bug, enhancement, documentation, question, etc.)
3. Brief reasoning for your assessment

Issue Details:
- Title: %s
- Body: %s
- Author: %s
- Current Labels: %s

Provide a concise analysis focusing on:
- Is the issue well-described with clear steps to reproduce (for bugs) or clear requirements (for features)?
- What type of issue is this?
- Are there any red flags (spam, duplicate, off-topic)?

Format your response as:
Quality: [good/needs-improvement/poor]
Labels: [comma-separated list]
Reasoning: [your analysis]`,
		issue.Title,
		truncate(issue.Body, 1000), // Limit body length
		issue.Author,
		strings.Join(issue.Labels, ", "),
	)
}

// buildResponsePrompt creates a prompt for generating a response about similar issues.
func buildResponsePrompt(similar []SimilarIssueInput) string {
	var issueList strings.Builder
	for i, s := range similar {
		status := "open"
		if s.State == "closed" {
			status = "closed"
		}
		issueList.WriteString(fmt.Sprintf("%d. #%d: %s (%.0f%% similar, %s)\n   %s\n",
			i+1, s.Number, s.Title, s.Similarity*100, status, s.URL))
	}

	return fmt.Sprintf(`You are an AI assistant helping users find related GitHub issues. 

The following similar issues were found:

%s

Generate a friendly, helpful comment to inform the user about these similar issues. The comment should:
- Be concise and professional
- Mention that these are AI-detected similar issues
- Encourage the user to check if any of these resolve their question/problem
- If there are closed issues, mention they might contain solutions
- Use markdown formatting for links

Keep the response under 200 words.`,
		issueList.String(),
	)
}

// truncate limits a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
