// services/stream-management-service/internal/service/stream_service.go
package service

import (
	"encoding/json"
	"fmt"
	_ "log"
	"strconv"
	"time"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/repository"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/aws"
	"github.com/gin-gonic/gin"
)

type StreamService struct {
	config        *config.Config
	dynamoRepo    *repository.DynamoDBRepository
	redisRepo     *repository.RedisRepository
	kinesisClient *aws.KinesisClient
	s3Client      *aws.S3Client
}

func NewStreamService(cfg *config.Config, dynamoRepo *repository.DynamoDBRepository, redisRepo *repository.RedisRepository) *StreamService {
	return &StreamService{
		config:        cfg,
		dynamoRepo:    dynamoRepo,
		redisRepo:     redisRepo,
		kinesisClient: aws.NewKinesisClient(cfg.AWSRegion, cfg.KinesisStreamName),
		s3Client:      aws.NewS3Client(cfg.AWSRegion, cfg.S3BucketName),
	}
}

func (s *StreamService) CreateStream(stream *models.Stream) (string, error) {
	// Generate unique stream ID
	stream.ID = s.generateStreamID()

	// Store in DynamoDB
	err := s.dynamoRepo.CreateStream(stream)
	if err != nil {
		return "", fmt.Errorf("failed to create stream in DynamoDB: %w", err)
	}

	// Cache in Redis
	streamJSON, _ := json.Marshal(stream)
	s.redisRepo.SetStreamData(stream.ID, string(streamJSON), 24*time.Hour)

	return stream.ID, nil
}

func (s *StreamService) GetStreamByID(c *gin.Context) {
	streamID := c.Param("id")

	// Try Redis first
	streamData, err := s.redisRepo.GetStreamData(streamID)
	if err == nil && streamData != "" {
		var stream models.Stream
		if json.Unmarshal([]byte(streamData), &stream) == nil {
			c.JSON(200, stream)
			return
		}
	}

	// Fallback to DynamoDB
	stream, err := s.dynamoRepo.GetStreamByID(streamID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Stream not found"})
		return
	}

	c.JSON(200, stream)
}

func (s *StreamService) GetActiveStreams(c *gin.Context) {
	streams, err := s.dynamoRepo.GetStreamsByStatus(models.StreamStatusLive)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not get active streams"})
		return
	}

	c.JSON(200, gin.H{
		"streams": streams,
		"count":   len(streams),
	})
}

func (s *StreamService) EndStream(streamKey string, duration string) error {
	// Find stream by stream key
	stream, err := s.dynamoRepo.GetStreamByStreamKey(streamKey)
	if err != nil {
		return fmt.Errorf("stream not found: %w", err)
	}

	// Parse duration
	durationSec := int64(0)
	if duration != "" {
		if d, err := strconv.ParseInt(duration, 10, 64); err == nil {
			durationSec = d
		}
	}

	// Update stream
	now := time.Now()
	stream.Status = models.StreamStatusEnded
	stream.EndedAt = &now
	stream.Duration = durationSec
	stream.UpdatedAt = now

	// Update in DynamoDB
	err = s.dynamoRepo.UpdateStream(stream)
	if err != nil {
		return fmt.Errorf("failed to update stream: %w", err)
	}

	// Update cache
	streamJSON, _ := json.Marshal(stream)
	s.redisRepo.SetStreamData(stream.ID, string(streamJSON), time.Hour)

	// Publish stream ended event
	event := map[string]interface{}{
		"event_type": "stream_ended",
		"stream_id":  stream.ID,
		"user_id":    stream.UserID,
		"duration":   durationSec,
		"timestamp":  time.Now().Unix(),
	}
	s.PublishEvent(event)

	return nil
}

func (s *StreamService) UpdateStreamRecording(streamKey string, filePath string) error {
	// Find stream by stream key
	stream, err := s.dynamoRepo.GetStreamByStreamKey(streamKey)
	if err != nil {
		return fmt.Errorf("stream not found: %w", err)
	}

	// Upload to S3 (optional, or just store the file path)
	recordingURL := filePath // For now, just store the path
	// TODO: Implement S3 upload if needed
	// recordingURL, err = s.s3Client.UploadRecording(filePath)

	// Update stream with recording URL
	stream.RecordingURL = recordingURL
	stream.UpdatedAt = time.Now()

	err = s.dynamoRepo.UpdateStream(stream)
	if err != nil {
		return fmt.Errorf("failed to update stream recording: %w", err)
	}

	// Update cache
	streamJSON, _ := json.Marshal(stream)
	s.redisRepo.SetStreamData(stream.ID, string(streamJSON), time.Hour)

	return nil
}

func (s *StreamService) StoreStreamSession(streamKey string, sessionData map[string]interface{}) error {
	sessionJSON, _ := json.Marshal(sessionData)
	return s.redisRepo.SetStreamSession(streamKey, string(sessionJSON), time.Hour)
}

func (s *StreamService) GetStreamSession(streamKey string) (map[string]interface{}, error) {
	sessionData, err := s.redisRepo.GetStreamSession(streamKey)
	if err != nil {
		return nil, err
	}

	var session map[string]interface{}
	err = json.Unmarshal([]byte(sessionData), &session)
	return session, err
}

func (s *StreamService) CleanupStreamSession(streamKey string) error {
	return s.redisRepo.DeleteStreamSession(streamKey)
}

func (s *StreamService) PublishEvent(event map[string]interface{}) error {
	eventJSON, _ := json.Marshal(event)
	return s.kinesisClient.PutRecord(string(eventJSON))
}

//func (s *StreamService) generateStreamID() string {
//	bytes := make([]byte, 16)
//	rand.Read(bytes)
//	return hex.EncodeToString(bytes)
//}
