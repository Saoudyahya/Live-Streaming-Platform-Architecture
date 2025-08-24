// services/stream-management-service/internal/service/rtmp_handler.go
// UPDATED VERSION - Now passes app_name to gRPC validation

package service

import (
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

type RTMPHandler struct {
	config        *config.Config
	streamService *StreamService
	userClient    *grpcClient.UserServiceClient
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

	// Extract stream key from name
	streamKey := h.extractStreamKey(req.Name)
	log.Printf("🔍 Extracted stream key: %s", streamKey)

	// Validate stream key with app_name parameter
	valid, userID, username, err := h.validateStreamKey(streamKey, req.IP, req.App)
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

	// Return success response - FIXED: Return proper auth response
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

func (h *RTMPHandler) validateStreamKey(streamKey, ipAddress, appName string) (bool, int64, string, error) {
	log.Printf("🔑 Validating stream key: %s from IP: %s, app: %s", streamKey, ipAddress, appName)

	// Try gRPC validation first if client is available
	if h.userClient != nil {
		log.Printf("🔌 Attempting gRPC validation for stream key: %s", streamKey)

		// Create the request with all parameters including app_name
		request := map[string]interface{}{
			"stream_key": streamKey,
			"ip_address": ipAddress,
			"app_name":   appName,
		}

		// Call the gRPC validation
		valid, userID, username, err := h.userClient.ValidateStreamKey(request)
		if err == nil {
			log.Printf("✅ gRPC validation successful for stream key: %s", streamKey)
			return valid, userID, username, nil
		}

		log.Printf("⚠️ gRPC validation failed, falling back to HTTP: %v", err)
	} else {
		log.Printf("⚠️ No gRPC client available, using HTTP fallback")
	}

	// Fallback to HTTP validation
	return h.validateStreamKeyHTTP(streamKey, ipAddress)
}

// HTTP fallback method to validate stream key with User Service REST API
func (h *RTMPHandler) validateStreamKeyHTTP(streamKey, ipAddress string) (bool, int64, string, error) {
	log.Printf("🌐 HTTP validation for stream key: %s", streamKey)

	// This will be handled by the gRPC client's HTTP fallback
	// We create a request map and let the client handle it
	request := map[string]interface{}{
		"stream_key": streamKey,
		"ip_address": ipAddress,
	}

	// Use the gRPC client's HTTP fallback if available
	if h.userClient != nil {
		return h.userClient.ValidateStreamKey(request)
	}

	// Final fallback for development

	log.Printf("❌ Development validation failed for stream key: %s", streamKey)
	return false, 0, "", nil
}

func (h *RTMPHandler) StreamStarted(c *gin.Context) {
	var req RTMPStreamRequest

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

	// Create stream record
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

	// End stream
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

	// Publish stream ended event
	event := map[string]interface{}{
		"event_type": "stream_ended",
		"stream_id":  streamID,
		"timestamp":  time.Now().Unix(),
		"duration":   durationSec,
		"metadata": map[string]interface{}{
			"stream_key": streamKey,
			"end_reason": "normal",
		},
	}

	if err := h.streamService.PublishEvent(event); err != nil {
		log.Printf("⚠️ Warning: Could not publish stream ended event: %v", err)
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

	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("❌ Error parsing recording completed request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	}

	log.Printf("📹 Recording COMPLETED - Name: %s, File: %s", req.Name, req.File)

	streamKey := h.extractStreamKey(req.Name)

	// Update stream with recording info
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

func (h *RTMPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "stream-management",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"rtmp":      "ready",
	})
}

func (h *RTMPHandler) extractStreamKey(name string) string {
	streamKey := strings.TrimSpace(name)
	streamKey = strings.TrimPrefix(streamKey, "/")

	if strings.Contains(streamKey, "/") {
		parts := strings.Split(streamKey, "/")
		streamKey = parts[len(parts)-1]
	}

	return streamKey
}
