// services/stream-management-service/internal/service/stream_service_extended.go
package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	_ "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/repository"
	_ "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/aws"
)

// Add these methods to the existing StreamService struct

// GetStreamByIDInternal gets a stream by ID for internal use (used by gRPC server)
func (s *StreamService) GetStreamByIDInternal(streamID string) (*models.Stream, error) {
	// Try Redis first
	streamData, err := s.redisRepo.GetStreamData(streamID)
	if err == nil && streamData != "" {
		var stream models.Stream
		if json.Unmarshal([]byte(streamData), &stream) == nil {
			return &stream, nil
		}
	}

	// Fallback to DynamoDB
	return s.dynamoRepo.GetStreamByID(streamID)
}

// GetStreamByStreamKeyInternal gets a stream by stream key for internal use
func (s *StreamService) GetStreamByStreamKeyInternal(streamKey string) (*models.Stream, error) {
	return s.dynamoRepo.GetStreamByStreamKey(streamKey)
}

// GetActiveStreamsInternal gets active streams for internal use (used by gRPC server)
func (s *StreamService) GetActiveStreamsInternal() ([]*models.Stream, error) {
	return s.dynamoRepo.GetStreamsByStatus(models.StreamStatusLive)
}

// UpdateStreamInternal updates a stream for internal use (used by gRPC server)
func (s *StreamService) UpdateStreamInternal(stream *models.Stream) error {
	// Update in DynamoDB
	err := s.dynamoRepo.UpdateStream(stream)
	if err != nil {
		return fmt.Errorf("failed to update stream in DynamoDB: %w", err)
	}

	// Update cache
	streamJSON, _ := json.Marshal(stream)
	s.redisRepo.SetStreamData(stream.ID, string(streamJSON), 24*time.Hour)

	return nil
}

// ValidateStreamKeyInternal validates a stream key internally
func (s *StreamService) ValidateStreamKeyInternal(streamKey, ipAddress string) (bool, int64, string, error) {
	// This method would typically call the User Service
	// For now, we'll implement basic validation

	// Check if stream key exists and is valid format
	if len(streamKey) < 10 {
		return false, 0, "", nil
	}

	// TODO: Implement proper validation with User Service
	// For development, we'll allow any stream key with valid format
	return true, 123, "test_user", nil
}

// Additional utility methods for stream management

// GetStreamMetrics gets various metrics for a stream
func (s *StreamService) GetStreamMetrics(streamID string) (map[string]interface{}, error) {
	stream, err := s.GetStreamByIDInternal(streamID)
	if err != nil {
		return nil, err
	}

	metrics := map[string]interface{}{
		"stream_id":     stream.ID,
		"user_id":       stream.UserID,
		"status":        stream.Status,
		"viewer_count":  stream.ViewerCount,
		"duration":      stream.Duration,
		"started_at":    stream.StartedAt,
		"ended_at":      stream.EndedAt,
		"recording_url": stream.RecordingURL,
	}

	// Add uptime if stream is live
	if stream.Status == models.StreamStatusLive && stream.StartedAt != nil {
		uptime := time.Since(*stream.StartedAt)
		metrics["uptime_seconds"] = int64(uptime.Seconds())
		metrics["uptime_minutes"] = int64(uptime.Minutes())
	}

	return metrics, nil
}

// GetUserStreams gets all streams for a specific user
func (s *StreamService) GetUserStreams(userID int64, limit int) ([]*models.Stream, error) {
	// This would require a GSI on user_id in DynamoDB
	// For now, we'll scan (not efficient for production)
	allStreams, err := s.dynamoRepo.GetStreamsByStatus(models.StreamStatusLive)
	if err != nil {
		return nil, err
	}

	var userStreams []*models.Stream
	count := 0
	for _, stream := range allStreams {
		if stream.UserID == userID {
			userStreams = append(userStreams, stream)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	return userStreams, nil
}

// UpdateViewerCount updates the viewer count for a stream
func (s *StreamService) UpdateViewerCount(streamID string, viewerCount int) error {
	stream, err := s.GetStreamByIDInternal(streamID)
	if err != nil {
		return err
	}

	stream.ViewerCount = viewerCount
	stream.UpdatedAt = time.Now()

	return s.UpdateStreamInternal(stream)
}

// StartStreamRecording initiates recording for a stream
func (s *StreamService) StartStreamRecording(streamID string) error {
	stream, err := s.GetStreamByIDInternal(streamID)
	if err != nil {
		return err
	}

	// Add recording metadata
	if stream.Metadata == nil {
		stream.Metadata = make(map[string]string)
	}
	stream.Metadata["recording_started"] = time.Now().Format(time.RFC3339)
	stream.Metadata["recording_status"] = "started"

	return s.UpdateStreamInternal(stream)
}

// StopStreamRecording stops recording for a stream
func (s *StreamService) StopStreamRecording(streamID, recordingPath string) error {
	stream, err := s.GetStreamByIDInternal(streamID)
	if err != nil {
		return err
	}

	// Update recording info
	stream.RecordingURL = recordingPath
	if stream.Metadata == nil {
		stream.Metadata = make(map[string]string)
	}
	stream.Metadata["recording_completed"] = time.Now().Format(time.RFC3339)
	stream.Metadata["recording_status"] = "completed"

	return s.UpdateStreamInternal(stream)
}

// GetPlatformStats gets platform-wide statistics
func (s *StreamService) GetPlatformStats() (map[string]interface{}, error) {
	liveStreams, err := s.GetActiveStreamsInternal()
	if err != nil {
		return nil, err
	}

	totalViewers := 0
	for _, stream := range liveStreams {
		totalViewers += stream.ViewerCount
	}

	stats := map[string]interface{}{
		"live_streams":  len(liveStreams),
		"total_viewers": totalViewers,
		"last_updated":  time.Now().Unix(),
	}

	// Cache stats in Redis for quick access
	statsJSON, _ := json.Marshal(stats)
	s.redisRepo.SetStreamData("platform_stats", string(statsJSON), 1*time.Minute)

	return stats, nil
}

// CleanupExpiredStreams cleans up streams that have been stuck in "live" status
func (s *StreamService) CleanupExpiredStreams() error {
	liveStreams, err := s.GetActiveStreamsInternal()
	if err != nil {
		return err
	}

	expiredCount := 0
	now := time.Now()

	for _, stream := range liveStreams {
		// Consider streams expired if they've been live for more than 12 hours without updates
		if stream.StartedAt != nil && now.Sub(*stream.StartedAt) > 12*time.Hour {
			if stream.UpdatedAt.Before(now.Add(-1 * time.Hour)) {
				// Mark as ended
				stream.Status = models.StreamStatusEnded
				stream.EndedAt = &now
				stream.Duration = int64(now.Sub(*stream.StartedAt).Seconds())
				stream.UpdatedAt = now

				if err := s.UpdateStreamInternal(stream); err != nil {
					continue // Skip this one and continue
				}

				// Publish cleanup event
				event := map[string]interface{}{
					"event_type": "stream_cleanup",
					"stream_id":  stream.ID,
					"user_id":    stream.UserID,
					"reason":     "expired",
					"timestamp":  now.Unix(),
				}
				s.PublishEvent(event)

				expiredCount++
			}
		}
	}

	if expiredCount > 0 {
		fmt.Printf("🧹 Cleaned up %d expired streams", expiredCount)
	}

	return nil
}

// SearchStreams searches for streams based on criteria
func (s *StreamService) SearchStreams(query string, status models.StreamStatus, limit int) ([]*models.Stream, error) {
	var streams []*models.Stream
	var err error

	if status != "" {
		streams, err = s.dynamoRepo.GetStreamsByStatus(status)
	} else {
		// Get all live streams as default
		streams, err = s.dynamoRepo.GetStreamsByStatus(models.StreamStatusLive)
	}

	if err != nil {
		return nil, err
	}

	// If no query, return streams with limit
	if query == "" {
		if limit > 0 && len(streams) > limit {
			return streams[:limit], nil
		}
		return streams, nil
	}

	// Simple text search in title and stream key
	var filtered []*models.Stream
	query = strings.ToLower(query)

	for _, stream := range streams {
		if strings.Contains(strings.ToLower(stream.Title), query) ||
			strings.Contains(strings.ToLower(stream.StreamKey), query) {
			filtered = append(filtered, stream)
			if limit > 0 && len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// Helper method to generate stream IDs
func (s *StreamService) generateStreamID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "stream_" + hex.EncodeToString(bytes)[:16]
}
