package steps

import "testing"

func TestNormalizeSimilarThreadType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "missing defaults to issue", input: "", expected: "issue"},
		{name: "issue stays issue", input: "issue", expected: "issue"},
		{name: "uppercase issue", input: "ISSUE", expected: "issue"},
		{name: "pr stays pr", input: "pr", expected: "pr"},
		{name: "pull request alias", input: "pull_request", expected: "pr"},
		{name: "pull request with space", input: "pull request", expected: "pr"},
		{name: "unknown defaults to issue", input: "discussion", expected: "issue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSimilarThreadType(tt.input)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
