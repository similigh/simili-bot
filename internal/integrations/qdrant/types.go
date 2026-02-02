// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

// Package qdrant provides the vector database integration.
package qdrant

import "context"

// Point represents a data point in the vector database.
type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// SearchResult represents a single result from a similarity search.
type SearchResult struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
	Vector  []float32              `json:"vector,omitempty"`
}

// VectorStore defines the interface for vector database operations.
type VectorStore interface {
	// CreateCollection creates a new collection if it doesn't exist.
	CreateCollection(ctx context.Context, name string, dimension int) error

	// CollectionExists checks if a collection exists.
	CollectionExists(ctx context.Context, name string) (bool, error)

	// Upsert inserts or updates points in the collection.
	Upsert(ctx context.Context, collectionName string, points []*Point) error

	// Search finds the nearest neighbors for a given vector.
	Search(ctx context.Context, collectionName string, vector []float32, limit int, threshold float64) ([]*SearchResult, error)

	// Delete removes a point by ID.
	Delete(ctx context.Context, collectionName string, id string) error

	// Close closes the connection to the database.
	Close() error
}
