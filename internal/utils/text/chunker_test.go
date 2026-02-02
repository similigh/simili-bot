// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package text

import (
	"testing"
)

func TestSplitText(t *testing.T) {
	splitter := NewRecursiveCharacterSplitter()
	splitter.config.ChunkSize = 10 // Small chunk size for testing
	splitter.config.ChunkOverlap = 0

	text := "Hello World This Is A Test"
	chunks := splitter.SplitText(text)

	if len(chunks) == 0 {
		t.Fatal("Expected chunks, got none")
	}

	for _, chunk := range chunks {
		if len(chunk) > splitter.config.ChunkSize {
			t.Errorf("Chunk size %d exceeds limit %d: %q", len(chunk), splitter.config.ChunkSize, chunk)
		}
	}
}

func TestSplitTextWithSeparators(t *testing.T) {
	splitter := NewRecursiveCharacterSplitter()
	// Config to force split on newlines
	splitter.config.ChunkSize = 20
	splitter.config.Separators = []string{"\n", " "}

	text := "Line 1 is long\nLine 2 is also long"
	chunks := splitter.SplitText(text)

	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d: %v", len(chunks), chunks)
	}

	if chunks[0] != "Line 1 is long" {
		t.Errorf("Expected first chunk 'Line 1 is long', got %q", chunks[0])
	}
}
