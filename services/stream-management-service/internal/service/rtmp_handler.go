// services/stream-management-service/internal/service/rtmp_handler.go
package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	grpcClient "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/grpc"
)

// For now, we'll create a placeholder interface until gRPC is fully set up
type StreamServiceClient interface {
	ValidateStreamKey(ctx context.Context, req *ValidateStreamKeyRequest) (*ValidateStreamKeyResponse, error)
	CreateStream(ctx context.Context, req *CreateStreamRequest) (*CreateStreamResponse, error)
	EndStream(ctx context.Context, req *EndStreamRequest) (*EndStreamResponse, error)
	RecordingCompleted(ctx context.Context, req *RecordingCompletedRequest) (*RecordingCompletedResponse, error)
	GetStream(ctx context.Context, req *GetStreamRequest) (*GetStreamResponse, error)
}

// Temporary message types until protobuf is generated
type ValidateStreamKeyRequest struct {
	StreamKey string
	IpAddress string
	AppName   string
}

type ValidateStreamKeyResponse struct {
	Status      *Status
	IsValid     bool
	UserId      int64
	Username    string
	Permissions *StreamPermissions
}

type CreateStreamRequest struct {
	UserId    int64
	StreamKey string
	Title     string
	Metadata  *StreamMetadata
}

type CreateStreamResponse struct {
	Status   *Status
	StreamId string
}

type EndStreamRequest struct {
	StreamId        string
	DurationSeconds int64
	RecordingPath   string
}

type EndStreamResponse struct {
	Status *Status
}

type RecordingCompletedRequest struct {
	StreamId        string
	RecordingPath   string
	FileSizeBytes   int64
	DurationSeconds int64
}

type RecordingCompletedResponse struct {
	Status       *Status
	RecordingUrl string
}

type GetStreamRequest struct {
	StreamId string
}

type GetStreamResponse struct {
	Status *Status
	Stream *Stream
}

type Status struct {
	Code    int32
	Message string
	Success bool
}

type StreamPermissions struct {
	CanStream          bool
	CanRecord          bool
	MaxBitrate         int32
	MaxDurationMinutes int32
}

type StreamMetadata struct {
	ClientIp   string
	AppName    string
	CustomData map[string]string
}

type Stream struct {
	Id          string
	UserId      int64
	StreamKey   string
	Title       string
	Status      string
	ViewerCount int64
	Metadata    *StreamMetadata
}

type RTMPHandler struct {
	config           *config.Config
	streamService    *StreamService
	userClient       *grpcClient.UserServiceClient
	grpcStreamClient StreamServiceClient // Interface instead of concrete gRPC client
}

type RTMPAuthRequest struct {
	Name   string `json:"name" form:"name"`     // Stream key from media server
	IP     string `json:"addr" form:"addr"`     // Client IP
	App    string `json:"app" form:"app"`       // Application name
	Swfurl string `json:"swfurl" form:"swfurl"` // SWF URL
	Tcurl  string `json:"tcurl" form:"tcurl"`   // TC URL
	Vhost  string `json:"vhost" form:"vhost"`   // Virtual host
}

type RTMPStreamRequest struct {
	Name     string `json:"name" form:"name"`         // Stream key
	IP       string `json:"addr" form:"addr"`         // Client IP
	App      string `json:"app" form:"app"`           // Application name
	Duration string `json:"duration" form:"duration"` // Duration in seconds (for ended streams)
	File     string `json:"file" form:"file"`         // Recording file path
	Size     string `json:"size" form:"size"`         // File size
}

func NewRTMPHandler(cfg *config.Config, streamService *StreamService, userClient *grpcClient.UserServiceClient) *RTMPHandler {
	return &RTMPHandler{
		config:        cfg,
		streamService: streamService,
		userClient:    userClient,
	}
}

// SetGRPCClient sets the gRPC stream client for internal communication
func (h *RTMPHandler) SetGRPCClient(client StreamServiceClient) {
	h.grpcStreamClient = client
}

func (h *RTMPHandler) AuthenticateStream(c *gin.Context) {
	var req RTMPAuthRequest

	// Try to bind JSON first, then form data
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("❌ Error parsing auth request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	}

	log.Printf("🔑 RTMP Auth Request - Name: %s, IP: %s, App: %s", req.Name, req.IP, req.App)

	// Extract stream key from name (media server might send app/stream_key format)
	streamKey := h.extractStreamKey(req.Name)
	log.Printf("🔍 Extracted stream key: %s", streamKey)

	// For now, let's use HTTP fallback to User Service instead of gRPC
	// This will work until we have the full gRPC setup
	valid, userID, username, err := h.validateStreamKeyHTTP(streamKey, req.IP)
	if err != nil {
		log.Printf("❌ Error validating stream key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
			"code":  "VALIDATION_FAILED",
		})
		return
	}

	if !valid {
		log.Printf("❌ Invalid stream key: %s", streamKey)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Invalid stream key",
			"code":  "INVALID_STREAM_KEY",
		})
		return
	}

	log.Printf("✅ Stream authorized - User: %s (ID: %d), Key: %s", username, userID, streamKey)

	// Store stream session info in Redis for quick access
	sessionData := map[string]interface{}{
		"user_id":    userID,
		"username":   username,
		"stream_key": streamKey,
		"client_ip":  req.IP,
		"app_name":   req.App,
		"started_at": time.Now().Unix(),
		"permissions": map[string]interface{}{
			"can_stream":           true,
			"can_record":           true,
			"max_bitrate":          8000,
			"max_duration_minutes": 240,
		},
	}

	if err := h.streamService.StoreStreamSession(streamKey, sessionData); err != nil {
		log.Printf("⚠️ Warning: Could not store stream session: %v", err)
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"authorized": true,
		"user_id":    userID,
		"username":   username,
		"permissions": gin.H{
			"can_stream":           true,
			"can_record":           true,
			"max_bitrate":          8000,
			"max_duration_minutes": 240,
		},
	})
}

func (h *RTMPHandler) StreamStarted(c *gin.Context) {
	var req RTMPStreamRequest

	// Try to bind JSON first, then form data
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("❌ Error parsing stream started request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	}

	log.Printf("🔴 Stream STARTED - Name: %s, IP: %s", req.Name, req.IP)

	streamKey := h.extractStreamKey(req.Name)

	// Get session info from Redis
	sessionData, err := h.streamService.GetStreamSession(streamKey)
	if err != nil {
		log.Printf("❌ Could not get stream session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session not found"})
		return
	}

	userID, ok := sessionData["user_id"].(float64)
	if !ok {
		log.Printf("❌ Invalid user_id in session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session"})
		return
	}

	// Create stream record using direct StreamService method
	stream := &models.Stream{
		UserID:    int64(userID),
		StreamKey: streamKey,
		Title:     fmt.Sprintf("Live Stream - %s", time.Now().Format("2006-01-02 15:04")),
		Status:    models.StreamStatusLive,
		Metadata: map[string]string{
			"client_ip":       req.IP,
			"app_name":        req.App,
			"session_started": time.Now().Format(time.RFC3339),
			"rtmp_app":        req.App,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	now := time.Now()
	stream.StartedAt = &now

	streamID, err := h.streamService.CreateStream(stream)
	if err != nil {
		log.Printf("❌ Error creating stream: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create stream"})
		return
	}

	log.Printf("✅ Stream created with ID: %s", streamID)

	// Update session with stream ID
	sessionData["stream_id"] = streamID
	sessionData["stream_started_at"] = time.Now().Unix()
	h.streamService.StoreStreamSession(streamKey, sessionData)

	// Publish stream started event to Kinesis
	event := map[string]interface{}{
		"event_type": "stream_started",
		"stream_id":  streamID,
		"user_id":    userID,
		"timestamp":  time.Now().Unix(),
		"metadata": map[string]interface{}{
			"stream_key": streamKey,
			"client_ip":  req.IP,
			"app_name":   req.App,
		},
	}

	if err := h.streamService.PublishEvent(event); err != nil {
		log.Printf("⚠️ Warning: Could not publish stream started event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Stream started",
		"stream_id": streamID,
		"status":    "live",
	})
}

func (h *RTMPHandler) StreamEnded(c *gin.Context) {
	var req RTMPStreamRequest

	// Try to bind JSON first, then form data
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("❌ Error parsing stream ended request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	}

	log.Printf("🔴 Stream ENDED - Name: %s, Duration: %s", req.Name, req.Duration)

	streamKey := h.extractStreamKey(req.Name)

	// Get session info to find stream ID
	sessionData, err := h.streamService.GetStreamSession(streamKey)
	if err != nil {
		log.Printf("❌ Could not get stream session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session not found"})
		return
	}

	streamID, ok := sessionData["stream_id"].(string)
	if !ok {
		log.Printf("❌ No stream ID in session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stream ID not found in session"})
		return
	}

	// Parse duration
	durationSec := int64(0)
	if req.Duration != "" {
		if d, err := strconv.ParseInt(req.Duration, 10, 64); err == nil {
			durationSec = d
		}
	}

	// End stream using direct StreamService method
	err = h.streamService.EndStream(streamKey, req.Duration)
	if err != nil {
		log.Printf("❌ Error ending stream: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not end stream"})
		return
	}

	// Clean up session
	if err := h.streamService.CleanupStreamSession(streamKey); err != nil {
		log.Printf("⚠️ Warning: Could not cleanup stream session: %v", err)
	}

	log.Printf("✅ Stream ended successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":   "Stream ended",
		"stream_id": streamID,
		"duration":  durationSec,
		"status":    "ended",
	})
}

func (h *RTMPHandler) RecordingCompleted(c *gin.Context) {
	var req RTMPStreamRequest

	// Try to bind JSON first, then form data
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("❌ Error parsing recording completed request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	}

	log.Printf("📹 Recording COMPLETED - Name: %s, File: %s", req.Name, req.File)

	streamKey := h.extractStreamKey(req.Name)

	// Update stream with recording info using direct StreamService method
	err := h.streamService.UpdateStreamRecording(streamKey, req.File)
	if err != nil {
		log.Printf("❌ Error updating stream recording: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update recording info"})
		return
	}

	log.Printf("✅ Recording updated successfully")

	// Parse file size if provided
	fileSize := int64(0)
	if req.Size != "" {
		if s, err := strconv.ParseInt(req.Size, 10, 64); err == nil {
			fileSize = s
		}
	}

	// Parse duration if provided
	durationSec := int64(0)
	if req.Duration != "" {
		if d, err := strconv.ParseInt(req.Duration, 10, 64); err == nil {
			durationSec = d
		}
	}

	// Publish recording completed event
	event := map[string]interface{}{
		"event_type":     "recording_completed",
		"stream_key":     streamKey,
		"recording_path": req.File,
		"file_size":      fileSize,
		"duration":       durationSec,
		"timestamp":      time.Now().Unix(),
	}

	if err := h.streamService.PublishEvent(event); err != nil {
		log.Printf("⚠️ Warning: Could not publish recording completed event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Recording completed",
		"recording_url": req.File,
		"file_size":     fileSize,
		"status":        "completed",
	})
}

// Health check endpoint for media server callbacks
func (h *RTMPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "stream-management",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"rtmp":      "ready",
	})
}

// Utility function to extract stream key from various formats
func (h *RTMPHandler) extractStreamKey(name string) string {
	// Handle different formats:
	// - "streamkey" (direct)
	// - "live/streamkey" (app/key)
	// - "/live/streamkey" (with leading slash)

	streamKey := strings.TrimSpace(name)

	// Remove leading slash if present
	streamKey = strings.TrimPrefix(streamKey, "/")

	// If contains slash, take the last part
	if strings.Contains(streamKey, "/") {
		parts := strings.Split(streamKey, "/")
		streamKey = parts[len(parts)-1]
	}

	return streamKey
}

// HTTP fallback method to validate stream key with User Service REST API
func (h *RTMPHandler) validateStreamKeyHTTP(streamKey, ipAddress string) (bool, int64, string, error) {
	// For development, we'll do simple validation
	// In production, this should make HTTP request to User Service REST API

	log.Printf("🔍 Validating stream key via HTTP: %s", streamKey)

	// Simple validation - any non-empty key is valid for now
	if len(streamKey) > 5 {
		return true, 123, "test_user", nil
	}

	return false, 0, "", nil
}

// Additional helper method to get detailed stream info
func (h *RTMPHandler) GetStreamInfo(c *gin.Context) {
	streamKey := c.Param("stream_key")
	if streamKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stream key required"})
		return
	}

	// Get session info
	sessionData, err := h.streamService.GetStreamSession(streamKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stream session not found"})
		return
	}

	// Try to get stream details if stream ID is available
	streamID, ok := sessionData["stream_id"].(string)
	if ok {
		// For now, just return session data
		// When gRPC is ready, we can get full stream details
		c.JSON(http.StatusOK, gin.H{
			"stream_id": streamID,
			"session":   sessionData,
			"status":    "active",
		})
		return
	}

	// Fallback to session data only
	c.JSON(http.StatusOK, gin.H{
		"session": sessionData,
		"status":  "session_only",
	})
}

// TODO: Implement proper HTTP client to User Service REST API
func (h *RTMPHandler) validateStreamKeyWithUserService(streamKey, ipAddress string) (bool, int64, string, error) {
	// This will make HTTP POST request to User Service
	// POST http://user-service:8000/api/v1/auth/validate-stream-key
	// {
	//   "stream_key": "abc123",
	//   "ip_address": "1.2.3.4"
	// }

	// For now, return mock data
	return true, 123, "test_user", nil
}
