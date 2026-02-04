// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-04
// Last Modified: 2026-02-04

package transfer

import (
	"testing"

	"github.com/similigh/simili-bot/internal/core/config"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestNewRuleMatcher_FiltersDisabledRules(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "enabled-default", Target: "org/repo1"},
		{Name: "enabled-explicit", Target: "org/repo2", Enabled: boolPtr(true)},
		{Name: "disabled", Target: "org/repo3", Enabled: boolPtr(false)},
	}

	matcher := NewRuleMatcher(rules)

	if len(matcher.rules) != 2 {
		t.Errorf("Expected 2 enabled rules, got %d", len(matcher.rules))
	}
}

func TestNewRuleMatcher_SortsByPriority(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "low", Priority: 1, Target: "org/low"},
		{Name: "high", Priority: 100, Target: "org/high"},
		{Name: "medium", Priority: 50, Target: "org/medium"},
	}

	matcher := NewRuleMatcher(rules)

	if matcher.rules[0].Name != "high" {
		t.Errorf("Expected first rule to be 'high', got '%s'", matcher.rules[0].Name)
	}
	if matcher.rules[1].Name != "medium" {
		t.Errorf("Expected second rule to be 'medium', got '%s'", matcher.rules[1].Name)
	}
	if matcher.rules[2].Name != "low" {
		t.Errorf("Expected third rule to be 'low', got '%s'", matcher.rules[2].Name)
	}
}

func TestMatch_LabelsAll(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "all-labels", Target: "org/target", Labels: []string{"bug", "urgent"}},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		labels  []string
		matched bool
	}{
		{"all match", []string{"bug", "urgent", "extra"}, true},
		{"missing one", []string{"bug"}, false},
		{"none match", []string{"feature"}, false},
		{"case insensitive", []string{"BUG", "URGENT"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &IssueInput{Labels: tt.labels}
			result := matcher.Match(issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_LabelsAny(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "any-labels", Target: "org/target", LabelsAny: []string{"bug", "urgent"}},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		labels  []string
		matched bool
	}{
		{"both match", []string{"bug", "urgent"}, true},
		{"one matches", []string{"bug"}, true},
		{"none match", []string{"feature"}, false},
		{"case insensitive", []string{"BUG"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &IssueInput{Labels: tt.labels}
			result := matcher.Match(issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_TitleContains(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "title-rule", Target: "org/target", TitleContains: []string{"crash", "error"}},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		title   string
		matched bool
	}{
		{"contains crash", "App crash on startup", true},
		{"contains error", "Error handling issue", true},
		{"no match", "Feature request", false},
		{"case insensitive", "CRASH reported", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &IssueInput{Title: tt.title}
			result := matcher.Match(issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_BodyContains(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "body-rule", Target: "org/target", BodyContains: []string{"stack trace", "exception"}},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		body    string
		matched bool
	}{
		{"contains stack trace", "Here is the stack trace:\n...", true},
		{"contains exception", "Got an exception", true},
		{"no match", "Feature description", false},
		{"case insensitive", "STACK TRACE follows", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &IssueInput{Body: tt.body}
			result := matcher.Match(issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_Author(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "author-rule", Target: "org/target", Author: []string{"bot-user", "ci-user"}},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		author  string
		matched bool
	}{
		{"bot-user matches", "bot-user", true},
		{"ci-user matches", "ci-user", true},
		{"no match", "human-user", false},
		{"case insensitive", "BOT-USER", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &IssueInput{Author: tt.author}
			result := matcher.Match(issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_MultipleConditions(t *testing.T) {
	rules := []config.TransferRule{
		{
			Name:          "multi-condition",
			Target:        "org/target",
			Labels:        []string{"bug"},
			TitleContains: []string{"crash"},
		},
	}
	matcher := NewRuleMatcher(rules)

	tests := []struct {
		name    string
		issue   *IssueInput
		matched bool
	}{
		{
			"both conditions match",
			&IssueInput{Labels: []string{"bug"}, Title: "App crash"},
			true,
		},
		{
			"only labels match",
			&IssueInput{Labels: []string{"bug"}, Title: "Feature request"},
			false,
		},
		{
			"only title matches",
			&IssueInput{Labels: []string{"feature"}, Title: "App crash"},
			false,
		},
		{
			"neither matches",
			&IssueInput{Labels: []string{"feature"}, Title: "Feature request"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(tt.issue)
			if result.Matched != tt.matched {
				t.Errorf("Expected matched=%v, got %v", tt.matched, result.Matched)
			}
		})
	}
}

func TestMatch_PriorityOrder(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "low-priority", Priority: 1, Target: "org/low", Labels: []string{"bug"}},
		{Name: "high-priority", Priority: 100, Target: "org/high", Labels: []string{"bug"}},
	}
	matcher := NewRuleMatcher(rules)

	issue := &IssueInput{Labels: []string{"bug"}}
	result := matcher.Match(issue)

	if !result.Matched {
		t.Fatal("Expected a match")
	}
	if result.Target != "org/high" {
		t.Errorf("Expected target 'org/high', got '%s'", result.Target)
	}
	if result.Rule.Name != "high-priority" {
		t.Errorf("Expected rule 'high-priority', got '%s'", result.Rule.Name)
	}
}

func TestMatch_NoRules(t *testing.T) {
	matcher := NewRuleMatcher(nil)
	issue := &IssueInput{Labels: []string{"bug"}}
	result := matcher.Match(issue)

	if result.Matched {
		t.Error("Expected no match with empty rules")
	}
}

func TestMatch_EmptyConditions(t *testing.T) {
	// Rule with no conditions should match everything
	rules := []config.TransferRule{
		{Name: "catch-all", Target: "org/target"},
	}
	matcher := NewRuleMatcher(rules)

	issue := &IssueInput{
		Title:  "Any title",
		Body:   "Any body",
		Labels: []string{"any-label"},
		Author: "any-author",
	}
	result := matcher.Match(issue)

	if !result.Matched {
		t.Error("Expected match for rule with no conditions")
	}
}

func TestMatchResult_Fields(t *testing.T) {
	rules := []config.TransferRule{
		{Name: "test-rule", Target: "org/target", Labels: []string{"bug"}},
	}
	matcher := NewRuleMatcher(rules)

	issue := &IssueInput{Labels: []string{"bug"}}
	result := matcher.Match(issue)

	if !result.Matched {
		t.Fatal("Expected a match")
	}
	if result.Target != "org/target" {
		t.Errorf("Expected target 'org/target', got '%s'", result.Target)
	}
	if result.Rule == nil {
		t.Fatal("Expected Rule to be set")
	}
	if result.Rule.Name != "test-rule" {
		t.Errorf("Expected rule name 'test-rule', got '%s'", result.Rule.Name)
	}
	if result.Reason == "" {
		t.Error("Expected Reason to be set")
	}
}
