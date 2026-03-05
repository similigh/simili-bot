// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-03-05
// Last Modified: 2026-03-05

package commands

import (
	"strings"
	"testing"
)

func TestBuildPREmbeddingContent(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		body        string
		files       []string
		wantHas     []string
		wantAbsent  []string
	}{
		{
			name:    "with files",
			title:   "Fix auth bug",
			body:    "Resolves auth issue in login flow",
			files:   []string{"pkg/auth/auth.go", "pkg/auth/auth_test.go"},
			wantHas: []string{"Title: Fix auth bug", "Body: Resolves auth issue in login flow", "Changed Files:", "- pkg/auth/auth.go", "- pkg/auth/auth_test.go"},
		},
		{
			name:       "without files",
			title:      "Update docs",
			body:       "Improves documentation",
			files:      nil,
			wantHas:    []string{"Title: Update docs", "Body: Improves documentation"},
			wantAbsent: []string{"Changed Files:"},
		},
		{
			name:       "empty files slice",
			title:      "Refactor",
			body:       "General refactor",
			files:      []string{},
			wantHas:    []string{"Title: Refactor", "Body: General refactor"},
			wantAbsent: []string{"Changed Files:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPREmbeddingContent(tt.title, tt.body, tt.files)

			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("expected content to contain %q\ngot:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("expected content NOT to contain %q\ngot:\n%s", absent, got)
				}
			}
		})
	}
}
