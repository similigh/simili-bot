// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-13
// Last Modified: 2026-02-13

package text

import (
	"testing"
)

func TestBuildEmbeddingContent(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		body     string
		comments []Comment
		want     string
	}{
		{
			name:  "title and body only",
			title: "My Issue",
			body:  "Some body text",
			want:  "Title: My Issue\n\nBody: Some body text\n\n",
		},
		{
			name:  "title body and multiple comments",
			title: "My Issue",
			body:  "Some body text",
			comments: []Comment{
				{Author: "alice", Body: "First comment"},
				{Author: "bob", Body: "Second comment"},
			},
			want: "Title: My Issue\n\nBody: Some body text\n\nComments:\n- alice: First comment\n- bob: Second comment\n",
		},
		{
			name:  "empty body",
			title: "My Issue",
			body:  "",
			comments: []Comment{
				{Author: "alice", Body: "A comment"},
			},
			want: "Title: My Issue\n\nComments:\n- alice: A comment\n",
		},
		{
			name:  "whitespace-only body treated as empty",
			title: "My Issue",
			body:  "   \n  ",
			comments: []Comment{
				{Author: "alice", Body: "A comment"},
			},
			want: "Title: My Issue\n\nComments:\n- alice: A comment\n",
		},
		{
			name:  "empty comment bodies skipped",
			title: "My Issue",
			body:  "Body",
			comments: []Comment{
				{Author: "alice", Body: ""},
				{Author: "bob", Body: "Valid comment"},
			},
			want: "Title: My Issue\n\nBody: Body\n\nComments:\n- bob: Valid comment\n",
		},
		{
			name:     "nil comments slice",
			title:    "My Issue",
			body:     "Body text",
			comments: nil,
			want:     "Title: My Issue\n\nBody: Body text\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEmbeddingContent(tt.title, tt.body, tt.comments)
			if got != tt.want {
				t.Errorf("BuildEmbeddingContent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
