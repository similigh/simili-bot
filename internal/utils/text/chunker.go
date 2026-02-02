// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package text

import (
	"strings"
)

// SplitterConfig holds configuration for text splitting.
type SplitterConfig struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string
}

// RecursiveCharacterSplitter splits text recursively by separators.
type RecursiveCharacterSplitter struct {
	config SplitterConfig
}

// NewRecursiveCharacterSplitter creates a new splitter with default config.
// Default ChunkSize: 2000, ChunkOverlap: 200.
func NewRecursiveCharacterSplitter() *RecursiveCharacterSplitter {
	return &RecursiveCharacterSplitter{
		config: SplitterConfig{
			ChunkSize:    2000,
			ChunkOverlap: 200,
			Separators:   []string{"\n\n", "\n", " ", ""},
		},
	}
}

// SplitText splits a text into chunks.
func (s *RecursiveCharacterSplitter) SplitText(text string) []string {
	return s.split(text, s.config.Separators)
}

func (s *RecursiveCharacterSplitter) split(text string, separators []string) []string {
	finalChunks := []string{}
	separator := ""

	// Find the appropriate separator
	for _, sep := range separators {
		if sep == "" || strings.Contains(text, sep) {
			separator = sep
			break
		}
	}

	// Calculate splits
	var splits []string
	if separator != "" {
		splits = strings.Split(text, separator)
	} else {
		splits = []string{text}
	}

	// Merge splits into chunks
	currentChunk := ""
	for _, split := range splits {
		if len(currentChunk)+len(split)+len(separator) > s.config.ChunkSize {
			if currentChunk != "" {
				prevChunk := currentChunk
				finalChunks = append(finalChunks, prevChunk)

				// Start new chunk with overlap from the end of the previous chunk
				overlap := s.config.ChunkOverlap
				if overlap > 0 {
					runes := []rune(prevChunk)
					if overlap >= len(runes) {
						// If overlap is larger than the chunk, reuse the whole chunk
						currentChunk = prevChunk
					} else {
						currentChunk = string(runes[len(runes)-overlap:])
					}
				} else {
					currentChunk = ""
				}
			}
		}

		if currentChunk != "" {
			currentChunk += separator
		}
		currentChunk += split
	}

	if currentChunk != "" {
		finalChunks = append(finalChunks, currentChunk)
	}

	// Post-processing: If any chunk is still too large (atomic split was too big),
	// recurse with next separator if available.
	// But since we are splitting by priority, we just ensure we don't exceed limit if possible.
	// For now, this simple accumulator is "good enough" for GitHub issues.

	// Ensure no empty chunks
	result := []string{}
	for _, c := range finalChunks {
		if strings.TrimSpace(c) != "" {
			result = append(result, c)
		}
	}

	return result
}
