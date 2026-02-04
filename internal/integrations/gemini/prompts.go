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

// buildRouteIssuePrompt creates a prompt for repository routing analysis.
func buildRouteIssuePrompt(input *RouteIssueInput) string {
	var repoList strings.Builder
	for i, r := range input.Repositories {
		repoList.WriteString(fmt.Sprintf("%d. %s/%s: %s\n", i+1, r.Org, r.Repo, r.Description))
	}

	return fmt.Sprintf(`You are an AI assistant helping route GitHub issues to the correct repository.

Issue Details:
- Title: %s
- Body: %s

Available Repositories:
%s

Analyze the issue content and rank ALL repositories by relevance. For each repository, provide:
- Confidence score (0.0-1.0) indicating how well the issue matches the repository
- Brief reasoning for the score

Respond with valid JSON in this exact format:
{
  "rankings": [
    {
      "org": "org-name",
      "repo": "repo-name",
      "confidence": 0.85,
      "reasoning": "Issue describes API errors which are handled by this backend service"
    }
  ]
}

Guidelines:
- Confidence 0.9+ = Very strong match
- Confidence 0.6-0.9 = Possible match
- Confidence <0.6 = Weak or no match
- Include ALL repositories in rankings, even with low confidence`,
		input.Issue.Title,
		truncate(input.Issue.Body, 1000),
		repoList.String(),
	)
}

// buildQualityAssessmentPrompt creates a prompt for issue quality analysis.
func buildQualityAssessmentPrompt(issue *IssueInput) string {
	return fmt.Sprintf(`You are an AI assistant evaluating GitHub issue quality.

Issue Details:
- Title: %s
- Body: %s
- Author: %s

Assess the issue quality based on:
1. Clarity: Is the problem/request clearly described?
2. Completeness: Are there reproduction steps (for bugs) or requirements (for features)?
3. Context: Is there enough background information?
4. Actionability: Can a developer act on this?

Respond with valid JSON in this exact format:
{
  "score": 0.85,
  "assessment": "good",
  "issues": ["Missing error logs", "No environment details"],
  "suggestions": ["Add error messages", "Specify OS and version"],
  "reasoning": "Issue has clear reproduction steps but lacks error logs and environment details"
}

Score scale:
- 0.9-1.0 = excellent (complete, clear, actionable)
- 0.7-0.9 = good (mostly complete, minor improvements needed)
- 0.4-0.7 = needs-improvement (missing key information)
- 0.0-0.4 = poor (unclear or severely incomplete)

Assessment must be one of: "excellent", "good", "needs-improvement", "poor"`,
		issue.Title,
		truncate(issue.Body, 1000),
		issue.Author,
	)
}

// buildDuplicateDetectionPrompt creates a prompt for duplicate detection analysis.
func buildDuplicateDetectionPrompt(input *DuplicateCheckInput) string {
	var similarList strings.Builder
	for i, s := range input.SimilarIssues {
		similarList.WriteString(fmt.Sprintf("%d. #%d: %s (%.0f%% similar, %s)\n",
			i+1, s.Number, s.Title, s.Similarity*100, s.State))
	}

	return fmt.Sprintf(`You are an AI assistant detecting duplicate GitHub issues.

Current Issue:
- Title: %s
- Body: %s

Similar Issues Found:
%s

Analyze whether the current issue is a duplicate of any similar issues. Consider:
1. Are they describing the same problem/feature?
2. Do they have the same root cause?
3. Would fixing one resolve the other?

Note: High vector similarity doesn't always mean duplicate. Issues can be related but distinct.

Respond with valid JSON in this exact format:
{
  "is_duplicate": true,
  "duplicate_of": 42,
  "confidence": 0.9,
  "reasoning": "Both issues describe the same API timeout error with identical reproduction steps",
  "similar_issues": [38, 45]
}

Confidence guidelines:
- 0.9+ = Very likely duplicate
- 0.7-0.9 = Probable duplicate
- 0.5-0.7 = Possibly related
- <0.5 = Different issues

Only set is_duplicate to true if confidence >= 0.8`,
		input.CurrentIssue.Title,
		truncate(input.CurrentIssue.Body, 1000),
		similarList.String(),
	)
}

// truncate limits a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
