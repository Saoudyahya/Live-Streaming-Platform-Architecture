// services/stream-management-service/internal/repository/redis.go
package repository

import (
	"fmt"
	"time"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(cfg *config.Config) *RedisRepository {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	return &RedisRepository{
		client: rdb,
	}
}

func (r *RedisRepository) SetStreamData(streamID, data string, expiration time.Duration) error {
	ctx := context.Background()
	key := fmt.Sprintf("stream:%s", streamID)

	err := r.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set stream data: %w", err)
	}

	return nil
}

func (r *RedisRepository) GetStreamData(streamID string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("stream:%s", streamID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get stream data: %w", err)
	}

	return data, nil
}

func (r *RedisRepository) SetStreamSession(streamKey, sessionData string, expiration time.Duration) error {
	ctx := context.Background()
	key := fmt.Sprintf("session:%s", streamKey)

	err := r.client.Set(ctx, key, sessionData, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set stream session: %w", err)
	}

	return nil
}

func (r *RedisRepository) GetStreamSession(streamKey string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("session:%s", streamKey)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get stream session: %w", err)
	}

	return data, nil
}

func (r *RedisRepository) DeleteStreamSession(streamKey string) error {
	ctx := context.Background()
	key := fmt.Sprintf("session:%s", streamKey)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete stream session: %w", err)
	}

	return nil
}
