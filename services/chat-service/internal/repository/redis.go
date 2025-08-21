package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/yourcompany/chat-app/services/chat-service/internal/config"
	"github.com/yourcompany/chat-app/services/chat-service/internal/models"
)

type RedisRepository interface {
	AddUserToChatroom(ctx context.Context, userID, chatroomID string) error
	RemoveUserFromChatroom(ctx context.Context, userID, chatroomID string) error
	CacheMessage(ctx context.Context, message *models.Message) error
	GetCachedMessages(ctx context.Context, chatroomID string, limit int) ([]*models.Message, error)
	SetUserOnline(ctx context.Context, userID string) error
	SetUserOffline(ctx context.Context, userID string) error
	IsUserOnline(ctx context.Context, userID string) (bool, error)
}

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(cfg config.RedisConfig) (RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &redisRepository{
		client: client,
	}, nil
}

func (r *redisRepository) AddUserToChatroom(ctx context.Context, userID, chatroomID string) error {
	key := fmt.Sprintf("user:%s:chatrooms", userID)
	return r.client.SAdd(ctx, key, chatroomID).Err()
}

func (r *redisRepository) RemoveUserFromChatroom(ctx context.Context, userID, chatroomID string) error {
	key := fmt.Sprintf("user:%s:chatrooms", userID)
	return r.client.SRem(ctx, key, chatroomID).Err()
}

func (r *redisRepository) CacheMessage(ctx context.Context, message *models.Message) error {
	key := fmt.Sprintf("chatroom:%s:messages", message.ChatroomID)

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Use sorted set with timestamp as score
	score := float64(message.CreatedAt.Unix())
	err = r.client.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: messageJSON,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to cache message: %w", err)
	}

	// Keep only last 100 messages
	r.client.ZRemRangeByRank(ctx, key, 0, -101)

	return nil
}

func (r *redisRepository) GetCachedMessages(ctx context.Context, chatroomID string, limit int) ([]*models.Message, error) {
	key := fmt.Sprintf("chatroom:%s:messages", chatroomID)

	// Get messages in reverse chronological order
	result, err := r.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached messages: %w", err)
	}

	messages := make([]*models.Message, 0, len(result))
	for _, messageJSON := range result {
		var message models.Message
		err = json.Unmarshal([]byte(messageJSON), &message)
		if err != nil {
			continue // Skip invalid messages
		}
		messages = append(messages, &message)
	}

	return messages, nil
}

func (r *redisRepository) SetUserOnline(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s:online", userID)
	return r.client.Set(ctx, key, "true", 5*time.Minute).Err()
}

func (r *redisRepository) SetUserOffline(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s:online", userID)
	return r.client.Del(ctx, key).Err()
}

func (r *redisRepository) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("user:%s:online", userID)
	result, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	online, err := strconv.ParseBool(result)
	if err != nil {
		return false, err
	}

	return online, nil
}
