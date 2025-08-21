// services/chat-service/internal/config/config.go
package config

import (
	"os"
)

type Config struct {
	Server      ServerConfig
	DynamoDB    DynamoDBConfig
	Redis       RedisConfig
	UserService UserServiceConfig
}

type ServerConfig struct {
	GRPCPort string
	HTTPPort string
}

type DynamoDBConfig struct {
	Region          string
	ChatroomTable   string
	MessageTable    string
	AccessKeyID     string
	SecretAccessKey string
}

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type UserServiceConfig struct {
	Address string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCPort: getEnv("GRPC_PORT", ":8080"),
			HTTPPort: getEnv("HTTP_PORT", ":8081"),
		},
		DynamoDB: DynamoDBConfig{
			Region:          getEnv("AWS_REGION", "us-west-2"),
			ChatroomTable:   getEnv("DYNAMODB_CHATROOM_TABLE", "chatrooms"),
			MessageTable:    getEnv("DYNAMODB_MESSAGE_TABLE", "messages"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		UserService: UserServiceConfig{
			Address: getEnv("USER_SERVICE_ADDRESS", "localhost:8082"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
