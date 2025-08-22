// services/stream-management-service/internal/service/rtmp_handler.go
package service

import (
	_ "encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/grpc"
	"github.com/gin-gonic/gin"
)

type RTMPHandler struct {
	config        *config.Config
	streamService *StreamService
	userClient    *grpc.UserServiceClient
}

type RTMPAuthRequest struct {
	Name   string `json:"name"`   // Stream key from SRS
	IP     string `json:"addr"`   // Client IP
	App    string `json:"app"`    // Application name
	Swfurl string `json:"swfurl"` // SWF URL
	Tcurl  string `json:"tcurl"`  // TC URL
}

type RTMPStreamRequest struct {
	Name     string `json:"name"`     // Stream key
	IP       string `json:"addr"`     // Client IP
	App      string `json:"app"`      // Application name
	Duration string `json:"duration"` // Duration in seconds (for ended streams)
	File     string `json:"file"`     // Recording file path
}

func NewRTMPHandler(cfg *config.Config, streamService *StreamService, userClient *grpc.UserServiceClient) *RTMPHandler {
	return &RTMPHandler{
		config:        cfg,
		streamService: streamService,
		userClient:    userClient,
	}
}

func (h *RTMPHandler) AuthenticateStream(c *gin.Context) {
	var req RTMPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Error parsing auth request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("🔑 RTMP Auth Request - Name: %s, IP: %s, App: %s", req.Name, req.IP, req.App)

	// Extract stream key from name (SRS might send app/stream_key format)
	streamKey := req.Name
	if strings.Contains(req.Name, "/") {
		parts := strings.Split(req.Name, "/")
		streamKey = parts[len(parts)-1] // Get the last part
	}

	log.Printf("🔍 Extracted stream key: %s", streamKey)

	// Validate stream key with User Service
	validateReq := map[string]interface{}{
		"stream_key": streamKey,
		"ip_address": req.IP,
	}

	valid, userID, username, err := h.userClient.ValidateStreamKey(validateReq)
	if err != nil {
		log.Printf("❌ Error validating stream key with User Service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !valid {
		log.Printf("❌ Invalid stream key: %s", streamKey)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Unauthorized",
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
		"started_at": time.Now().Unix(),
	}

	if err := h.streamService.StoreStreamSession(streamKey, sessionData); err != nil {
		log.Printf("⚠️ Warning: Could not store stream session: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"authorized": true,
		"user_id":    userID,
		"username":   username,
	})
}

func (h *RTMPHandler) StreamStarted(c *gin.Context) {
	var req RTMPStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Error parsing stream started request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("🔴 Stream STARTED - Name: %s, IP: %s", req.Name, req.IP)

	// Extract stream key
	streamKey := req.Name
	if strings.Contains(req.Name, "/") {
		parts := strings.Split(req.Name, "/")
		streamKey = parts[len(parts)-1]
	}

	// Get session info from Redis
	sessionData, err := h.streamService.GetStreamSession(streamKey)
	if err != nil {
		log.Printf("❌ Could not get stream session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session not found"})
		return
	}

	userID, ok := sessionData["user_id"].(float64) // JSON numbers are float64
	if !ok {
		log.Printf("❌ Invalid user_id in session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session"})
		return
	}

	// Create stream record
	stream := &models.Stream{
		UserID:      int64(userID),
		StreamKey:   streamKey,
		Status:      models.StreamStatusLive,
		StartedAt:   &time.Time{},
		ViewerCount: 0,
		Metadata: map[string]string{
			"client_ip": req.IP,
			"app":       req.App,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	*stream.StartedAt = time.Now()

	streamID, err := h.streamService.CreateStream(stream)
	if err != nil {
		log.Printf("❌ Error creating stream record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create stream"})
		return
	}

	log.Printf("✅ Stream created with ID: %s", streamID)

	// Publish stream started event to Kinesis
	event := map[string]interface{}{
		"event_type": "stream_started",
		"stream_id":  streamID,
		"user_id":    userID,
		"timestamp":  time.Now().Unix(),
		"metadata": map[string]interface{}{
			"stream_key": streamKey,
			"client_ip":  req.IP,
		},
	}

	if err := h.streamService.PublishEvent(event); err != nil {
		log.Printf("⚠️ Warning: Could not publish stream started event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Stream started",
		"stream_id": streamID,
	})
}

func (h *RTMPHandler) StreamEnded(c *gin.Context) {
	var req RTMPStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Error parsing stream ended request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("🔴 Stream ENDED - Name: %s, Duration: %s", req.Name, req.Duration)

	// Extract stream key
	streamKey := req.Name
	if strings.Contains(req.Name, "/") {
		parts := strings.Split(req.Name, "/")
		streamKey = parts[len(parts)-1]
	}

	// Update stream status to ended
	err := h.streamService.EndStream(streamKey, req.Duration)
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
		"message": "Stream ended",
	})
}

func (h *RTMPHandler) RecordingCompleted(c *gin.Context) {
	var req RTMPStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Error parsing recording completed request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("📹 Recording COMPLETED - Name: %s, File: %s", req.Name, req.File)

	// Extract stream key
	streamKey := req.Name
	if strings.Contains(req.Name, "/") {
		parts := strings.Split(req.Name, "/")
		streamKey = parts[len(parts)-1]
	}

	// Update stream with recording info
	err := h.streamService.UpdateStreamRecording(streamKey, req.File)
	if err != nil {
		log.Printf("❌ Error updating stream recording: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update recording"})
		return
	}

	log.Printf("✅ Recording updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message": "Recording completed",
	})
}
