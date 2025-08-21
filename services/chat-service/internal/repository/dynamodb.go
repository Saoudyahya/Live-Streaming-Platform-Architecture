package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/models"
)

type DynamoDBRepository interface {
	CreateChatroom(ctx context.Context, chatroom *models.Chatroom) error
	GetChatroom(ctx context.Context, chatroomID string) (*models.Chatroom, error)
	AddMemberToChatroom(ctx context.Context, chatroomID, userID string) error
	RemoveMemberFromChatroom(ctx context.Context, chatroomID, userID string) error
	IsUserMemberOfChatroom(ctx context.Context, chatroomID, userID string) (bool, error)
	GetUserChatrooms(ctx context.Context, userID string) ([]*models.Chatroom, error)
	CreateMessage(ctx context.Context, message *models.Message) error
	GetMessages(ctx context.Context, chatroomID string, limit int, cursor string) ([]*models.Message, error)
}

type dynamoDBRepository struct {
	db            *dynamodb.DynamoDB
	chatroomTable string
	messageTable  string
}

func NewDynamoDBRepository(cfg config.DynamoDBConfig) (DynamoDBRepository, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region),
		Credentials: credentials.NewStaticCredentials(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &dynamoDBRepository{
		db:            dynamodb.New(sess),
		chatroomTable: cfg.ChatroomTable,
		messageTable:  cfg.MessageTable,
	}, nil
}

func (r *dynamoDBRepository) CreateChatroom(ctx context.Context, chatroom *models.Chatroom) error {
	item, err := dynamodbattribute.MarshalMap(chatroom)
	if err != nil {
		return fmt.Errorf("failed to marshal chatroom: %w", err)
	}

	_, err = r.db.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.chatroomTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put chatroom item: %w", err)
	}

	return nil
}

func (r *dynamoDBRepository) GetChatroom(ctx context.Context, chatroomID string) (*models.Chatroom, error) {
	result, err := r.db.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.chatroomTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(chatroomID),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get chatroom: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("chatroom not found")
	}

	var chatroom models.Chatroom
	err = dynamodbattribute.UnmarshalMap(result.Item, &chatroom)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chatroom: %w", err)
	}

	return &chatroom, nil
}

func (r *dynamoDBRepository) AddMemberToChatroom(ctx context.Context, chatroomID, userID string) error {
	updateExpr := expression.SET(expression.Name("member_ids"), expression.ListAppend(expression.Name("member_ids"), expression.Value([]string{userID})))
	expr, err := expression.NewBuilder().WithUpdate(updateExpr).Build()
	if err != nil {
		return fmt.Errorf("failed to build update expression: %w", err)
	}

	_, err = r.db.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.chatroomTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(chatroomID),
			},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		return fmt.Errorf("failed to add member to chatroom: %w", err)
	}

	return nil
}

func (r *dynamoDBRepository) RemoveMemberFromChatroom(ctx context.Context, chatroomID, userID string) error {
	// This is a simplified implementation. In practice, you'd need to find the index and remove it.
	// For production, consider using a separate table for chatroom memberships.
	chatroom, err := r.GetChatroom(ctx, chatroomID)
	if err != nil {
		return err
	}

	updatedMembers := make([]string, 0, len(chatroom.MemberIDs))
	for _, memberID := range chatroom.MemberIDs {
		if memberID != userID {
			updatedMembers = append(updatedMembers, memberID)
		}
	}

	updateExpr := expression.SET(expression.Name("member_ids"), expression.Value(updatedMembers))
	expr, err := expression.NewBuilder().WithUpdate(updateExpr).Build()
	if err != nil {
		return fmt.Errorf("failed to build update expression: %w", err)
	}

	_, err = r.db.UpdateItemWithContext(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.chatroomTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(chatroomID),
			},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		return fmt.Errorf("failed to remove member from chatroom: %w", err)
	}

	return nil
}

func (r *dynamoDBRepository) IsUserMemberOfChatroom(ctx context.Context, chatroomID, userID string) (bool, error) {
	chatroom, err := r.GetChatroom(ctx, chatroomID)
	if err != nil {
		return false, err
	}

	for _, memberID := range chatroom.MemberIDs {
		if memberID == userID {
			return true, nil
		}
	}

	return false, nil
}

func (r *dynamoDBRepository) GetUserChatrooms(ctx context.Context, userID string) ([]*models.Chatroom, error) {
	// This requires a GSI on member_ids or a separate table for efficient querying
	// Simplified implementation using scan (not recommended for production)
	filterExpr := expression.Contains(expression.Name("member_ids"), userID)
	expr, err := expression.NewBuilder().WithFilter(filterExpr).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build filter expression: %w", err)
	}

	result, err := r.db.ScanWithContext(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(r.chatroomTable),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan chatrooms: %w", err)
	}

	var chatrooms []*models.Chatroom
	for _, item := range result.Items {
		var chatroom models.Chatroom
		err = dynamodbattribute.UnmarshalMap(item, &chatroom)
		if err != nil {
			continue // Skip invalid items
		}
		chatrooms = append(chatrooms, &chatroom)
	}

	return chatrooms, nil
}

func (r *dynamoDBRepository) CreateMessage(ctx context.Context, message *models.Message) error {
	item, err := dynamodbattribute.MarshalMap(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = r.db.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.messageTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put message item: %w", err)
	}

	return nil
}

func (r *dynamoDBRepository) GetMessages(ctx context.Context, chatroomID string, limit int, cursor string) ([]*models.Message, error) {
	// This requires a GSI on chatroom_id sorted by created_at
	// Simplified implementation
	filterExpr := expression.Equal(expression.Name("chatroom_id"), expression.Value(chatroomID))
	expr, err := expression.NewBuilder().WithFilter(filterExpr).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build filter expression: %w", err)
	}

	input := &dynamodb.ScanInput{
		TableName:                 aws.String(r.messageTable),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int64(int64(limit)),
	}

	result, err := r.db.ScanWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	var messages []*models.Message
	for _, item := range result.Items {
		var message models.Message
		err = dynamodbattribute.UnmarshalMap(item, &message)
		if err != nil {
			continue // Skip invalid items
		}
		messages = append(messages, &message)
	}

	return messages, nil
}
