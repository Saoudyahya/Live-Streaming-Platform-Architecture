// ====================================
// 2. Updated gRPC Client for Stream Key Validation
// services/stream-management-service/pkg/grpc/clients.go
// ====================================

package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	userpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/gen/user"
)

type UserServiceClient struct {
	conn    *grpc.ClientConn
	client  userpb.UserServiceClient
	httpURL string // Fallback HTTP URL
}

func NewUserServiceClient(address string) (*UserServiceClient, error) {
	log.Printf("🔌 Connecting to User Service at: %s", address)

	// Connection with timeout and keepalive
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	var client userpb.UserServiceClient
	var httpURL string

	if err != nil {
		log.Printf("⚠️ gRPC connection failed: %v", err)
		log.Printf("🌐 Will use HTTP fallback to User Service")
		httpURL = "http://localhost:8000" // User Service REST API
		client = nil
	} else {
		client = userpb.NewUserServiceClient(conn)

		// Test connection
		testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer testCancel()

		testReq := &userpb.GetUserRequest{UserId: "test"}
		_, err = client.GetUser(testCtx, testReq)
		if err != nil {
			log.Printf("⚠️ User Service gRPC test failed: %v", err)
			log.Printf("🌐 Will use HTTP fallback for validation")
			httpURL = "http://localhost:8000"
		} else {
			log.Printf("✅ User Service gRPC connection test successful")
		}
	}

	return &UserServiceClient{
		conn:    conn,
		client:  client,
		httpURL: httpURL,
	}, nil
}

// ValidateStreamKey tries gRPC first, then HTTP fallback
func (c *UserServiceClient) ValidateStreamKey(request map[string]interface{}) (bool, int64, string, error) {
	streamKey, ok := request["stream_key"].(string)
	if !ok {
		return false, 0, "", fmt.Errorf("invalid stream_key in request")
	}

	ipAddress, _ := request["ip_address"].(string)

	log.Printf("🔍 Validating stream key: %s from IP: %s", streamKey, ipAddress)

	// Try gRPC first if client is available
	if c.client != nil {
		valid, userID, username, err := c.validateStreamKeyGRPC(streamKey, ipAddress)
		if err == nil {
			log.Printf("✅ gRPC validation successful for stream key: %s", streamKey)
			return valid, userID, username, nil
		}
		log.Printf("⚠️ gRPC validation failed, trying HTTP fallback: %v", err)
	}

	// Fallback to HTTP
	return c.validateStreamKeyHTTP(streamKey, ipAddress)
}

// validateStreamKeyGRPC validates using gRPC to User Service
func (c *UserServiceClient) validateStreamKeyGRPC(streamKey, ipAddress string) (bool, int64, string, error) {
	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()

	// For now, we don't have a direct stream key validation gRPC method
	// We can simulate by trying to find a user, or implement the method in User Service
	// This is a placeholder that should be replaced with actual gRPC stream key validation

	log.Printf("🔌 Attempting gRPC stream key validation: %s", streamKey)

	// TODO: Implement proper gRPC stream key validation method in User Service
	// For now, return error to fall back to HTTP
	return false, 0, "", fmt.Errorf("gRPC stream key validation not yet implemented")
}

// validateStreamKeyHTTP validates using HTTP REST API to User Service
func (c *UserServiceClient) validateStreamKeyHTTP(streamKey, ipAddress string) (bool, int64, string, error) {
	log.Printf("🌐 HTTP validation for stream key: %s", streamKey)

	if c.httpURL == "" {
		return false, 0, "", fmt.Errorf("no HTTP URL configured")
	}

	// Create request payload
	payload := map[string]string{
		"stream_key": streamKey,
		"ip_address": ipAddress,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request to User Service
	url := c.httpURL + "/api/v1/stream/validate-stream-key"
	log.Printf("📡 Making HTTP request to: %s", url)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP request failed: %v", err)
		return false, 0, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, 0, "", fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("📨 HTTP response status: %d, body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ HTTP validation failed with status: %d", resp.StatusCode)
		return false, 0, "", fmt.Errorf("HTTP validation failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Valid    bool   `json:"valid"`
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Message  string `json:"message"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("❌ Failed to parse HTTP response: %v", err)
		return false, 0, "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Valid {
		log.Printf("✅ HTTP validation successful - User: %s (ID: %d)", response.Username, response.UserID)
		return true, response.UserID, response.Username, nil
	} else {
		log.Printf("❌ HTTP validation failed: %s", response.Message)
		return false, 0, "", nil // Not an error, just invalid
	}
}

func (c *UserServiceClient) GetUser(userID string) (*userpb.User, error) {
	if c.client == nil {
		return nil, fmt.Errorf("gRPC client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &userpb.GetUserRequest{
		UserId: userID,
	}

	resp, err := c.client.GetUser(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if resp.GetStatus() != nil && !resp.GetStatus().GetSuccess() {
		return nil, fmt.Errorf("user service error: %s", resp.GetStatus().GetMessage())
	}

	return resp.User, nil
}

func (c *UserServiceClient) ValidateUser(userID, token string) (bool, *userpb.User, error) {
	if c.client == nil {
		return false, nil, fmt.Errorf("gRPC client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &userpb.ValidateUserRequest{
		UserId: userID,
		Token:  token,
	}

	resp, err := c.client.ValidateUser(ctx, req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to validate user: %w", err)
	}

	return resp.IsValid, resp.User, nil
}

func (c *UserServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Health check method
func (c *UserServiceClient) HealthCheck() error {
	if c.client == nil {
		// Try HTTP health check
		if c.httpURL != "" {
			url := c.httpURL + "/api/v1/health/"
			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
			return fmt.Errorf("HTTP health check failed with status: %d", resp.StatusCode)
		}
		return fmt.Errorf("no client available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simple health check by trying to get a test user
	req := &userpb.GetUserRequest{UserId: "healthcheck"}
	_, err := c.client.GetUser(ctx, req)

	// We don't care about the response, just that the connection works
	return err
}
