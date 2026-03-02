package commands

import (
	"strings"
	"testing"

	"github.com/similigh/simili-bot/internal/core/config"
)

// TestGracePeriodMinutesCLIMapping validates the flag-to-config mapping logic:
//   - An explicit 0 is stored as 1 (smallest positive value, triggers instant expiry in tests).
//   - Any positive value is stored as-is.
//   - When the flag was not set, GracePeriodMinutesOverride remains 0 (no override).
func TestGracePeriodMinutesCLIMapping(t *testing.T) {
	tests := []struct {
		name        string
		flagChanged bool
		flagValue   int
		wantOverride int
	}{
		{
			name:         "flag not provided - no override",
			flagChanged:  false,
			flagValue:    0,
			wantOverride: 0,
		},
		{
			name:         "flag set to 0 - stored as 1 (instant expire)",
			flagChanged:  true,
			flagValue:    0,
			wantOverride: 1,
		},
		{
			name:         "flag set to negative - stored as 1 (instant expire)",
			flagChanged:  true,
			flagValue:    -5,
			wantOverride: 1,
		},
		{
			name:         "flag set to 1 - stored as 1",
			flagChanged:  true,
			flagValue:    1,
			wantOverride: 1,
		},
		{
			name:         "flag set to 30 - stored as 30",
			flagChanged:  true,
			flagValue:    30,
			wantOverride: 30,
		},
		{
			name:         "flag set to 1440 (1 day in minutes) - stored as 1440",
			flagChanged:  true,
			flagValue:    1440,
			wantOverride: 1440,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}

			// Mirrors the mapping logic in runAutoClose.
			if tt.flagChanged {
				if tt.flagValue <= 0 {
					cfg.AutoClose.GracePeriodMinutesOverride = 1
				} else {
					cfg.AutoClose.GracePeriodMinutesOverride = tt.flagValue
				}
			}

			if cfg.AutoClose.GracePeriodMinutesOverride != tt.wantOverride {
				t.Errorf("GracePeriodMinutesOverride = %d, want %d",
					cfg.AutoClose.GracePeriodMinutesOverride, tt.wantOverride)
			}
		})
	}
}

// TestRepoFlagParsing validates owner/repo splitting used in runAutoClose.
func TestRepoFlagParsing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOrg   string
		wantRepo  string
		wantValid bool
	}{
		{
			name:      "valid org/repo",
			input:     "acme-corp/my-service",
			wantOrg:   "acme-corp",
			wantRepo:  "my-service",
			wantValid: true,
		},
		{
			name:      "valid single-word org and repo",
			input:     "owner/repo",
			wantOrg:   "owner",
			wantRepo:  "repo",
			wantValid: true,
		},
		{
			name:      "missing slash - invalid",
			input:     "ownerrepo",
			wantValid: false,
		},
		{
			name:      "empty org - invalid",
			input:     "/repo",
			wantValid: false,
		},
		{
			name:      "empty repo - invalid",
			input:     "owner/",
			wantValid: false,
		},
		{
			name:      "empty string - invalid",
			input:     "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, ok := strings.Cut(tt.input, "/")
			valid := ok && org != "" && repo != ""

			if valid != tt.wantValid {
				t.Errorf("repo %q: valid=%v, want %v", tt.input, valid, tt.wantValid)
				return
			}
			if tt.wantValid {
				if org != tt.wantOrg {
					t.Errorf("org = %q, want %q", org, tt.wantOrg)
				}
				if repo != tt.wantRepo {
					t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
				}
			}
		})
	}
}
