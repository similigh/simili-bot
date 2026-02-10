// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-05

package gemini

import (
	"fmt"
	"strings"
)

// truncate limits a string to a maximum length in runes (UTF-8 safe).
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// indentText indents each non-empty line of text with the given prefix.
func indentText(text string, indent string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

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
	hasDefinitions := false

	for i, r := range input.Repositories {
		repoList.WriteString(fmt.Sprintf("%d. %s/%s\n", i+1, r.Org, r.Repo))

		// Include full definition if available
		if r.Definition != "" {
			hasDefinitions = true
			// Truncate if too long (max 2000 chars per repo to prevent token explosion)
			definition := r.Definition
			if len(definition) > 2000 {
				definition = definition[:2000] + "\n... (truncated)"
			}
			repoList.WriteString(fmt.Sprintf("   Documentation:\n%s\n\n", indentText(definition, "   ")))
		} else if r.Description != "" {
			// Fallback to short description from config
			repoList.WriteString(fmt.Sprintf("   Description: %s\n", r.Description))
		}
		repoList.WriteString("\n")
	}

	// Build weighting guidance based on available information
	weightingGuidance := ""
	if hasDefinitions {
		weightingGuidance = `
Decision-Making Framework:
- Primary consideration (~60% weight): Match the issue against repository documentation
  to understand what each repo is responsible for and what problems it solves
- Secondary consideration (~40% weight): Consider any historical patterns or precedents
  from similar issues, if available
- The repository documentation provides the authoritative definition of what belongs where
- When documentation clearly indicates a match, prioritize it over other signals`
	} else {
		weightingGuidance = `
Note: Limited repository documentation available. Base routing primarily on repository descriptions.`
	}

	// Add current repo context if provided
	currentRepoGuidance := ""
	if input.CurrentRepo != "" {
		currentRepoGuidance = fmt.Sprintf(`
IMPORTANT - Current Repository Context:
- This issue was created in: %s
- Default assumption: Issues should STAY in their current repository unless there is CLEAR evidence they belong elsewhere
- Only recommend transfer if you are CONFIDENT (>= 0.7) the issue truly belongs in a different repository
- When in doubt, keep the issue in its current repository
`, input.CurrentRepo)
	}

	return fmt.Sprintf(`You are an AI assistant helping route GitHub issues to the correct repository.

Issue Details:
- Title: %s
- Body: %s
%s
Available Repositories:
%s
%s

Task: Analyze the issue content and rank ALL repositories by relevance.

For each repository, provide:
- Confidence score (0.0-1.0) indicating how well the issue matches the repository
- Brief reasoning explaining your score

Respond with valid JSON in this exact format:
{
  "rankings": [
    {
      "org": "org-name",
      "repo": "repo-name",
      "confidence": 0.85,
      "reasoning": "Issue describes API authentication errors. The backend repository's documentation indicates it handles the authentication service and API layer."
    }
  ]
}

Confidence Score Guidelines:
- 0.9+ = Very strong match (issue clearly aligns with repo's documented responsibilities)
- 0.7-0.9 = Strong match (issue has significant alignment with repo's purpose)
- 0.5-0.7 = Moderate match (some alignment but not definitive)
- <0.5 = Weak or no match (little to no alignment with repo's documented purpose)

Important: Include ALL repositories in your rankings, even those with low confidence scores.`,
		input.Issue.Title,
		truncate(input.Issue.Body, 1000),
		currentRepoGuidance,
		repoList.String(),
		weightingGuidance,
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
		body := truncate(s.Body, 500)
		fmt.Fprintf(&similarList, "--- Similar Issue %d ---\n", i+1)
		fmt.Fprintf(&similarList, "Issue #%d [%s]: %s\nVector similarity: %.0f%%\n",
			s.Number, s.State, s.Title, s.Similarity*100)
		if body != "" {
			fmt.Fprintf(&similarList, "Content:\n%s\n", body)
		}
		similarList.WriteString("\n")
	}

	return fmt.Sprintf(`You are a precise duplicate detection system for GitHub issues.

CRITICAL DISTINCTION:
- DUPLICATE: Two issues describe the EXACT SAME bug or feature request. Fixing one FULLY resolves the other. They must have the same root cause AND the same expected outcome.
- RELATED: Two issues are in the same area or component but describe DIFFERENT problems. They may share keywords or affect the same module, but have different root causes or expected outcomes.

Being related is NOT enough to be a duplicate. Most issues in the same project will be related.

Current Issue:
- Title: %s
- Body: %s

Similar Issues Found (by vector similarity — high similarity does NOT mean duplicate):
%s

Compare the FULL CONTENT of the current issue against each similar issue. Look for:
1. Same root cause — not just same component or area
2. Same expected outcome — not just similar symptoms
3. Would a single fix resolve BOTH issues completely?

If two issues affect the same module but describe different failure modes, different inputs, or different expected behaviors, they are RELATED, not duplicates.

Respond with valid JSON:
{
  "is_duplicate": false,
  "duplicate_of": 0,
  "confidence": 0.0,
  "reasoning": "Brief explanation",
  "similar_issues": []
}

Confidence scale (be strict):
- 0.95+ = Certain duplicate (identical problem, identical root cause)
- 0.85-0.95 = Very likely duplicate (same root cause, same expected fix)
- 0.70-0.85 = Related but likely distinct issues
- <0.70 = Different issues

ONLY set is_duplicate to true if confidence >= 0.85. When in doubt, set is_duplicate to false.`,
		input.CurrentIssue.Title,
		truncate(input.CurrentIssue.Body, 1000),
		similarList.String(),
	)
}
