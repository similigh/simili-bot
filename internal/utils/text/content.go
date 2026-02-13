// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-13
// Last Modified: 2026-02-13

package text

import (
	"fmt"
	"strings"
)

// Comment represents a single issue/PR comment for embedding purposes.
type Comment struct {
	Author string
	Body   string
}

// BuildEmbeddingContent constructs the text content used for vector embedding.
// It combines the title, body, and comments into a single string.
// Comments with empty bodies are skipped.
func BuildEmbeddingContent(title, body string, comments []Comment) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n\n", title)

	if b := strings.TrimSpace(body); b != "" {
		fmt.Fprintf(&sb, "Body: %s\n\n", b)
	}

	hasHeader := false
	for _, c := range comments {
		if c.Body == "" {
			continue
		}
		if !hasHeader {
			sb.WriteString("Comments:\n")
			hasHeader = true
		}
		fmt.Fprintf(&sb, "- %s: %s\n", c.Author, c.Body)
	}

	return sb.String()
}
