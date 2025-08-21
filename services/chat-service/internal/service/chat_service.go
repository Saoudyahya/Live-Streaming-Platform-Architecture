// services/chat-service/internal/service/chat_service.go
package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatpb "github.com/yourcompany/chat-app/gen/chat"
	commonpb "github.com/yourcompany/chat-app/gen/common"
	userpb "github.com/yourcompany/chat-app/gen/user"
	"github.com/yourcompany/chat-app/services/chat-service/internal/models"
	"github.com/yourcompany/chat-app/services/chat-service/internal/repository"
)

type ChatService struct {
	chatpb.UnimplementedChatServiceServer
	dynamoRepo repository.DynamoDBRepository
	redisRepo  repository.RedisRepository
	userClient userpb.UserServiceClient
}

func NewChatService(
	dynamoRepo repository.DynamoDBRepository,
	redisRepo repository.RedisRepository,
	userClient userpb.UserServiceClient,
) *ChatService {
	return &ChatService{
		dynamoRepo: dynamoRepo,
		redisRepo:  redisRepo,
		userClient: userClient,
	}
}

func (s *ChatService) CreateChatroom(ctx context.Context, req *chatpb.CreateChatroomRequest) (*chatpb.CreateChatroomResponse, error) {
	// Validate user exists
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.CreatorId,
	})
	if err != nil {
		log.Printf("Failed to validate user %s: %v", req.CreatorId, err)
		return &chatpb.CreateChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to validate user",
				Success: false,
			},
		}, nil
	}

	if !userResp.Status.Success {
		return &chatpb.CreateChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	// Create chatroom
	chatroom := &models.Chatroom{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		CreatorID:   req.CreatorId,
		IsPrivate:   req.IsPrivate,
		MemberIDs:   []string{req.CreatorId}, // Creator is automatically a member
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = s.dynamoRepo.CreateChatroom(ctx, chatroom)
	if err != nil {
		log.Printf("Failed to create chatroom: %v", err)
		return &chatpb.CreateChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to create chatroom",
				Success: false,
			},
		}, nil
	}

	// Add to user's chatrooms in Redis
	err = s.redisRepo.AddUserToChatroom(ctx, req.CreatorId, chatroom.ID)
	if err != nil {
		log.Printf("Failed to add user to chatroom in Redis: %v", err)
	}

	return &chatpb.CreateChatroomResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Chatroom created successfully",
			Success: true,
		},
		Chatroom: chatroomToProto(chatroom),
	}, nil
}

func (s *ChatService) JoinChatroom(ctx context.Context, req *chatpb.JoinChatroomRequest) (*chatpb.JoinChatroomResponse, error) {
	// Validate user exists
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.UserId,
	})
	if err != nil || !userResp.Status.Success {
		return &chatpb.JoinChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	// Get chatroom
	chatroom, err := s.dynamoRepo.GetChatroom(ctx, req.ChatroomId)
	if err != nil {
		return &chatpb.JoinChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "Chatroom not found",
				Success: false,
			},
		}, nil
	}

	// Check if user is already a member
	for _, memberID := range chatroom.MemberIDs {
		if memberID == req.UserId {
			return &chatpb.JoinChatroomResponse{
				Status: &commonpb.Status{
					Code:    int32(codes.AlreadyExists),
					Message: "User is already a member",
					Success: false,
				},
			}, nil
		}
	}

	// Add user to chatroom
	err = s.dynamoRepo.AddMemberToChatroom(ctx, req.ChatroomId, req.UserId)
	if err != nil {
		log.Printf("Failed to add member to chatroom: %v", err)
		return &chatpb.JoinChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to join chatroom",
				Success: false,
			},
		}, nil
	}

	// Update Redis
	err = s.redisRepo.AddUserToChatroom(ctx, req.UserId, req.ChatroomId)
	if err != nil {
		log.Printf("Failed to add user to chatroom in Redis: %v", err)
	}

	// Send system message
	systemMessage := &models.Message{
		ID:         uuid.New().String(),
		ChatroomID: req.ChatroomId,
		UserID:     "system",
		Username:   "System",
		Content:    fmt.Sprintf("%s joined the chatroom", userResp.User.Username),
		Type:       models.MessageTypeSystem,
		CreatedAt:  time.Now(),
		IsEdited:   false,
	}

	err = s.dynamoRepo.CreateMessage(ctx, systemMessage)
	if err != nil {
		log.Printf("Failed to create system message: %v", err)
	}

	return &chatpb.JoinChatroomResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Successfully joined chatroom",
			Success: true,
		},
	}, nil
}

func (s *ChatService) LeaveChatroom(ctx context.Context, req *chatpb.LeaveChatroomRequest) (*chatpb.LeaveChatroomResponse, error) {
	// Validate user exists
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.UserId,
	})
	if err != nil || !userResp.Status.Success {
		return &chatpb.LeaveChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	// Remove user from chatroom
	err = s.dynamoRepo.RemoveMemberFromChatroom(ctx, req.ChatroomId, req.UserId)
	if err != nil {
		log.Printf("Failed to remove member from chatroom: %v", err)
		return &chatpb.LeaveChatroomResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to leave chatroom",
				Success: false,
			},
		}, nil
	}

	// Update Redis
	err = s.redisRepo.RemoveUserFromChatroom(ctx, req.UserId, req.ChatroomId)
	if err != nil {
		log.Printf("Failed to remove user from chatroom in Redis: %v", err)
	}

	// Send system message
	systemMessage := &models.Message{
		ID:         uuid.New().String(),
		ChatroomID: req.ChatroomId,
		UserID:     "system",
		Username:   "System",
		Content:    fmt.Sprintf("%s left the chatroom", userResp.User.Username),
		Type:       models.MessageTypeSystem,
		CreatedAt:  time.Now(),
		IsEdited:   false,
	}

	err = s.dynamoRepo.CreateMessage(ctx, systemMessage)
	if err != nil {
		log.Printf("Failed to create system message: %v", err)
	}

	return &chatpb.LeaveChatroomResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Successfully left chatroom",
			Success: true,
		},
	}, nil
}

func (s *ChatService) SendMessage(ctx context.Context, req *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {
	// Validate user exists
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.UserId,
	})
	if err != nil || !userResp.Status.Success {
		return &chatpb.SendMessageResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	// Check if user is member of chatroom
	isMember, err := s.dynamoRepo.IsUserMemberOfChatroom(ctx, req.ChatroomId, req.UserId)
	if err != nil {
		log.Printf("Failed to check chatroom membership: %v", err)
		return &chatpb.SendMessageResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to validate membership",
				Success: false,
			},
		}, nil
	}

	if !isMember {
		return &chatpb.SendMessageResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.PermissionDenied),
				Message: "User is not a member of this chatroom",
				Success: false,
			},
		}, nil
	}

	// Create message
	message := &models.Message{
		ID:         uuid.New().String(),
		ChatroomID: req.ChatroomId,
		UserID:     req.UserId,
		Username:   userResp.User.Username,
		Content:    req.Content,
		Type:       messageTypeFromProto(req.Type),
		CreatedAt:  time.Now(),
		IsEdited:   false,
	}

	err = s.dynamoRepo.CreateMessage(ctx, message)
	if err != nil {
		log.Printf("Failed to create message: %v", err)
		return &chatpb.SendMessageResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to send message",
				Success: false,
			},
		}, nil
	}

	// Cache message in Redis
	err = s.redisRepo.CacheMessage(ctx, message)
	if err != nil {
		log.Printf("Failed to cache message in Redis: %v", err)
	}

	return &chatpb.SendMessageResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Message sent successfully",
			Success: true,
		},
		Message: messageToProto(message),
	}, nil
}

func (s *ChatService) GetMessages(ctx context.Context, req *chatpb.GetMessagesRequest) (*chatpb.GetMessagesResponse, error) {
	// Validate user exists and is member of chatroom
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.UserId,
	})
	if err != nil || !userResp.Status.Success {
		return &chatpb.GetMessagesResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	isMember, err := s.dynamoRepo.IsUserMemberOfChatroom(ctx, req.ChatroomId, req.UserId)
	if err != nil || !isMember {
		return &chatpb.GetMessagesResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.PermissionDenied),
				Message: "User is not a member of this chatroom",
				Success: false,
			},
		}, nil
	}

	// Get messages from cache first
	messages, err := s.redisRepo.GetCachedMessages(ctx, req.ChatroomId, int(req.Limit))
	if err != nil {
		log.Printf("Failed to get cached messages: %v", err)
		// Fallback to DynamoDB
		messages, err = s.dynamoRepo.GetMessages(ctx, req.ChatroomId, int(req.Limit), req.Cursor)
		if err != nil {
			log.Printf("Failed to get messages from DynamoDB: %v", err)
			return &chatpb.GetMessagesResponse{
				Status: &commonpb.Status{
					Code:    int32(codes.Internal),
					Message: "Failed to retrieve messages",
					Success: false,
				},
			}, nil
		}
	}

	protoMessages := make([]*chatpb.Message, len(messages))
	for i, msg := range messages {
		protoMessages[i] = messageToProto(msg)
	}

	return &chatpb.GetMessagesResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Messages retrieved successfully",
			Success: true,
		},
		Messages:   protoMessages,
		NextCursor: "", // Implement pagination cursor logic
	}, nil
}

func (s *ChatService) GetChatrooms(ctx context.Context, req *chatpb.GetChatroomsRequest) (*chatpb.GetChatroomsResponse, error) {
	// Validate user exists
	userResp, err := s.userClient.GetUser(ctx, &userpb.GetUserRequest{
		UserId: req.UserId,
	})
	if err != nil || !userResp.Status.Success {
		return &chatpb.GetChatroomsResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.NotFound),
				Message: "User not found",
				Success: false,
			},
		}, nil
	}

	// Get user's chatrooms
	chatrooms, err := s.dynamoRepo.GetUserChatrooms(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get user chatrooms: %v", err)
		return &chatpb.GetChatroomsResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to retrieve chatrooms",
				Success: false,
			},
		}, nil
	}

	protoChatrooms := make([]*chatpb.Chatroom, len(chatrooms))
	for i, chatroom := range chatrooms {
		protoChatrooms[i] = chatroomToProto(chatroom)
	}

	return &chatpb.GetChatroomsResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Chatrooms retrieved successfully",
			Success: true,
		},
		Chatrooms: protoChatrooms,
	}, nil
}

// Helper functions for proto conversion
func chatroomToProto(chatroom *models.Chatroom) *chatpb.Chatroom {
	return &chatpb.Chatroom{
		Id:          chatroom.ID,
		Name:        chatroom.Name,
		Description: chatroom.Description,
		CreatorId:   chatroom.CreatorID,
		IsPrivate:   chatroom.IsPrivate,
		MemberIds:   chatroom.MemberIDs,
		CreatedAt: &commonpb.Timestamp{
			Seconds: chatroom.CreatedAt.Unix(),
			Nanos:   int32(chatroom.CreatedAt.Nanosecond()),
		},
		UpdatedAt: &commonpb.Timestamp{
			Seconds: chatroom.UpdatedAt.Unix(),
			Nanos:   int32(chatroom.UpdatedAt.Nanosecond()),
		},
	}
}

func messageToProto(message *models.Message) *chatpb.Message {
	return &chatpb.Message{
		Id:         message.ID,
		ChatroomId: message.ChatroomID,
		UserId:     message.UserID,
		Username:   message.Username,
		Content:    message.Content,
		Type:       messageTypeToProto(message.Type),
		CreatedAt: &commonpb.Timestamp{
			Seconds: message.CreatedAt.Unix(),
			Nanos:   int32(message.CreatedAt.Nanosecond()),
		},
		IsEdited: message.IsEdited,
	}
}

func messageTypeFromProto(protoType chatpb.MessageType) models.MessageType {
	switch protoType {
	case chatpb.MessageType_TEXT:
		return models.MessageTypeText
	case chatpb.MessageType_IMAGE:
		return models.MessageTypeImage
	case chatpb.MessageType_FILE:
		return models.MessageTypeFile
	case chatpb.MessageType_SYSTEM:
		return models.MessageTypeSystem
	default:
		return models.MessageTypeText
	}
}

func messageTypeToProto(msgType models.MessageType) chatpb.MessageType {
	switch msgType {
	case models.MessageTypeText:
		return chatpb.MessageType_TEXT
	case models.MessageTypeImage:
		return chatpb.MessageType_IMAGE
	case models.MessageTypeFile:
		return chatpb.MessageType_FILE
	case models.MessageTypeSystem:
		return chatpb.MessageType_SYSTEM
	default:
		return chatpb.MessageType_TEXT
	}
}
