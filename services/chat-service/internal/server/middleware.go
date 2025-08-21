package server

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

// LoggingInterceptor logs gRPC requests and responses
func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	log.Printf("gRPC request - Method: %s, Request: %+v", info.FullMethod, req)

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	if err != nil {
		log.Printf("gRPC response - Method: %s, Duration: %v, Error: %v", info.FullMethod, duration, err)
	} else {
		log.Printf("gRPC response - Method: %s, Duration: %v, Success", info.FullMethod, duration)
	}

	return resp, err
}

// AuthInterceptor validates user authentication (simplified)
func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Skip auth for health checks
	if info.FullMethod == "/grpc.health.v1.Health/Check" {
		return handler(ctx, req)
	}

	// Extract auth token from metadata
	// This is a simplified implementation - in production, you'd validate JWT tokens
	// md, ok := metadata.FromIncomingContext(ctx)
	// if !ok {
	//     return nil, status.Errorf(codes.Unauthenticated, "no metadata provided")
	// }

	// For now, just proceed without auth validation
	return handler(ctx, req)
}

// RateLimitInterceptor implements basic rate limiting
func RateLimitInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Implement rate limiting logic here
	// For simplicity, we'll skip this in the example
	return handler(ctx, req)
}
