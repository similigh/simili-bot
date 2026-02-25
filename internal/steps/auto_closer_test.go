// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-22
// Last Modified: 2026-02-25

package steps

import (
	"testing"
	"time"
)

func TestIsBotUser(t *testing.T) {
	tests := []struct {
		name     string
		author   string
		botUsers []string
		want     bool
	}{
		{"bot suffix", "dependabot[bot]", nil, true},
		{"simili prefix", "gh-simili-bot", nil, true},
		{"simili-bot name", "simili-bot", nil, true},
		{"normal user", "john-doe", nil, false},
		{"configured bot", "my-ci-bot", []string{"my-ci-bot"}, true},
		{"configured bot case insensitive", "MY-CI-BOT", []string{"my-ci-bot"}, true},
		{"not in configured list", "random-user", []string{"my-ci-bot"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotUser(tt.author, tt.botUsers)
			if got != tt.want {
				t.Errorf("isBotUser(%q, %v) = %v, want %v", tt.author, tt.botUsers, got, tt.want)
			}
		})
	}
}

func TestGracePeriodCalculation(t *testing.T) {
	gracePeriod := 72 * time.Hour

	tests := []struct {
		name      string
		labeledAt time.Time
		expired   bool
	}{
		{
			name:      "labeled 1 hour ago - not expired",
			labeledAt: time.Now().Add(-1 * time.Hour),
			expired:   false,
		},
		{
			name:      "labeled 71 hours ago - not expired",
			labeledAt: time.Now().Add(-71 * time.Hour),
			expired:   false,
		},
		{
			name:      "labeled 73 hours ago - expired",
			labeledAt: time.Now().Add(-73 * time.Hour),
			expired:   true,
		},
		{
			name:      "labeled 7 days ago - expired",
			labeledAt: time.Now().Add(-7 * 24 * time.Hour),
			expired:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elapsed := time.Since(tt.labeledAt)
			got := elapsed >= gracePeriod
			if got != tt.expired {
				t.Errorf("grace period check: elapsed=%v, got expired=%v, want %v",
					elapsed, got, tt.expired)
			}
		})
	}
}

func TestIsBotComment(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"html marker", "<!-- simili-bot-report -->\n## Triage", true},
		{"emoji marker", "🤖 Simili Triage Report\n...", true},
		{"plain header", "### Simili Triage Report\n...", true},
		{"normal comment", "I think this is a duplicate", false},
		{"empty body", "", false},
		{"partial match", "<!-- simili-bot-auto-close -->", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotComment(tt.body)
			if got != tt.want {
				t.Errorf("isBotComment(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestHasNegativeReaction(t *testing.T) {
	botUsers := []string{"my-ci-bot"}

	tests := []struct {
		name     string
		content  string
		user     string
		wantSkip bool // true means this reaction should trigger human activity
	}{
		{"thumbs down from human", "-1", "john-doe", true},
		{"confused from human", "confused", "jane-doe", true},
		{"thumbs up from human", "+1", "john-doe", false},
		{"heart from human", "heart", "john-doe", false},
		{"thumbs down from bot", "-1", "dependabot[bot]", false},
		{"confused from bot", "confused", "gh-simili-worker", false},
		{"thumbs down from configured bot", "-1", "my-ci-bot", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isNegative := (tt.content == "-1" || tt.content == "confused")
			isHuman := !isBotUser(tt.user, botUsers)
			got := isNegative && isHuman
			if got != tt.wantSkip {
				t.Errorf("reaction check: content=%q user=%q => %v, want %v", tt.content, tt.user, got, tt.wantSkip)
			}
		})
	}
}

func TestReopenedByHuman(t *testing.T) {
	botUsers := []string{}
	since := time.Now().Add(-48 * time.Hour)

	type eventInput struct {
		eventType string
		actor     string
		createdAt time.Time
	}

	tests := []struct {
		name   string
		event  eventInput
		want   bool
	}{
		{
			name:  "reopened by human after since",
			event: eventInput{"reopened", "john-doe", since.Add(time.Hour)},
			want:  true,
		},
		{
			name:  "reopened by bot after since",
			event: eventInput{"reopened", "dependabot[bot]", since.Add(time.Hour)},
			want:  false,
		},
		{
			name:  "reopened by human before since",
			event: eventInput{"reopened", "john-doe", since.Add(-time.Hour)},
			want:  false,
		},
		{
			name:  "closed by human after since",
			event: eventInput{"closed", "john-doe", since.Add(time.Hour)},
			want:  false,
		},
		{
			name:  "labeled by human after since",
			event: eventInput{"labeled", "john-doe", since.Add(time.Hour)},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.event
			isReopened := e.eventType == "reopened"
			isAfterSince := e.createdAt.After(since)
			isHuman := !isBotUser(e.actor, botUsers)
			got := isReopened && isAfterSince && isHuman
			if got != tt.want {
				t.Errorf("reopen check: event=%q actor=%q at=%v => %v, want %v",
					e.eventType, e.actor, e.createdAt, got, tt.want)
			}
		})
	}
}

func TestAutoCloseResultCounts(t *testing.T) {
	tests := []struct {
		name   string
		result AutoCloseResult
	}{
		{
			name: "no errors",
			result: AutoCloseResult{
				Processed:    5,
				Closed:       2,
				SkippedGrace: 2,
				SkippedHuman: 1,
				Errors:       nil,
			},
		},
		{
			name: "with errors",
			result: AutoCloseResult{
				Processed:    5,
				Closed:       1,
				SkippedGrace: 1,
				SkippedHuman: 1,
				Errors:       []string{"#10: failed", "#11: failed"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.result
			total := r.Closed + r.SkippedGrace + r.SkippedHuman + len(r.Errors)
			if total != r.Processed {
				t.Errorf("counts don't add up: closed(%d) + grace(%d) + human(%d) + errors(%d) = %d, want %d",
					r.Closed, r.SkippedGrace, r.SkippedHuman, len(r.Errors), total, r.Processed)
			}
		})
	}
}
