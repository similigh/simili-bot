// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

package transfer

import (
	"sort"
	"strings"

	"github.com/similigh/simili-bot/internal/core/config"
)

// RuleMatcher evaluates transfer rules against issues.
type RuleMatcher struct {
	rules []config.TransferRule
}

// NewRuleMatcher creates a new RuleMatcher with the given rules.
// It filters out disabled rules and sorts by priority (descending).
func NewRuleMatcher(rules []config.TransferRule) *RuleMatcher {
	// Filter enabled rules
	enabled := make([]config.TransferRule, 0, len(rules))
	for _, r := range rules {
		if r.Enabled == nil || *r.Enabled {
			enabled = append(enabled, r)
		}
	}

	// Sort by priority (descending - higher priority first)
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority > enabled[j].Priority
	})

	return &RuleMatcher{rules: enabled}
}

// Match evaluates all rules against the issue and returns the first match.
// Returns a MatchResult with Matched=false if no rules match.
func (m *RuleMatcher) Match(issue *IssueInput) *MatchResult {
	for i := range m.rules {
		rule := &m.rules[i]
		if m.evaluateRule(rule, issue) {
			return &MatchResult{
				Matched: true,
				Rule:    rule,
				Target:  rule.Target,
				Reason:  "Matched rule: " + rule.Name,
			}
		}
	}
	return &MatchResult{Matched: false}
}

// evaluateRule checks if an issue matches a single rule.
// All specified conditions must match (AND logic between condition types).
func (m *RuleMatcher) evaluateRule(rule *config.TransferRule, issue *IssueInput) bool {
	// Labels (AND): ALL must match
	if len(rule.Labels) > 0 {
		if !m.matchLabelsAll(issue.Labels, rule.Labels) {
			return false
		}
	}

	// LabelsAny (OR): ANY must match
	if len(rule.LabelsAny) > 0 {
		if !m.matchLabelsAny(issue.Labels, rule.LabelsAny) {
			return false
		}
	}

	// TitleContains (OR): ANY must match
	if len(rule.TitleContains) > 0 {
		if !m.matchContainsAny(issue.Title, rule.TitleContains) {
			return false
		}
	}

	// BodyContains (OR): ANY must match
	if len(rule.BodyContains) > 0 {
		if !m.matchContainsAny(issue.Body, rule.BodyContains) {
			return false
		}
	}

	// Author (OR): ANY must match
	if len(rule.Author) > 0 {
		if !m.matchAuthor(issue.Author, rule.Author) {
			return false
		}
	}

	return true
}

// matchLabelsAll returns true if all required labels are present (case-insensitive).
func (m *RuleMatcher) matchLabelsAll(issueLabels, requiredLabels []string) bool {
	labelSet := make(map[string]bool, len(issueLabels))
	for _, l := range issueLabels {
		labelSet[strings.ToLower(l)] = true
	}

	for _, required := range requiredLabels {
		if !labelSet[strings.ToLower(required)] {
			return false
		}
	}
	return true
}

// matchLabelsAny returns true if any of the required labels are present (case-insensitive).
func (m *RuleMatcher) matchLabelsAny(issueLabels, requiredLabels []string) bool {
	labelSet := make(map[string]bool, len(issueLabels))
	for _, l := range issueLabels {
		labelSet[strings.ToLower(l)] = true
	}

	for _, required := range requiredLabels {
		if labelSet[strings.ToLower(required)] {
			return true
		}
	}
	return false
}

// matchContainsAny returns true if the text contains any of the patterns (case-insensitive).
func (m *RuleMatcher) matchContainsAny(text string, patterns []string) bool {
	lowerText := strings.ToLower(text)
	for _, pattern := range patterns {
		if strings.Contains(lowerText, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// matchAuthor returns true if the issue author matches any in the list (case-insensitive).
func (m *RuleMatcher) matchAuthor(author string, authors []string) bool {
	lowerAuthor := strings.ToLower(author)
	for _, a := range authors {
		if strings.ToLower(a) == lowerAuthor {
			return true
		}
	}
	return false
}
