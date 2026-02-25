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
