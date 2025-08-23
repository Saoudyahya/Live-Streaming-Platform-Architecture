// services/stream-management-service/internal/server/grpc.go
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	_ "strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	_ "google.golang.org/grpc/status"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/service"
	grpcClient "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/grpc"

	// Import the generated protobuf files (you'll need to generate these)
	commonpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/gen/common"
	timestamppb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/gen/common"
	streampb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/gen/stream"
)

type StreamGRPCServer struct {
	streampb.UnimplementedStreamServiceServer
	config        *config.Config
	streamService *service.StreamService
	userClient    *grpcClient.UserServiceClient
}

func NewStreamGRPCServer(cfg *config.Config, streamService *service.StreamService, userClient *grpcClient.UserServiceClient) *StreamGRPCServer {
	return &StreamGRPCServer{
		config:        cfg,
		streamService: streamService,
		userClient:    userClient,
	}
}

func (s *StreamGRPCServer) ValidateStreamKey(ctx context.Context, req *streampb.ValidateStreamKeyRequest) (*streampb.ValidateStreamKeyResponse, error) {
	log.Printf("🔑 gRPC ValidateStreamKey: %s from IP: %s", req.StreamKey, req.IpAddress)

	// Validate with User Service
	userReq := map[string]interface{}{
		"stream_key": req.StreamKey,
		"ip_address": req.IpAddress,
	}

	valid, userID, username, err := s.userClient.ValidateStreamKey(userReq)
	if err != nil {
		log.Printf("❌ Error validating stream key with User Service: %v", err)
		return &streampb.ValidateStreamKeyResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Internal server error",
				Success: false,
			},
			IsValid: false,
		}, nil
	}

	if !valid {
		log.Printf("❌ Invalid stream key: %s", req.StreamKey)
		return &streampb.ValidateStreamKeyResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.PermissionDenied),
				Message: "Invalid stream key",
				Success: false,
			},
			IsValid: false,
		}, nil
	}

	log.Printf("✅ Stream key validated - User: %s (ID: %d)", username, userID)

	// Return successful validation with permissions
	return &streampb.ValidateStreamKeyResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Stream key validated successfully",
			Success: true,
		},
		IsValid:  true,
		UserId:   userID,
		Username: username,
		Permissions: &streampb.StreamPermissions{
			CanStream:          true,
			CanRecord:          true,
			MaxBitrate:         8000, // 8 Mbps max
			MaxDurationMinutes: 240,  // 4 hours max
		},
	}, nil
}

func (s *StreamGRPCServer) CreateStream(ctx context.Context, req *streampb.CreateStreamRequest) (*streampb.CreateStreamResponse, error) {
	log.Printf("🎬 gRPC CreateStream for User: %d", req.UserId)

	// Convert gRPC request to internal model
	stream := &models.Stream{
		UserID:    req.UserId,
		StreamKey: req.StreamKey,
		Title:     req.Title,
		Status:    models.StreamStatusLive,
		Metadata: map[string]string{
			"client_ip": req.Metadata.ClientIp,
			"app_name":  req.Metadata.AppName,
			"bitrate":   strconv.Itoa(int(req.Metadata.Bitrate)),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	now := time.Now()
	stream.StartedAt = &now

	// Create stream
	streamID, err := s.streamService.CreateStream(stream)
	if err != nil {
		log.Printf("❌ Error creating stream: %v", err)
		return &streampb.CreateStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: fmt.Sprintf("Failed to create stream: %v", err),
				Success: false,
			},
		}, nil
	}

	// Convert back to gRPC response
	stream.ID = streamID
	grpcStream := s.modelToGRPCStream(stream)

	return &streampb.CreateStreamResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Stream created successfully",
			Success: true,
		},
		StreamId: streamID,
		Stream:   grpcStream,
	}, nil
}

func (s *StreamGRPCServer) UpdateStream(ctx context.Context, req *streampb.UpdateStreamRequest) (*streampb.UpdateStreamResponse, error) {
	log.Printf("📝 gRPC UpdateStream: %s", req.StreamId)

	// Get existing stream
	stream, err := s.streamService.GetStreamByIDInternal(req.StreamId)
	if err != nil {
		return &streampb.UpdateStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "Stream not found",
				Success: false,
			},
		}, nil
	}

	// Update stream fields
	if req.Status != streampb.StreamStatus_STREAM_PENDING {
		stream.Status = s.grpcToModelStatus(req.Status)
	}

	if req.ViewerCount > 0 {
		stream.ViewerCount = int(req.ViewerCount)
	}

	if req.DurationSeconds > 0 {
		stream.Duration = req.DurationSeconds
	}

	if req.Metadata != nil {
		if stream.Metadata == nil {
			stream.Metadata = make(map[string]string)
		}
		if req.Metadata.ClientIp != "" {
			stream.Metadata["client_ip"] = req.Metadata.ClientIp
		}
		if req.Metadata.Bitrate > 0 {
			stream.Metadata["bitrate"] = strconv.Itoa(int(req.Metadata.Bitrate))
		}
	}

	stream.UpdatedAt = time.Now()

	// Update in database
	err = s.streamService.UpdateStreamInternal(stream)
	if err != nil {
		return &streampb.UpdateStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: fmt.Sprintf("Failed to update stream: %v", err),
				Success: false,
			},
		}, nil
	}

	return &streampb.UpdateStreamResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Stream updated successfully",
			Success: true,
		},
		Stream: s.modelToGRPCStream(stream),
	}, nil
}

func (s *StreamGRPCServer) GetStream(ctx context.Context, req *streampb.GetStreamRequest) (*streampb.GetStreamResponse, error) {
	stream, err := s.streamService.GetStreamByIDInternal(req.StreamId)
	if err != nil {
		return &streampb.GetStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "Stream not found",
				Success: false,
			},
		}, nil
	}

	return &streampb.GetStreamResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Stream retrieved successfully",
			Success: true,
		},
		Stream: s.modelToGRPCStream(stream),
	}, nil
}

func (s *StreamGRPCServer) GetActiveStreams(ctx context.Context, req *streampb.GetActiveStreamsRequest) (*streampb.GetActiveStreamsResponse, error) {
	streams, err := s.streamService.GetActiveStreamsInternal()
	if err != nil {
		return &streampb.GetActiveStreamsResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: fmt.Sprintf("Failed to get active streams: %v", err),
				Success: false,
			},
		}, nil
	}

	var grpcStreams []*streampb.Stream
	for _, stream := range streams {
		grpcStreams = append(grpcStreams, s.modelToGRPCStream(stream))
	}

	return &streampb.GetActiveStreamsResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Active streams retrieved successfully",
			Success: true,
		},
		Streams:    grpcStreams,
		TotalCount: int32(len(grpcStreams)),
	}, nil
}

func (s *StreamGRPCServer) EndStream(ctx context.Context, req *streampb.EndStreamRequest) (*streampb.EndStreamResponse, error) {
	log.Printf("🔴 gRPC EndStream: %s", req.StreamId)

	stream, err := s.streamService.GetStreamByIDInternal(req.StreamId)
	if err != nil {
		return &streampb.EndStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "Stream not found",
				Success: false,
			},
		}, nil
	}

	// End the stream
	now := time.Now()
	stream.Status = models.StreamStatusEnded
	stream.EndedAt = &now
	stream.Duration = req.DurationSeconds
	stream.UpdatedAt = now

	if req.RecordingPath != "" {
		stream.RecordingURL = req.RecordingPath
	}

	err = s.streamService.UpdateStreamInternal(stream)
	if err != nil {
		return &streampb.EndStreamResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: fmt.Sprintf("Failed to end stream: %v", err),
				Success: false,
			},
		}, nil
	}

	// Publish stream ended event
	event := map[string]interface{}{
		"event_type":   "stream_ended",
		"stream_id":    stream.ID,
		"user_id":      stream.UserID,
		"duration":     req.DurationSeconds,
		"timestamp":    time.Now().Unix(),
		"viewer_count": stream.ViewerCount,
	}
	s.streamService.PublishEvent(event)

	return &streampb.EndStreamResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Stream ended successfully",
			Success: true,
		},
	}, nil
}

func (s *StreamGRPCServer) RecordingCompleted(ctx context.Context, req *streampb.RecordingCompletedRequest) (*streampb.RecordingCompletedResponse, error) {
	log.Printf("📹 gRPC RecordingCompleted: %s", req.StreamId)

	stream, err := s.streamService.GetStreamByIDInternal(req.StreamId)
	if err != nil {
		return &streampb.RecordingCompletedResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "Stream not found",
				Success: false,
			},
		}, nil
	}

	// Update recording info
	stream.RecordingURL = req.RecordingPath
	stream.UpdatedAt = time.Now()

	// Add recording metadata
	if stream.Metadata == nil {
		stream.Metadata = make(map[string]string)
	}
	stream.Metadata["recording_size"] = strconv.FormatInt(req.FileSizeBytes, 10)
	stream.Metadata["recording_duration"] = strconv.FormatInt(req.DurationSeconds, 10)

	err = s.streamService.UpdateStreamInternal(stream)
	if err != nil {
		return &streampb.RecordingCompletedResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: fmt.Sprintf("Failed to update recording info: %v", err),
				Success: false,
			},
		}, nil
	}

	// TODO: Upload to S3 and return public URL
	recordingURL := req.RecordingPath

	return &streampb.RecordingCompletedResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Recording info updated successfully",
			Success: true,
		},
		RecordingUrl: recordingURL,
	}, nil
}

// Helper functions
func (s *StreamGRPCServer) modelToGRPCStream(stream *models.Stream) *streampb.Stream {
	grpcStream := &streampb.Stream{
		Id:           stream.ID,
		UserId:       stream.UserID,
		StreamKey:    stream.StreamKey,
		Title:        stream.Title,
		Status:       s.modelToGRPCStatus(stream.Status),
		ViewerCount:  int64(stream.ViewerCount),
		RecordingUrl: stream.RecordingURL,
		CreatedAt: &timestamppb.Timestamp{
			Seconds: stream.CreatedAt.Unix(),
			Nanos:   int32(stream.CreatedAt.Nanosecond()),
		},
		UpdatedAt: &timestamppb.Timestamp{
			Seconds: stream.UpdatedAt.Unix(),
			Nanos:   int32(stream.UpdatedAt.Nanosecond()),
		},
	}

	if stream.StartedAt != nil {
		grpcStream.StartedAt = &timestamppb.Timestamp{
			Seconds: stream.StartedAt.Unix(),
			Nanos:   int32(stream.StartedAt.Nanosecond()),
		}
	}

	if stream.EndedAt != nil {
		grpcStream.EndedAt = &timestamppb.Timestamp{
			Seconds: stream.EndedAt.Unix(),
			Nanos:   int32(stream.EndedAt.Nanosecond()),
		}
	}

	if stream.Metadata != nil {
		metadata := &streampb.StreamMetadata{
			ClientIp:   stream.Metadata["client_ip"],
			AppName:    stream.Metadata["app_name"],
			CustomData: stream.Metadata,
		}

		if bitrate, err := strconv.Atoi(stream.Metadata["bitrate"]); err == nil {
			metadata.Bitrate = int32(bitrate)
		}

		grpcStream.Metadata = metadata
	}

	grpcStream.DurationSeconds = stream.Duration

	return grpcStream
}

func (s *StreamGRPCServer) modelToGRPCStatus(status models.StreamStatus) streampb.StreamStatus {
	switch status {
	case models.StreamStatusPending:
		return streampb.StreamStatus_STREAM_PENDING
	case models.StreamStatusLive:
		return streampb.StreamStatus_STREAM_LIVE
	case models.StreamStatusEnded:
		return streampb.StreamStatus_STREAM_ENDED
	case models.StreamStatusError:
		return streampb.StreamStatus_STREAM_ERROR
	default:
		return streampb.StreamStatus_STREAM_PENDING
	}
}

func (s *StreamGRPCServer) grpcToModelStatus(status streampb.StreamStatus) models.StreamStatus {
	switch status {
	case streampb.StreamStatus_STREAM_PENDING:
		return models.StreamStatusPending
	case streampb.StreamStatus_STREAM_LIVE:
		return models.StreamStatusLive
	case streampb.StreamStatus_STREAM_ENDED:
		return models.StreamStatusEnded
	case streampb.StreamStatus_STREAM_ERROR:
		return models.StreamStatusError
	default:
		return models.StreamStatusPending
	}
}

// StartGRPCServer starts the gRPC server
func StartGRPCServer(cfg *config.Config, streamService *service.StreamService, userClient *grpcClient.UserServiceClient) (*grpc.Server, error) {
	// Create gRPC server
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(4*1024*1024), // 4MB max message size
		grpc.MaxSendMsgSize(4*1024*1024),
	)

	// Register stream service
	streamServer := NewStreamGRPCServer(cfg, streamService, userClient)
	streampb.RegisterStreamServiceServer(server, streamServer)

	// Enable reflection for grpcurl testing
	reflection.Register(server)

	// Find available port
	port := 9090
	var lis net.Listener
	var err error

	for i := 0; i < 10; i++ {
		lis, err = net.Listen("tcp", fmt.Sprintf(":%d", port+i))
		if err == nil {
			port = port + i
			break
		}
		if i == 9 {
			return nil, fmt.Errorf("could not find available port for gRPC server: %v", err)
		}
	}

	log.Printf("🚀 Starting gRPC server on port %d", port)

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Printf("❌ gRPC server failed: %v", err)
		}
	}()

	log.Printf("✅ gRPC server started successfully on port %d", port)
	return server, nil
}
