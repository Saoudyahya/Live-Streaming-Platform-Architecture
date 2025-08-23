// services/stream-management-service/pkg/grpc/clients.go
package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	userpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/gen/user"
)

type UserServiceClient struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
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
	if err != nil {
		return nil, fmt.Errorf("failed to connect to User Service: %w", err)
	}

	client := userpb.NewUserServiceClient(conn)

	// Test connection
	testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer testCancel()

	testReq := &userpb.GetUserRequest{UserId: "test"}
	_, err = client.GetUser(testCtx, testReq)
	if err != nil {
		log.Printf("⚠️ User Service connection test failed (this is OK if service isn't running): %v", err)
		// Don't fail here - the service might not be running yet
	} else {
		log.Printf("✅ User Service connection test successful")
	}

	return &UserServiceClient{
		conn:   conn,
		client: client,
	}, nil
}

func (c *UserServiceClient) ValidateStreamKey(request map[string]interface{}) (bool, int64, string, error) {
	if c.client == nil {
		return false, 0, "", fmt.Errorf("client not initialized")
	}

	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Extract stream key and IP from request
	streamKey, ok := request["stream_key"].(string)
	if !ok {
		return false, 0, "", fmt.Errorf("invalid stream_key in request")
	}

	ipAddress, _ := request["ip_address"].(string)

	// For now, we'll simulate validation since User Service might not be fully implemented
	// In production, this would make a real gRPC call to validate the stream key

	log.Printf("🔍 Validating stream key: %s from IP: %s", streamKey, ipAddress)

	// Simple validation - stream key must be at least 8 characters
	if len(streamKey) >= 8 {
		return true, 123, "test_user", nil
	}

	return false, 0, "", fmt.Errorf("invalid stream key")
}

func (c *UserServiceClient) GetUser(userID string) (*userpb.User, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not initialized")
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
		return false, nil, fmt.Errorf("client not initialized")
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
		return fmt.Errorf("client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simple health check by trying to get a test user
	req := &userpb.GetUserRequest{UserId: "healthcheck"}
	_, err := c.client.GetUser(ctx, req)

	// We don't care about the response, just that the connection works
	// The service might return "not found" but that's OK for health check
	return err
}
