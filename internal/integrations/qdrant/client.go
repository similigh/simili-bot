// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package qdrant

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client implements the VectorStore interface for Qdrant.
type Client struct {
	conn        *grpc.ClientConn
	collections pb.CollectionsClient
	points      pb.PointsClient
	apiKey      string
	timeout     time.Duration
}

// NewClient creates a new Qdrant client.
// URL can be in formats: "localhost:6334", "host:port", "https://cloud.qdrant.io:6334"
// TLS is automatically enabled for URLs containing "https://" or cloud-like domains.
func NewClient(url, apiKey string) (*Client, error) {
	// Determine target and TLS requirement
	target := url
	useTLS := false

	// Strip protocol if present and determine TLS
	if strings.HasPrefix(url, "https://") {
		target = strings.TrimPrefix(url, "https://")
		useTLS = true
	} else if strings.HasPrefix(url, "http://") {
		target = strings.TrimPrefix(url, "http://")
		useTLS = false
	} else {
		// No explicit protocol - check for cloud indicators
		useTLS = strings.Contains(strings.ToLower(url), "cloud") ||
			strings.Contains(strings.ToLower(url), ".qdrant.io")
	}

	// Create gRPC connection with appropriate credentials
	var opts []grpc.DialOption
	if useTLS {
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(nil)),
		}
	} else {
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &Client{
		conn:        conn,
		collections: pb.NewCollectionsClient(conn),
		points:      pb.NewPointsClient(conn),
		apiKey:      apiKey,
		timeout:     30 * time.Second,
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ctxWithAuth adds authentication to an existing context with timeout.
func (c *Client) ctxWithAuth(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	if c.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "api-key", c.apiKey)
	}
	return ctx, cancel
}

// CreateCollection creates a new collection if it doesn't exist.
func (c *Client) CreateCollection(ctx context.Context, name string, dimension int) error {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	// Check if exists first
	exists, err := c.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create collection
	_, err = c.collections.Create(authCtx, &pb.CreateCollection{
		CollectionName: name,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     uint64(dimension),
					Distance: pb.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// CollectionExists checks if a collection exists.
func (c *Client) CollectionExists(ctx context.Context, name string) (bool, error) {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	resp, err := c.collections.List(authCtx, &pb.ListCollectionsRequest{})
	if err != nil {
		return false, fmt.Errorf("failed to list collections: %w", err)
	}

	for _, col := range resp.Collections {
		if col.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// Upsert inserts or updates points in the collection.
func (c *Client) Upsert(ctx context.Context, collectionName string, points []*Point) error {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	qPoints := make([]*pb.PointStruct, len(points))
	for i, p := range points {
		// Convert payload map to Qdrant struct
		payload := make(map[string]*pb.Value)
		for k, v := range p.Payload {
			payload[k] = toQdrantValue(v)
		}

		// Parse UUID string to PointId
		pointID := &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: p.ID,
			},
		}

		qPoints[i] = &pb.PointStruct{
			Id:      pointID,
			Vectors: &pb.Vectors{VectorsOptions: &pb.Vectors_Vector{Vector: &pb.Vector{Data: p.Vector}}},
			Payload: payload,
		}
	}

	_, err := c.points.Upsert(authCtx, &pb.UpsertPoints{
		CollectionName: collectionName,
		Points:         qPoints,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	return nil
}

// Search finds the nearest neighbors for a given vector.
func (c *Client) Search(ctx context.Context, collectionName string, vector []float32, limit int, threshold float64) ([]*SearchResult, error) {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	scoreThreshold := float32(threshold)
	resp, err := c.points.Search(authCtx, &pb.SearchPoints{
		CollectionName: collectionName,
		Vector:         vector,
		Limit:          uint64(limit),
		ScoreThreshold: &scoreThreshold,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	results := make([]*SearchResult, len(resp.Result))
	for i, hit := range resp.Result {
		// Convert payload back to map
		payload := make(map[string]interface{})
		for k, v := range hit.Payload {
			payload[k] = fromQdrantValue(v)
		}

		id := ""
		if hit.Id.GetUuid() != "" {
			id = hit.Id.GetUuid()
		} else {
			id = fmt.Sprintf("%d", hit.Id.GetNum())
		}

		results[i] = &SearchResult{
			ID:      id,
			Score:   hit.Score,
			Payload: payload,
		}
	}

	return results, nil
}

// Delete removes a point by ID.
func (c *Client) Delete(ctx context.Context, collectionName string, id string) error {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	// Handle UUID vs Num ID logic if needed, simplify for now assuming UUID
	pointID := &pb.PointId{
		PointIdOptions: &pb.PointId_Uuid{
			Uuid: id,
		},
	}

	_, err := c.points.Delete(authCtx, &pb.DeletePoints{
		CollectionName: collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{pointID},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete point: %w", err)
	}

	return nil
}

// SetPayload updates payload fields on existing points without re-uploading vectors.
func (c *Client) SetPayload(ctx context.Context, collectionName string, id string, payload map[string]interface{}) error {
	authCtx, cancel := c.ctxWithAuth(ctx)
	defer cancel()

	qPayload := make(map[string]*pb.Value)
	for k, v := range payload {
		qPayload[k] = toQdrantValue(v)
	}

	pointID := &pb.PointId{
		PointIdOptions: &pb.PointId_Uuid{
			Uuid: id,
		},
	}

	_, err := c.points.SetPayload(authCtx, &pb.SetPayloadPoints{
		CollectionName: collectionName,
		Payload:        qPayload,
		PointsSelector: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{pointID},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set payload: %w", err)
	}

	return nil
}

// Helper to convert Go value to Qdrant Value
func toQdrantValue(v interface{}) *pb.Value {
	switch val := v.(type) {
	case string:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: val}}
	case int:
		return &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: int64(val)}}
	case int64:
		return &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: val}}
	case float64:
		return &pb.Value{Kind: &pb.Value_DoubleValue{DoubleValue: val}}
	case bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val}}
	default:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}

// Helper to convert Qdrant Value to Go value
func fromQdrantValue(v *pb.Value) interface{} {
	switch k := v.Kind.(type) {
	case *pb.Value_StringValue:
		return k.StringValue
	case *pb.Value_IntegerValue:
		return k.IntegerValue
	case *pb.Value_DoubleValue:
		return k.DoubleValue
	case *pb.Value_BoolValue:
		return k.BoolValue
	default:
		return nil
	}
}
