// services/stream-management-service/internal/models/stream.go
package models

import (
	"time"
)

type StreamStatus string

const (
	StreamStatusPending StreamStatus = "pending"
	StreamStatusLive    StreamStatus = "live"
	StreamStatusEnded   StreamStatus = "ended"
	StreamStatusError   StreamStatus = "error"
)

type Stream struct {
	ID           string            `json:"id" dynamodbav:"id"`
	UserID       int64             `json:"user_id" dynamodbav:"user_id"`
	StreamKey    string            `json:"stream_key" dynamodbav:"stream_key"`
	Title        string            `json:"title" dynamodbav:"title"`
	Status       StreamStatus      `json:"status" dynamodbav:"status"`
	StartedAt    *time.Time        `json:"started_at,omitempty" dynamodbav:"started_at,omitempty"`
	EndedAt      *time.Time        `json:"ended_at,omitempty" dynamodbav:"ended_at,omitempty"`
	Duration     int64             `json:"duration" dynamodbav:"duration"` // seconds
	ViewerCount  int               `json:"viewer_count" dynamodbav:"viewer_count"`
	RecordingURL string            `json:"recording_url,omitempty" dynamodbav:"recording_url,omitempty"`
	Metadata     map[string]string `json:"metadata" dynamodbav:"metadata"`
	CreatedAt    time.Time         `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at" dynamodbav:"updated_at"`
}

type StreamMetadata struct {
	Resolution string `json:"resolution"`
	Bitrate    int    `json:"bitrate"`
	FPS        int    `json:"fps"`
	Codec      string `json:"codec"`
	ClientIP   string `json:"client_ip"`
}
